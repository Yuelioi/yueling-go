import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { effectScope, nextTick } from 'vue'
import { authApi } from '../src/api/auth.ts'
import { groupsApi } from '../src/api/groups.ts'
import {
  configurationApi,
  type RuntimeConfiguration,
} from '../src/api/configuration.ts'
import {
  onUnauthorized,
  request,
  HttpError,
  UnauthorizedError,
} from '../src/api/http.ts'
import { useSessionStore } from '../src/stores/session.ts'
import { useGroupsStore } from '../src/stores/groups.ts'
import { useConfigurationStore } from '../src/stores/configuration.ts'
import { useRequestScope } from '../src/composables/useRequestScope.ts'

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((done) => {
    resolve = done
  })
  return { promise, resolve }
}
const snapshot = (revision: number): RuntimeConfiguration => ({
  revision,
  applied_revision: revision,
  restart_pending: false,
  sections: [],
  bootstrap: { data_dir: 'fixture', note: '', database_dsn_configured: true },
})
beforeEach(() => {
  setActivePinia(createPinia())
})
describe('shared state lifetime', () => {
  it('deduplicates login checks and invalidates a late response after expiration', async () => {
    const response = deferred<{ ok: true; authenticated: boolean }>()
    const me = vi.spyOn(authApi, 'me').mockReturnValue(response.promise)
    const session = useSessionStore()
    const first = session.ensureAuthenticated(),
      second = session.ensureAuthenticated()
    const results = Promise.allSettled([first, second])
    expect(me).toHaveBeenCalledTimes(1)
    session.expire()
    response.resolve({ ok: true, authenticated: true })
    expect((await results).every((r) => r.status === 'rejected')).toBe(true)
    expect(session.status).toBe('anonymous')
  })
  it('shares managed groups and refuses to republish data after logout', async () => {
    const response =
      deferred<Awaited<ReturnType<typeof groupsApi.groupAdministration>>>()
    const fetch = vi
      .spyOn(groupsApi, 'groupAdministration')
      .mockReturnValue(response.promise)
    const groups = useGroupsStore()
    const reads = Promise.allSettled([
      groups.loadManaged(),
      groups.loadAdministration(),
    ])
    expect(fetch).toHaveBeenCalledTimes(1)
    groups.reset()
    response.resolve({ ok: true, groups: [], warning: '' })
    expect((await reads).every((r) => r.status === 'rejected')).toBe(true)
    expect(groups.managed).toEqual([])
    fetch.mockResolvedValue({ ok: true, groups: [], warning: 'fixture' })
    await groups.loadManaged()
    await groups.loadAdministration()
    expect(fetch).toHaveBeenCalledTimes(2)
  })
  it('does not replace a saved configuration with an older concurrent load', async () => {
    const response = deferred<RuntimeConfiguration>()
    vi.spyOn(configurationApi, 'configuration').mockReturnValue(
      response.promise,
    )
    vi.spyOn(configurationApi, 'saveConfiguration').mockResolvedValue({
      ok: true,
      reload: false,
      config: snapshot(2),
    })
    const configuration = useConfigurationStore()
    const read = configuration.load()
    await configuration.save(1, 'ai', {}, [])
    response.resolve(snapshot(1))
    await read
    expect(configuration.snapshot?.revision).toBe(2)
    configuration.reset()
    expect(configuration.snapshot).toBe(null)
  })
  it('aborts superseded reads and reads owned by an unmounted scope', () => {
    const scope = effectScope()
    const requests = scope.run(() => useRequestScope())!
    const first = requests.begin('read'),
      second = requests.begin('read')
    expect(first()).toBe(false)
    expect(first.signal.aborted).toBe(true)
    expect(second()).toBe(true)
    scope.stop()
    expect(second()).toBe(false)
    expect(second.signal.aborted).toBe(true)
  })
})
describe('HTTP contract', () => {
  it('preserves headers and conflict status', async () => {
    const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ error: '配置版本冲突' }), {
        status: 409,
      }),
    )
    const result = request('/fixture', { headers: { 'X-Fixture': 'present' } })
    await expect(result).rejects.toMatchObject({
      status: 409,
      message: '配置版本冲突',
    })
    const headers = fetch.mock.calls[0]![1]!.headers as Headers
    expect(headers.get('X-Fixture')).toBe('present')
    expect(headers.get('Content-Type')).toBe('application/json')
  })
  it('publishes unauthorized state once and rejects malformed successful responses', async () => {
    const invalidated = vi.fn(),
      stop = onUnauthorized(invalidated)
    const fetch = vi
      .spyOn(globalThis, 'fetch')
      .mockResolvedValue(new Response('{}', { status: 401 }))
    await expect(request('/private')).rejects.toBeInstanceOf(UnauthorizedError)
    expect(invalidated).toHaveBeenCalledTimes(1)
    stop()
    fetch.mockResolvedValue(new Response('<html>proxy</html>', { status: 200 }))
    await expect(request('/fixture')).rejects.toBeInstanceOf(HttpError)
    await nextTick()
  })
})

it('ignores a tier-policy response after its mounted owner is disposed', async () => {
  const { useAffinityTiers } = await import('../src/features/affinity/tiers.ts')
  const { affinityClient } = await import('../src/features/affinity/client.ts')
  const { ref } = await import('vue')
  const response =
    deferred<Awaited<ReturnType<typeof affinityClient.affinitySettings>>>()
  vi.spyOn(affinityClient, 'affinitySettings').mockReturnValue(response.promise)
  const scope = effectScope(),
    threshold = ref(10)
  const tiers = scope.run(() => useAffinityTiers(affinityClient, threshold))!
  const load = tiers.loadTierSettings()
  scope.stop()
  response.resolve({
    ok: true,
    custom: false,
    tiers: [],
    initial: 50,
    min_score: 0,
    max_score: 100,
    block_below: 99,
    max_tiers: 8,
    max_name_chars: 20,
    max_prompt_chars: 4000,
  })
  await load
  expect(tiers.tierSettings.value).toBe(null)
  expect(threshold.value).toBe(10)
})
