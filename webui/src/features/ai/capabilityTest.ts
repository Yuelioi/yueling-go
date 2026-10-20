import { computed, ref } from 'vue'
import { aiApi, type AIModelProfile } from '../../api/ai.ts'
import { modelCapabilities } from '../../ai-models.ts'
import { capabilities, copy } from './shared.ts'
import { useRequestScope } from '../../composables/useRequestScope.ts'
export function useCapabilityTest(
  client: Pick<typeof aiApi, 'testAIProfile'> = aiApi,
) {
  const requests = useRequestScope()
  const target = ref<AIModelProfile | null>(null)
  const open = ref(false)
  const busy = ref(false)
  const capability = ref('chat')
  const result = ref('')
  const image = ref('')
  const options = computed(() =>
    capabilities.filter(
      (c) => target.value && modelCapabilities(target.value).includes(c.value),
    ),
  )
  function show(profile: AIModelProfile) {
    target.value = copy(profile)
    capability.value = modelCapabilities(profile)[0] ?? 'chat'
    result.value = ''
    image.value = ''
    open.value = true
  }
  async function run() {
    if (!target.value || busy.value) return
    const current = requests.begin('test')
    busy.value = true
    result.value = ''
    image.value = ''
    try {
      const response = await client.testAIProfile(
        target.value.id,
        capability.value,
      )
      if (!current()) return
      image.value = response.image_url || ''
      result.value = `${capabilities.find((c) => c.value === capability.value)?.label || capability.value}${response.status === 'verified' ? '测试通过' : '测试失败'}：${response.message}（${response.duration_ms}ms）`
    } catch (error) {
      if (current())
        result.value = error instanceof Error ? error.message : '测试失败'
    } finally {
      if (current()) busy.value = false
    }
  }
  return { target, open, busy, capability, result, image, options, show, run }
}
export type CapabilityTest = ReturnType<typeof useCapabilityTest>
