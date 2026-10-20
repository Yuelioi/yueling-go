import { request } from './http.ts'

export interface PluginEntry {
  id: number
  name: string
  group: string
  desc: string
  usage: string
  commands: string[] | null
}

export const pluginsApi = {
  async plugins(options?: Pick<RequestInit, 'signal'>) {
    const res = await request<{ ok: true; plugins: PluginEntry[] }>(
      '/api/webui/plugins',
      options,
    )
    return {
      ...res,
      plugins: res.plugins.map((plugin) => ({
        ...plugin,
        commands: Array.isArray(plugin.commands) ? plugin.commands : [],
      })),
    }
  },
  groupPlugins(groupID: number, options?: Pick<RequestInit, 'signal'>) {
    return request<{ ok: true; disabled: Record<string, boolean> }>(
      `/api/webui/groups/${groupID}/plugins`,
      options,
    )
  },
  setGroupPlugin(groupID: number, pluginID: number, disabled: boolean) {
    return request<{ ok: true }>(
      `/api/webui/groups/${groupID}/plugins/${pluginID}`,
      {
        method: 'PUT',
        body: JSON.stringify({ disabled }),
      },
    )
  },
  applyPluginAll(pluginID: number, groupIDs: number[], disabled: boolean) {
    return request<{ ok: true }>(`/api/webui/plugins/${pluginID}/apply-all`, {
      method: 'POST',
      body: JSON.stringify({ group_ids: groupIDs, disabled }),
    })
  },
}
