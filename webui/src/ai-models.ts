import type { AIConnection, AIModelProfile } from './api'

export function modelCapabilities(profile: AIModelProfile): string[] {
  return (
    profile.capabilities ?? [
      'chat',
      'tools',
      ...(profile.vision ? ['vision'] : []),
    ]
  )
}

export function accountDestination(account?: AIConnection): string {
  if (!account) return '未选择账号'
  if (account.kind === 'codex') return 'Codex 订阅 · 服务器登录态'
  return `API · ${account.base_url || '尚未填写地址'}`
}

export function isDeepSeekAccount(account?: AIConnection): boolean {
  if (!account || account.kind !== 'openai') return false
  try {
    return (
      new URL(account.base_url).hostname.toLowerCase() === 'api.deepseek.com'
    )
  } catch {
    return false
  }
}

export function modelRouteIssue(
  profile: AIModelProfile,
  accounts: AIConnection[],
): string {
  const account = accounts.find((c) => c.id === profile.connection_id)
  if (!account) return '请选择实际使用的账号。'
  if (isDeepSeekAccount(account) && /^gpt-/i.test(profile.model.trim())) {
    return `${profile.model} 不能发往 DeepSeek 官方接口。使用 Codex 订阅时，请选择 Codex 账号；使用 API 时，请选择支持该模型的服务商。`
  }
  if (
    account.kind === 'codex' &&
    modelCapabilities(profile).some((c) =>
      ['web_search', 'structured_output'].includes(c),
    )
  ) {
    return 'Codex 账号不支持这里的 DeepSeek 联网搜索或结构化输出约束，请更换账号或调整用途。'
  }
  return ''
}

export function profileOptionLabel(
  profile: AIModelProfile,
  accounts: AIConnection[],
): string {
  const account = accounts.find((c) => c.id === profile.connection_id)
  const parts = [
    profile.name || profile.model,
    account?.name || '未选择账号',
    account?.kind === 'codex' ? 'Codex 订阅' : 'API',
  ]
  return parts
    .filter(
      (part, i) =>
        parts.findIndex((p) => p.toLowerCase() === part.toLowerCase()) === i,
    )
    .join(' · ')
}

export const modelPurposes = [
  { value: 'chat', label: '对话与工具', capabilities: ['chat', 'tools'] },
  { value: 'vision', label: '图片理解', capabilities: ['chat', 'vision'] },
  {
    value: 'image',
    label: '生图与改图',
    capabilities: ['image_generation', 'image_edit'],
  },
  { value: 'search', label: '联网搜索', capabilities: ['web_search'] },
]
