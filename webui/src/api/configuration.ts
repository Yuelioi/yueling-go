import { request } from './http.ts'
import type { AIConnection, AIModelProfile } from './ai.ts'
import type { ImageEntry } from './images.ts'

export interface ConfigurationField {
  path: string
  label: string
  kind: string
  value:
    | string
    | number
    | boolean
    | number[]
    | ImageEntry[]
    | AIModelProfile[]
    | AIConnection[]
    | Record<string, string>
    | null
  secret: boolean
  configured: boolean
  reload: boolean
}

export interface RuntimeConfiguration {
  applied_revision: number
  restart_pending: boolean
  revision: number
  sections: { id: string; title: string; fields: ConfigurationField[] }[]
  bootstrap: {
    database_dsn_configured: boolean
    data_dir: string
    note: string
  }
}

export const configurationApi = {
  configuration(options?: Pick<RequestInit, 'signal'>) {
    return request<RuntimeConfiguration>('/api/webui/config', options)
  },
  saveConfiguration(
    revision: number,
    section: string,
    values: Record<string, unknown>,
    clear_secrets: string[],
  ) {
    return request<{ ok: true; reload: boolean; config: RuntimeConfiguration }>(
      '/api/webui/config',
      {
        method: 'PUT',
        body: JSON.stringify({ revision, section, values, clear_secrets }),
      },
    )
  },
}
