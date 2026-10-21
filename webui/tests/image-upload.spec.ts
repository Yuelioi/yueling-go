import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { effectScope, nextTick } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import {
  HttpError,
  imagesApi,
  groupsApi,
  UnauthorizedError,
  type ImageCategory,
} from '../src/api.ts'
import { request } from '../src/api/http.ts'
import ImageUploadView from '../src/views/ImageUploadView.vue'
import {
  defaultImageName,
  imageUploadPreview,
  maxUploadBytes,
  uploadNameError,
  useImageUpload,
} from '../src/features/images/useImageUpload.ts'
import { uiStubs } from './ui-stubs.ts'

const categories: ImageCategory[] = [
  {
    name: '吃的',
    aliases: [],
    upload: true,
    external: false,
    naming: 'label',
    name_required: true,
  },
  {
    name: '语录',
    aliases: [],
    upload: true,
    external: false,
    naming: 'quotation',
    name_required: true,
  },
  {
    name: '表情',
    aliases: [],
    upload: true,
    external: false,
    naming: 'optional_label',
    name_required: true,
  },
  { name: '随机', aliases: [], upload: true, external: false, naming: 'hash' },
  { name: '外站', aliases: [], upload: false, external: true },
]
const file = (name = '米饭.png') =>
  new File(['image'], name, { type: 'image/png' })
