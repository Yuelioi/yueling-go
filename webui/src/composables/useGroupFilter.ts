import { ref, watch, type Ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

export function useGroupFilter(existing?: Ref<number | null>) {
  const route = useRoute()
  const router = useRouter()
  function read() {
    const raw = route.query.group
    if (typeof raw !== 'string' || !/^\d+$/.test(raw)) return null
    const value = Number(raw)
    return Number.isSafeInteger(value) ? value : null
  }
  const group = existing ?? ref<number | null>(null)
  group.value = read()
  watch(() => route.query.group, () => { group.value = read() })
  watch(group, value => {
    const query = { ...route.query }
    if (value === null) delete query.group
    else query.group = String(value)
    void router.replace({ query })
  })
  return group
}
