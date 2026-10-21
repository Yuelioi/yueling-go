import { computed, onScopeDispose, ref, watch } from 'vue'
import {
  HttpError,
  imagesApi,
  UnauthorizedError,
  type ImageCategory,
} from '../../api.ts'

export const maxUploadBytes = 16 * 1024 * 1024
export const maxUploadFiles = 50
export interface UploadItem {
  id: number
  file: File
  name: string
  preview: string
  status:
    'pending' | 'uploading' | 'saved' | 'duplicate' | 'failed' | 'uncertain'
  message: string
  storedName: string
}

export function defaultImageName(filename: string, category?: ImageCategory) {
  let name = filename.replace(/\.[^.]+$/, '')
  // Re-importing an exported asset must not append its old storage suffix twice.
  name = name.replace(/_[a-f0-9]{16}$/i, '')
  if (category?.naming === 'quotation') name = name.replace(/^\d+_/, '')
  return name
}
export function imageLabel(name: string) {
  return Array.from(name.trim())
    .slice(0, 64)
    .map((char) => (/[\p{L}\p{N}_ -]/u.test(char) ? char : '_'))
    .join('')
    .replace(/^[ ._-]+|[ ._-]+$/g, '')
}
export function uploadNameError(name: string, category?: ImageCategory) {
  if (!category || category.naming === 'hash') return ''
  const value = name.trim()
  if (category.name_required && !value) return '请填写名称'
  if (
    value &&
    (value === '.' ||
      value === '..' ||
      /[/\\\0\r\n]/.test(value) ||
      new TextEncoder().encode(value).length > 160)
  )
    return '名称不能包含路径或换行，且不能超过 160 字节'
  if (category.naming === 'label' && !imageLabel(value))
    return '请填写可用的名称'
  return ''
}
export function imageUploadPreview(
  name: string,
  category?: ImageCategory,
  groupID = '',
) {
  const value = category?.naming === 'label' ? imageLabel(name) : name.trim()
  switch (category?.naming) {
    case 'quotation':
      return `${groupID || '群号'}_${value || '名称'}_哈希.格式`
    case 'label':
    case 'optional_label':
      return `${value ? value + '_' : ''}哈希.格式`
    default:
      return '哈希.格式'
  }
}

export function useImageUpload(
  client: Pick<typeof imagesApi, 'uploadImageAsset'> = imagesApi,
) {
  const categories = ref<ImageCategory[]>([])
  const category = ref('')
  const groupID = ref('')
  const items = ref<UploadItem[]>([])
  const busy = ref(false)
  const error = ref('')
  const selectedCategory = computed(() =>
    categories.value.find((item) => item.name === category.value),
  )
  const editable = (item: UploadItem) =>
    !['saved', 'duplicate', 'uploading'].includes(item.status)
  const remaining = computed(() => items.value.filter(editable))
  const dirty = computed(() => remaining.value.length > 0)
  const groupError = computed(() =>
    selectedCategory.value?.naming === 'quotation' &&
    (!/^[1-9]\d*$/.test(groupID.value) ||
      !Number.isSafeInteger(Number(groupID.value)))
      ? '请选择或输入有效的语录所属群号'
      : '',
  )
  const canUpload = computed(
    () =>
      !busy.value &&
      selectedCategory.value?.upload &&
      !selectedCategory.value.external &&
      remaining.value.length > 0 &&
      !groupError.value &&
      remaining.value.every(
        (item) =>
          !uploadNameError(item.name, selectedCategory.value) &&
          item.file.size > 0 &&
          item.file.size <= maxUploadBytes,
      ),
  )
  let nextID = 0
  let disposed = false
  let controller: AbortController | undefined
  function release(item: UploadItem) {
    URL.revokeObjectURL(item.preview)
  }
  function clear() {
    if (busy.value) return
    items.value.forEach(release)
    items.value = []
    error.value = ''
  }
  watch(category, () => {
    groupID.value = ''
    error.value = ''
  })
  function add(files: File[]) {
    if (busy.value) return
    error.value = ''
    if (files.length + items.value.length > maxUploadFiles) {
      error.value = `每批最多 ${maxUploadFiles} 张，请分批选择`
      return
    }
    const invalid = files.find(
      (file) =>
        !/\.(png|jpe?g|gif|webp)$/i.test(file.name) ||
        !file.size ||
        file.size > maxUploadBytes,
    )
    if (invalid) {
      error.value = `「${invalid.name}」不符合要求：支持 PNG、JPEG、GIF、WebP，单张不超过 16 MiB`
      return
    }
    for (const file of files) {
      items.value.push({
        id: ++nextID,
        file,
        name: defaultImageName(file.name, selectedCategory.value),
        preview: URL.createObjectURL(file),
        status: 'pending',
        message: '',
        storedName: '',
      })
    }
  }
  function remove(id: number) {
    if (busy.value) return
    const item = items.value.find((item) => item.id === id)
    if (item) release(item)
    items.value = items.value.filter((item) => item.id !== id)
  }
  async function upload() {
    if (!canUpload.value) return false
    busy.value = true
    error.value = ''
    const target = category.value
    const group =
      selectedCategory.value?.naming === 'quotation' ? groupID.value : ''
    const naming = selectedCategory.value?.naming
    controller = new AbortController()
    let changed = false
    try {
      for (const item of [...remaining.value]) {
        if (disposed) break
        item.status = 'uploading'
        item.message = ''
        try {
          const result = await client.uploadImageAsset(
            target,
            naming === 'hash' ? '' : item.name.trim(),
            group,
            item.file,
            { signal: controller.signal },
          )
          if (disposed) break
          item.status = result.duplicate ? 'duplicate' : 'saved'
          item.storedName = result.name
          item.message = result.duplicate ? '已存在，跳过' : '上传成功'
          changed = true
        } catch (err) {
          if (disposed) break
          item.status =
            err instanceof HttpError && err.status < 500
              ? 'failed'
              : 'uncertain'
          item.message = err instanceof Error ? err.message : '上传失败'
          if (item.status === 'uncertain')
            item.message = '结果未确认，可手动重试；相同名称和内容不会重复保存'
          if (err instanceof UnauthorizedError) {
            error.value = '登录已失效，队列已停止'
            break
          }
          // A lost response may follow a successful write. Leave retry to the user.
          if (item.status === 'uncertain') {
            error.value = '连接异常，队列已停止，请检查结果后重试'
            break
          }
        }
      }
    } finally {
      busy.value = false
      controller = undefined
    }
    return changed
  }
  onScopeDispose(() => {
    disposed = true
    controller?.abort()
    items.value.forEach(release)
  })
  return {
    categories,
    category,
    groupID,
    items,
    selectedCategory,
    busy,
    error,
    dirty,
    remaining,
    groupError,
    canUpload,
    editable,
    add,
    remove,
    clear,
    upload,
  }
}
