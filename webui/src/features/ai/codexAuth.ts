import { ref } from 'vue'
import { aiApi, type AIConnection } from '../../api/ai.ts'
import { useRequestScope } from '../../composables/useRequestScope.ts'
export function useCodexAuth(
  client: Pick<typeof aiApi, 'codexStatus' | 'importCodex'> = aiApi,
) {
  const requests = useRequestScope()
  const statuses = ref<
    Record<string, { imported?: boolean; message: string; error?: boolean }>
  >({})
  const busy = ref(false)
  let uploadingAccount = ''
  async function read(account: AIConnection) {
    if (uploadingAccount === account.id) return
    const current = requests.begin(`auth:${account.id}`)
    try {
      const result = await client.codexStatus(account.id, {
        signal: current.signal,
      })
      if (current())
        statuses.value[account.id] = {
          imported: result.imported,
          message: result.imported ? '登录态已导入' : '尚未导入登录态',
        }
    } catch (error) {
      if (current())
        statuses.value[account.id] = {
          message: error instanceof Error ? error.message : '状态读取失败',
          error: true,
        }
    }
  }
  async function upload(account: AIConnection, file: File, replace: boolean) {
    if (busy.value) return false
    uploadingAccount = account.id
    const current = requests.begin(`auth:${account.id}`)
    busy.value = true
    try {
      if (file.size > 65536) throw new Error('登录文件不能超过64KB')
      await client.importCodex(account.id, await file.text(), replace)
      if (!current()) return false
      statuses.value[account.id] = {
        imported: true,
        message: '登录态已导入。服务器会维护后续刷新。',
      }
      return true
    } catch (error) {
      if (current())
        statuses.value[account.id] = {
          ...statuses.value[account.id],
          error: true,
          message: error instanceof Error ? error.message : '导入失败',
        }
      return false
    } finally {
      if (current()) {
        busy.value = false
        uploadingAccount = ''
      }
    }
  }
  return { statuses, busy, read, upload }
}
export type CodexAuth = ReturnType<typeof useCodexAuth>
