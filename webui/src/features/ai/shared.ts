export function newID() {
  return Array.from(crypto.getRandomValues(new Uint8Array(16)), (b) =>
    b.toString(16).padStart(2, '0'),
  ).join('')
}
export function copy<T>(value: T): T {
  return JSON.parse(JSON.stringify(value))
}
export const capabilities = [
  { value: 'chat', label: '聊天' },
  { value: 'tools', label: '工具调用' },
  { value: 'vision', label: '读图' },
  { value: 'image_generation', label: '生图' },
  { value: 'image_edit', label: '改图' },
  { value: 'web_search', label: '联网搜索' },
  { value: 'structured_output', label: '结构化输出' },
]
