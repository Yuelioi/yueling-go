import { defineStore } from 'pinia'
import { ref } from 'vue'
import { authApi } from '../api/auth.ts'
import { UnauthorizedError } from '../api/http.ts'

export const useSessionStore = defineStore('session', () => {
  const status = ref<'unknown' | 'authenticated' | 'anonymous'>('unknown')
  let checkedAt = 0
  let generation = 0
  let pending: Promise<void> | undefined
  function expire() {
    generation++
    pending = undefined
    checkedAt = 0
    status.value = 'anonymous'
  }
  async function ensureAuthenticated() {
    if (status.value === 'authenticated' && Date.now() - checkedAt < 30_000)
      return
    if (pending) return pending
    const current = ++generation
    const task = (async () => {
      try {
        const result = await authApi.me()
        if (current !== generation) throw new UnauthorizedError()
        if (!result.authenticated) throw new UnauthorizedError()
        status.value = 'authenticated'
        checkedAt = Date.now()
      } catch (error) {
        if (current === generation && error instanceof UnauthorizedError)
          status.value = 'anonymous'
        throw error
      } finally {
        if (current === generation) pending = undefined
      }
    })()
    pending = task
    return task
  }
  async function login(password: string) {
    const current = ++generation
    pending = undefined
    await authApi.login(password)
    if (current !== generation) throw new UnauthorizedError()
    status.value = 'authenticated'
    checkedAt = Date.now()
  }
  async function logout() {
    await authApi.logout()
    expire()
  }
  return { status, ensureAuthenticated, login, logout, expire }
})
