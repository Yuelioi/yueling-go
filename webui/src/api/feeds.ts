import { request } from './http.ts'

export interface FeedHealth {
  id: number
  group_id: number
  status: string
  message: string
  checked_at: number
}

export interface FeedSubscription {
  id: number
  group_id: number
  url: string
  name: string
  created_by: number
  enabled: boolean
  translate_to_chinese: boolean
  created_at: number
  updated_at: number
  consecutive_failures: number
  last_error: string
  last_checked_at: number
  last_success_at: number
  next_check_at: number
}

export interface FeedSettings {
  poll_interval_minutes: number
  group_id: number
  quiet_enabled: boolean
  quiet_start: string
  quiet_end: string
  item_max_chars: number
  updated_at: number
}

export interface FeedCheckResult {
  checked: number
  updated: number
  items: number
  delivered: number
  queued: number
  failed: number
}

export const feedsApi = {
  feeds(options?: Pick<RequestInit, 'signal'>) {
    return request<{ ok: true; feeds: FeedSubscription[] }>(
      '/api/webui/feeds',
      options,
    )
  },
  diagnoseFeeds(groupID: number | null = null, id = 0) {
    return request<{ ok: true; results: FeedHealth[] }>(
      '/api/webui/feeds/diagnose',
      {
        method: 'POST',
        body: JSON.stringify({ group_id: groupID ?? 0, id }),
      },
    )
  },
  addFeed(groupID: number, url: string, name: string) {
    return request<{ ok: true; feed: FeedSubscription; latest_title: string }>(
      `/api/webui/groups/${groupID}/feeds`,
      {
        method: 'POST',
        body: JSON.stringify({ url, name }),
      },
    )
  },
  addPlatformFeed(
    groupID: number,
    platform: string,
    target: string,
    name: string,
  ) {
    return request<{ ok: true; feed: FeedSubscription; latest_title: string }>(
      `/api/webui/groups/${groupID}/feeds/platform`,
      {
        method: 'POST',
        body: JSON.stringify({ platform, target, name }),
      },
    )
  },
  deleteFeed(groupID: number, feedID: number) {
    return request<{ ok: true }>(
      `/api/webui/groups/${groupID}/feeds/${feedID}`,
      { method: 'DELETE' },
    )
  },
  setFeedEnabled(groupID: number, feedID: number, enabled: boolean) {
    return request<{ ok: true; feed: FeedSubscription }>(
      `/api/webui/groups/${groupID}/feeds/${feedID}`,
      {
        method: 'PUT',
        body: JSON.stringify({ enabled }),
      },
    )
  },
  setFeedTranslation(
    groupID: number,
    feedID: number,
    translateToChinese: boolean,
  ) {
    return request<{ ok: true; feed: FeedSubscription }>(
      `/api/webui/groups/${groupID}/feeds/${feedID}`,
      {
        method: 'PUT',
        body: JSON.stringify({ translate_to_chinese: translateToChinese }),
      },
    )
  },
  checkFeeds() {
    return request<{ ok: true; result: FeedCheckResult }>(
      `/api/webui/feeds/check`,
      { method: 'POST' },
    )
  },
  feedSettings(groupID: number, options?: Pick<RequestInit, 'signal'>) {
    return request<{ ok: true; settings: FeedSettings; pending_count: number }>(
      `/api/webui/groups/${groupID}/feeds/settings`,
      options,
    )
  },
  setFeedSettings(
    groupID: number,
    settings: Pick<
      FeedSettings,
      | 'quiet_enabled'
      | 'quiet_start'
      | 'quiet_end'
      | 'item_max_chars'
      | 'poll_interval_minutes'
    >,
  ) {
    return request<{ ok: true; settings: FeedSettings; pending_count: number }>(
      `/api/webui/groups/${groupID}/feeds/settings`,
      {
        method: 'PUT',
        body: JSON.stringify(settings),
      },
    )
  },
}
