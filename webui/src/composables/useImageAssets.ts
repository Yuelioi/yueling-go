import { computed, ref } from 'vue'
import { api, UnauthorizedError, type ImageAsset, type ImageAssetSort, type ImageCategory } from '../api.ts'
import { useRequestScope } from './useRequestScope.ts'

type Client = Pick<typeof api, 'imageCategories' | 'imageAssets' | 'deleteImageAsset' | 'renameImageAsset'>
export function useImageAssets(client: Client = api) {
  const categories = ref<ImageCategory[]>([])
  const category = ref('')
  const query = ref('')
  const sort = ref<ImageAssetSort>('modified_desc')
  const page = ref(1)
  const pageSize = ref(20)
  const total = ref(0)
  const rows = ref<ImageAsset[]>([])
  const selected = ref<string[]>([])
  const loading = ref(false)
  const deleting = ref(false)
  const renaming = ref(false)
  const busy = computed(() => deleting.value || renaming.value)
  const error = ref('')
  const notice = ref('')
  const scope = useRequestScope()
  let viewVersion = 0
  const external = computed(() => categories.value.find(c => c.name === category.value)?.external ?? false)
  const pages = computed(() => Math.max(1, Math.ceil(total.value / pageSize.value)))
  async function load() {
    ++viewVersion
    const current = scope.begin('list')
    const source = category.value
    selected.value = []
    rows.value = []
    total.value = 0
    error.value = ''
    loading.value = true
    try {
      if (!source || external.value) return
      const result = await client.imageAssets(source, query.value.trim(), page.value, sort.value)
      if (!current()) return
      rows.value = result.assets ?? []
      total.value = result.total
      pageSize.value = result.page_size
      if (!rows.value.length && page.value > 1 && page.value > pages.value) {
        page.value = pages.value
        await load()
      }
    } catch (err) {
      if (current()) error.value = err instanceof Error ? err.message : '图片加载失败'
    } finally {
      if (current()) loading.value = false
    }
  }
  async function initialize() {
    const current = scope.begin('categories')
    try {
      const result = await client.imageCategories()
      if (!current()) return
      const unique = new Map<string, ImageCategory>()
      for (const item of result.categories) {
        const existing = unique.get(item.name)
        unique.set(item.name, { ...item, external: item.external || !!existing?.external })
      }
      categories.value = [...unique.values()]
      category.value = categories.value.find(c => !c.external)?.name ?? categories.value[0]?.name ?? ''
      await load()
    } catch (err) {
      if (current()) error.value = err instanceof Error ? err.message : '类目加载失败'
    }
  }
  async function changeCategory(value: string) {
    category.value = value
    query.value = ''
    page.value = 1
    notice.value = ''
    await load()
  }
  async function search() { page.value = 1; await load() }
  async function changeSort(value: ImageAssetSort) {
    sort.value = value
    page.value = 1
    notice.value = ''
    await load()
  }
  async function rename(asset: ImageAsset, name: string) {
    if (busy.value || external.value || !rows.value.some(row => row.id === asset.id)) return false
    name = name.trim()
    if (!name) { error.value = '请输入新名称'; return false }
    const source = category.value
    const version = viewVersion
    renaming.value = true
    error.value = ''
    notice.value = ''
    try {
      await client.renameImageAsset(source, asset.id, name)
      if (category.value === source && viewVersion === version) {
        const refresh = load()
        const refreshVersion = viewVersion
        await refresh
        if (category.value === source && viewVersion === refreshVersion)
          notice.value = `已重命名为「${name}」。`
      }
      return true
    } catch (err) {
      if (category.value === source && viewVersion === version)
        error.value = err instanceof Error ? err.message : '重命名失败'
      return false
    } finally { renaming.value = false }
  }
  async function remove(items: ImageAsset[]) {
    if (busy.value || external.value || !items.length) return
    const source = category.value
    const version = viewVersion
    const ids = [...new Set(items.map(item => item.id))].filter(id => rows.value.some(row => row.id === id))
    if (!ids.length) return
    deleting.value = true
    error.value = ''
    const removed: string[] = []
    const failed: string[] = []
    let failure = ''
    try {
      for (const id of ids) {
        try { await client.deleteImageAsset(source, id); removed.push(id) }
        catch (err) {
          failed.push(id)
          failure = err instanceof Error ? err.message : '删除失败'
          if (err instanceof UnauthorizedError) break
        }
      }
      if (category.value === source && viewVersion === version) {
        rows.value = rows.value.filter(row => !removed.includes(row.id))
        await load()
        notice.value = `已移入回收站 ${removed.length} 张${failed.length ? `；${failed.length} 张删除失败：${failure}` : ''}${ids.length > removed.length + failed.length ? `；${ids.length - removed.length - failed.length} 张未处理` : ''}。`
      }
    } finally { deleting.value = false }
  }
  return { categories, category, query, sort, page, pageSize, pages, total, rows, selected, loading, deleting, renaming, busy, error, notice, external, initialize, changeCategory, changeSort, load, search, rename, remove }
}
