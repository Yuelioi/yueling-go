import { getCurrentScope, onScopeDispose } from 'vue'

// A result belongs to one request and one mounted owner, including its errors.
export function useRequestScope() {
  const generations = new Map<string, number>()
  let disposed = false
  if (getCurrentScope())
    onScopeDispose(() => {
      disposed = true
    })
  return {
    begin(key: string) {
      const generation = (generations.get(key) ?? 0) + 1
      generations.set(key, generation)
      return () => !disposed && generations.get(key) === generation
    },
  }
}
