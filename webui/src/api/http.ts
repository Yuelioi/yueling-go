export class HttpError extends Error {
  readonly status: number
  constructor(message: string, status: number) {
    super(message)
    this.status = status
    this.name = 'HttpError'
  }
}

export class UnauthorizedError extends HttpError {
  constructor(message = '请先登录') {
    super(message, 401)
    this.name = 'UnauthorizedError'
  }
}

const unauthorizedListeners = new Set<() => void>()
export function onUnauthorized(listener: () => void) {
  unauthorizedListeners.add(listener)
  return () => unauthorizedListeners.delete(listener)
}

export async function request<T>(
  url: string,
  init: RequestInit = {},
): Promise<T> {
  const headers = new Headers(init.headers)
  if (!headers.has('Content-Type'))
    headers.set('Content-Type', 'application/json')
  const response = await fetch(url, {
    ...init,
    headers,
    credentials: 'include',
  })
  const text = await response.text()
  let data: unknown
  try {
    data = text ? JSON.parse(text) : undefined
  } catch {
    if (response.ok)
      throw new HttpError(
        '服务返回的数据无法解析，请稍后重试。',
        response.status,
      )
  }
  if (!response.ok) {
    const message =
      data &&
      typeof data === 'object' &&
      'error' in data &&
      typeof data.error === 'string'
        ? data.error
        : `HTTP ${response.status}`
    if (response.status === 401) {
      unauthorizedListeners.forEach((listener) => listener())
      throw new UnauthorizedError(message)
    }
    throw new HttpError(message, response.status)
  }
  if (response.status !== 204 && data === undefined)
    throw new HttpError('服务返回的数据为空，请稍后重试。', response.status)
  return data as T
}
