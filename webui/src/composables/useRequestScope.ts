import { getCurrentScope, onScopeDispose } from 'vue'

export function useRequestScope() {
  const controllers = new Map<string, AbortController>()
  let disposed = false
  function dispose() {
    disposed = true
    controllers.forEach((controller) => controller.abort())
    controllers.clear()
  }
  if (getCurrentScope()) onScopeDispose(dispose)
  return {
    begin(key: string) {
      controllers.get(key)?.abort()
      const controller = new AbortController()
      controllers.set(key, controller)
      if (disposed) controller.abort()
      return Object.assign(
        () =>
          !disposed &&
          !controller.signal.aborted &&
          controllers.get(key) === controller,
        { signal: controller.signal },
      )
    },
  }
}
