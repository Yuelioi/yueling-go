import type {
  ConfigurationField,
  AIModelProfile,
  AIConnection,
  ImageEntry,
} from '../../api.ts'

export type DraftValue = Exclude<ConfigurationField['value'], null>
export type ConfigurationDraft = Record<string, DraftValue> & {
  'ai.profiles': AIModelProfile[]
  'ai.connections': AIConnection[]
  'ai.bindings': Record<string, string>
  'ai.conversation_profile': string
  'ai.vision_profile': string
}
export function emptyDraft(): ConfigurationDraft {
  return {
    'ai.profiles': [],
    'ai.connections': [],
    'ai.bindings': {},
    'ai.conversation_profile': '',
    'ai.vision_profile': '',
  }
}
export function normalizeField(field: ConfigurationField): DraftValue {
  if (field.kind === 'numbers')
    return (field.value as number[] | null)?.join(', ') ?? ''
  if (field.kind === 'ai_profiles' || field.kind === 'ai_connections') {
    return (
      (field.value as (AIModelProfile | AIConnection)[] | null) ?? []
    ).map((row) => ({ ...row, clear_key: false })) as
      AIModelProfile[] | AIConnection[]
  }
  if (field.kind === 'image_entries') {
    return ((field.value as ImageEntry[] | null) ?? []).map((entry) => ({
      ...entry,
      kind: entry.kind || 'single',
      call: entry.call || [],
      arg: entry.arg ?? null,
    }))
  }
  return field.value ?? (field.kind === 'ai_bindings' ? {} : '')
}
export function fieldChanged(
  path: string,
  values: ConfigurationDraft,
  baseline: ConfigurationDraft,
  cleared: Record<string, boolean>,
) {
  return (
    JSON.stringify(values[path]) !== JSON.stringify(baseline[path]) ||
    !!cleared[path]
  )
}
export function encodeFields(
  fields: ConfigurationField[],
  values: ConfigurationDraft,
  cleared: Record<string, boolean>,
) {
  const payload: Record<string, unknown> = {}
  const clear: string[] = []
  for (const field of fields) {
    if (cleared[field.path]) clear.push(field.path)
    let value: unknown = values[field.path]
    if (field.kind === 'number') {
      if (String(value).trim() === '' || !Number.isSafeInteger(Number(value)))
        throw new Error(`${field.label} 请填写整数`)
      value = Number(value)
    }
    if (field.kind === 'numbers') {
      const parts = String(value)
        .trim()
        .split(/[\s,，]+/)
        .filter(Boolean)
      if (
        parts.some(
          (part) =>
            !/^\d+$/.test(part) ||
            !Number.isSafeInteger(Number(part)) ||
            Number(part) <= 0,
        )
      )
        throw new Error(`${field.label} 请填写正整数，多个号码用逗号分隔`)
      value = [...new Set(parts.map(Number))]
    }
    if (field.kind === 'ai_profiles' || field.kind === 'ai_connections') {
      value = (value as (AIModelProfile | AIConnection)[]).map(
        ({ key_configured, clear_key, ...row }) => ({
          ...row,
          api_key: clear_key ? '' : row.api_key,
          clear_key: !!clear_key,
        }),
      )
    }
    payload[field.path] = cleared[field.path] ? '' : value
  }
  return { payload, clear }
}
