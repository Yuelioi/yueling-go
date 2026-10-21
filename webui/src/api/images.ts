import { request } from './http.ts'

export interface ImageCategory {
  name: string
  aliases: string[] | null
  external: boolean
  upload: boolean
  name_required?: boolean
  naming?: string
}

export interface ImageAsset {
  id: string
  name: string
  modified_at: string
}

export type ImageAssetSort =
  'modified_desc' | 'modified_asc' | 'name_asc' | 'name_desc'

export interface ImageAssetPage {
  assets: ImageAsset[]
  category: string
  total: number
  page: number
  page_size: number
}

export interface ImageEntry {
  folder: string
  call: string[]
  add: string
  kind: string
  arg: boolean | null
  url: string
  pick: string
  base: string
}

export function imagePreviewURL(
  category: string,
  id: string,
  thumbnail = false,
) {
  return `/api/webui/image-assets/${encodeURIComponent(id)}/preview?${new URLSearchParams({ category, ...(thumbnail ? { thumbnail: '1' } : {}) })}`
}

export const imagesApi = {
  uploadImageAsset(
    category: string,
    name: string,
    groupID: string,
    file: File,
    options?: Pick<RequestInit, 'signal'>,
  ) {
    const body = new FormData()
    body.set('category', category)
    body.set('name', name)
    if (groupID) body.set('group_id', groupID)
    body.set('file', file)
    return request<{ name: string; duplicate: boolean }>(
      '/api/webui/image-assets/upload',
      { ...options, method: 'POST', body },
    )
  },
  imageCategories(options?: Pick<RequestInit, 'signal'>) {
    return request<{ categories: ImageCategory[] }>(
      '/api/webui/image-assets/categories',
      options,
    )
  },
  imageAssets(
    category: string,
    query: string,
    page: number,
    sort: ImageAssetSort = 'modified_desc',
    options?: Pick<RequestInit, 'signal'>,
  ) {
    return request<ImageAssetPage>(
      `/api/webui/image-assets?${new URLSearchParams({ category, query, page: String(page), sort })}`,
      options,
    )
  },
  renameImageAsset(category: string, id: string, name: string) {
    return request<{ ok: true; name: string }>(
      `/api/webui/image-assets/${encodeURIComponent(id)}/rename?${new URLSearchParams({ category })}`,
      { method: 'POST', body: JSON.stringify({ name }) },
    )
  },
  deleteImageAsset(category: string, id: string) {
    return request<{ ok: true; name: string }>(
      `/api/webui/image-assets/${encodeURIComponent(id)}?${new URLSearchParams({ category })}`,
      { method: 'DELETE' },
    )
  },
}