const scopes: ReturnType<typeof effectScope>[] = []
beforeEach(() => {
  vi.spyOn(URL, 'createObjectURL').mockImplementation(() => 'blob:test')
  vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => {})
})
afterEach(() => {
  scopes.forEach((scope) => scope.stop())
  scopes.length = 0
  vi.unstubAllGlobals()
})
function fixture(
  uploadImageAsset = vi
    .fn()
    .mockResolvedValue({ name: '米饭_hash.png', duplicate: false }),
) {
  const scope = effectScope()
  scopes.push(scope)
  const state = scope.run(() => useImageUpload({ uploadImageAsset }))!
  state.categories.value = categories
  state.category.value = '吃的'
  return { state, uploadImageAsset, scope }
}
describe('category upload naming', () => {
  it('derives local names, removes existing storage metadata and mirrors category rules', () => {
    expect(defaultImageName('红烧肉.JPG', categories[0])).toBe('红烧肉')
    expect(
      defaultImageName('123_小明_0123456789abcdef.png', categories[1]),
    ).toBe('小明')
    expect(imageUploadPreview('  红烧肉🍖  ', categories[0])).toBe(
      '红烧肉_哈希.格式',
    )
    expect(imageUploadPreview('小明', categories[1], '123')).toBe(
      '123_小明_哈希.格式',
    )
    expect(imageUploadPreview('开心', categories[2])).toBe('开心_哈希.格式')
    expect(imageUploadPreview('忽略', categories[3])).toBe('哈希.格式')
    expect(uploadNameError('', categories[0])).toBeTruthy()
    expect(uploadNameError('../a', categories[2])).toBeTruthy()
    expect(uploadNameError('汉'.repeat(54), categories[0])).toBeTruthy()
  })
  it('requires a quotation group and sends only the explicit group and name', async () => {
    const { state, uploadImageAsset } = fixture()
    state.category.value = '语录'
    await nextTick()
    state.add([file('小明.png')])
    expect(state.canUpload.value).toBe(false)
    state.groupID.value = '123'
    expect(state.canUpload.value).toBe(true)
    await state.upload()
    expect(uploadImageAsset).toHaveBeenCalledWith(
      '语录',
      '小明',
      '123',
      expect.any(File),
      expect.objectContaining({ signal: expect.any(AbortSignal) }),
    )
    expect(state.dirty.value).toBe(false)
  })
})
describe('batch upload queue', () => {
  it('keeps per-file failures, skips successful files on retry and releases previews', async () => {
    const upload = vi
      .fn()
      .mockResolvedValueOnce({ name: 'a_hash.png', duplicate: false })
      .mockRejectedValueOnce(new HttpError('名称无效', 400))
      .mockResolvedValueOnce({ name: 'b_hash.png', duplicate: true })
    const { state, scope } = fixture(upload)
    state.add([file('a.png'), file('b.png')])
    expect(await state.upload()).toBe(true)
    expect(state.items.value.map((item) => item.status)).toEqual([
      'saved',
      'failed',
    ])
    state.items.value[1]!.name = '改名'
    await state.upload()
    expect(upload).toHaveBeenCalledTimes(3)
    expect(upload.mock.calls[2]![1]).toBe('改名')
    expect(state.items.value[1]!.status).toBe('duplicate')
    state.remove(state.items.value[0]!.id)
    expect(URL.revokeObjectURL).toHaveBeenCalledTimes(1)
    scope.stop()
    expect(URL.revokeObjectURL).toHaveBeenCalledTimes(2)
  })
  it.each([new Error('network'), new UnauthorizedError()])(
    'stops without automatic retries after an uncertain response or auth loss',
    async (failure) => {
      const { state, uploadImageAsset } = fixture(
        vi.fn().mockRejectedValue(failure),
      )
      state.add([file('a.png'), file('b.png')])
      await state.upload()
      expect(uploadImageAsset).toHaveBeenCalledTimes(1)
      expect(state.items.value[1]!.status).toBe('pending')
      expect(state.error.value).toBeTruthy()
    },
  )
  it('blocks oversize/unsupported/too-many files without uploading or leaking previews', () => {
    const { state, uploadImageAsset } = fixture()
    const large = file()
    Object.defineProperty(large, 'size', { value: maxUploadBytes + 1 })
    for (const files of [
      [large],
      [file('a.svg')],
      Array.from({ length: 51 }, () => file()),
    ]) {
      state.add(files)
      expect(state.items.value).toHaveLength(0)
      expect(state.error.value).toBeTruthy()
    }
    expect(URL.createObjectURL).not.toHaveBeenCalled()
    expect(uploadImageAsset).not.toHaveBeenCalled()
  })
  it('aborts on disposal and ignores a late response', async () => {
    let resolve!: (value: { name: string; duplicate: boolean }) => void
    const client = vi.fn<typeof imagesApi.uploadImageAsset>(
      () =>
        new Promise<{ name: string; duplicate: boolean }>((done) => {
          resolve = done
        }),
    )
    const { state, scope } = fixture(client)
    state.add([file(), file('b.png')])
    const pending = state.upload()
    scope.stop()
    expect(client.mock.calls[0]?.[4]?.signal?.aborted).toBe(true)
    resolve({ name: 'late.png', duplicate: false })
    expect(await pending).toBe(false)
    expect(client).toHaveBeenCalledTimes(1)
    expect(state.items.value[0]!.storedName).toBe('')
  })
})
describe('multipart client', () => {
  it('lets the browser create the boundary and retains session credentials', async () => {
    const fetcher = vi
      .fn()
      .mockImplementation(
        async () =>
          new Response(JSON.stringify({ name: 'a.png', duplicate: false })),
      )
    vi.stubGlobal('fetch', fetcher)
    await imagesApi.uploadImageAsset('语录', '小明', '123', file())
    const init = fetcher.mock.calls[0]![1] as RequestInit
    expect(new Headers(init.headers).has('Content-Type')).toBe(false)
    expect(init.credentials).toBe('include')
    const body = init.body as FormData
    expect(body.get('group_id')).toBe('123')
    expect(body.get('name')).toBe('小明')
    expect(body.get('file')).toBeInstanceOf(File)
    await request('/json', { method: 'POST', body: '{}' })
    expect(
      new Headers(fetcher.mock.calls[1]![1].headers).get('Content-Type'),
    ).toBe('application/json')
  })
})
describe('batch upload interface', () => {
  it('reads multiple local names, edits names, switches category rules and uploads a group quotation', async () => {
    vi.spyOn(imagesApi, 'imageCategories').mockResolvedValue({ categories })
    vi.spyOn(groupsApi, 'managedGroups').mockResolvedValue({
      ok: true,
      groups: [{ group_id: 123, group_name: '测试群' }],
    } as Awaited<ReturnType<typeof groupsApi.managedGroups>>)
    const upload = vi
      .spyOn(imagesApi, 'uploadImageAsset')
      .mockResolvedValue({ name: '123_新名字_hash.png', duplicate: false })
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/images', component: ImageUploadView }],
    })
    await router.push('/images?tab=upload')
    await router.isReady()
    const wrapper = mount(ImageUploadView, {
      global: {
        plugins: [router],
        stubs: {
          ...uiStubs,
          UCard: { template: '<div><slot /></div>' },
          ConfigurationGuard: true,
          EmptyState: true,
        },
      },
    })
    await flushPromises()
    const input = wrapper.find('input[type=file]')
    Object.defineProperty(input.element, 'files', {
      value: [file(), file('奶茶.jpg')],
      configurable: true,
    })
    await input.trigger('change')
    expect(wrapper.text()).toContain('米饭_哈希.格式')
    expect(wrapper.text()).toContain('奶茶_哈希.格式')
    await wrapper.find('select[aria-label="上传分类"]').setValue('语录')
    await nextTick()
    expect(wrapper.text()).toContain('群号_米饭_哈希.格式')
    await wrapper.find('input[aria-label="语录群号"]').setValue('123')
    await wrapper.find('input[aria-label="米饭.png 的名称"]').setValue('新名字')
    expect(wrapper.text()).toContain('123_新名字_哈希.格式')
    const button = wrapper
      .findAll('button')
      .find((item) => item.text() === '上传剩余 2 张')!
    await button.trigger('click')
    await flushPromises()
    expect(upload).toHaveBeenCalledTimes(2)
    expect(upload.mock.calls[0]!.slice(0, 3)).toEqual(['语录', '新名字', '123'])
    expect(wrapper.text()).toContain('全部完成')
    expect(wrapper.emitted('uploaded')).toEqual([['语录']])
    wrapper.unmount()
  })
})
