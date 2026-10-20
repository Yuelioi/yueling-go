import { request } from './http.ts'

export const authApi = {
  login(password: string) {
    return request<{ ok: true }>('/api/webui/auth/login', {
      method: 'POST',
      body: JSON.stringify({ password }),
    })
  },
  me(options?: Pick<RequestInit, 'signal'>) {
    return request<{ ok: true; authenticated: boolean }>(
      '/api/webui/auth/me',
      options,
    )
  },
  logout() {
    return request<{ ok: true }>('/api/webui/auth/logout', { method: 'POST' })
  },
}
