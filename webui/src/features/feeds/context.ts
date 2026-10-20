import type { Ref } from 'vue'
import type { FeedSubscription } from '../../api/feeds.ts'
export interface FeedContext {
  selectedGroupID: Ref<number | null>
  feeds: Ref<FeedSubscription[]>
  error: Ref<string>
  notice: Ref<string>
}
