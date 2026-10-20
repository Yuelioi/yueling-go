<script setup lang="ts">
import EmptyState from '../../../components/EmptyState.vue'
import type { useFeed } from '../useFeed.ts'
import MoreActions from '../../../components/MoreActions.vue'
const props = defineProps<{
  catalog: Pick<
    ReturnType<typeof useFeed>['catalog'],
    | 'groups'
    | 'selectedGroupID'
    | 'selectedGroup'
    | 'groupFeeds'
    | 'sourceHost'
    | 'formatDate'
  >
  actions: Pick<
    ReturnType<typeof useFeed>['actions'],
    | 'diagnosing'
    | 'health'
    | 'toggling'
    | 'translationSaving'
    | 'diagnose'
    | 'askDelete'
    | 'setEnabled'
    | 'setTranslation'
  >
}>()
const {
  groups,
  selectedGroupID,
  selectedGroup,
  groupFeeds,
  sourceHost,
  formatDate,
} = props.catalog
const {
  diagnosing,
  health,
  toggling,
  translationSaving,
  diagnose,
  askDelete,
  setEnabled,
  setTranslation,
} = props.actions
</script>
<template>
  <section
    class="@container rounded-2xl border border-default bg-default overflow-hidden"
  >
    <div
      class="flex flex-col items-stretch justify-between gap-4 border-b border-default px-5 py-4 sm:flex-row sm:items-center"
    >
      <div class="min-w-0">
        <div class="text-sm font-semibold text-highlighted">当前信息源</div>
        <div class="mt-1 text-xs leading-5 text-muted">
          {{
            selectedGroup?.group_name ||
            (selectedGroupID === null ? '全部群' : selectedGroupID)
          }}
          · {{ groupFeeds.length }}
          {{ selectedGroupID === null ? '个订阅' : '/ 10' }}
        </div>
      </div>
    </div>

    <div v-if="groupFeeds.length">
      <div
        v-for="row in groupFeeds"
        :key="row.id"
        class="flex flex-wrap items-start gap-4 border-t border-default p-5 transition-colors first:border-t-0 hover:bg-muted/30 @min-[52rem]:flex-nowrap @min-[52rem]:items-center"
      >
        <span
          class="grid size-[34px] shrink-0 place-items-center rounded-[10px] border border-[var(--border)] bg-[rgba(103,216,236,0.08)] text-[var(--cyan)]"
          ><UIcon name="i-tabler-rss" class="size-4"
        /></span>
        <div class="min-w-[calc(100%-4rem)] flex-1 @min-[52rem]:min-w-0">
          <div class="truncate text-sm font-medium text-highlighted">
            {{ row.name }}
          </div>
          <UButton
            size="xs"
            variant="link"
            @click="selectedGroupID = row.group_id"
            >{{
              groups.find((g) => g.group_id === row.group_id)?.group_name ||
              '群'
            }}
            · {{ row.group_id }}</UButton
          >
          <div
            v-if="health[row.id]"
            class="my-2 text-xs"
            :class="
              health[row.id]?.status === 'readable'
                ? 'text-emerald-300'
                : 'text-amber-300'
            "
            role="status"
          >
            {{ health[row.id]?.message }} ·
            {{ formatDate(health[row.id]!.checked_at) }}
          </div>
          <a
            :href="row.url"
            target="_blank"
            rel="noreferrer"
            class="mt-1 block truncate text-xs text-violet-300 hover:text-violet-200"
            >{{ sourceHost(row.url) }}</a
          >
          <div class="mt-1 text-[11px] text-dimmed">
            {{
              row.last_checked_at
                ? `上次检查 ${formatDate(row.last_checked_at)}`
                : `添加于 ${formatDate(row.created_at)}`
            }}
            <span v-if="row.next_check_at">
              · 下次 {{ formatDate(row.next_check_at) }}</span
            >
          </div>
          <div
            v-if="row.enabled && row.last_error"
            class="mt-1 truncate text-[11px] text-rose-400"
            :title="row.last_error"
          >
            {{ row.last_error }}
          </div>
        </div>
        <div
          class="flex w-full min-w-0 flex-wrap items-center gap-3 @min-[52rem]:w-auto @min-[52rem]:shrink-0 @min-[52rem]:justify-end"
        >
          <UBadge
            :color="
              !row.enabled
                ? 'neutral'
                : row.consecutive_failures
                  ? 'error'
                  : 'success'
            "
            variant="subtle"
          >
            {{
              !row.enabled
                ? '已暂停'
                : row.consecutive_failures
                  ? `异常 ${row.consecutive_failures} 次`
                  : row.last_checked_at
                    ? '最近拉取成功'
                    : '尚未检查'
            }}
          </UBadge>
          <div
            class="inline-flex min-h-8 items-center gap-[7px] rounded-[9px] border px-2 py-1 pl-[9px] text-xs font-semibold transition-[border-color,background-color,color] duration-150"
            :class="
              row.translate_to_chinese
                ? 'border-[rgba(125,108,200,0.34)] bg-[rgba(125,108,200,0.1)] text-[var(--violet-bright)] hover:border-[rgba(125,108,200,0.34)] hover:bg-[rgba(125,108,200,0.1)] hover:text-[var(--violet-bright)]'
                : 'border-[rgba(147,141,168,0.2)] bg-white/[0.025] text-muted hover:border-[rgba(185,175,230,0.34)] hover:bg-white/[0.045] hover:text-toned'
            "
          >
            <UIcon
              name="i-tabler-language"
              class="size-3.5"
              aria-hidden="true"
            />
            <span>翻译中文</span>
            <USwitch
              :model-value="row.translate_to_chinese"
              color="primary"
              :disabled="translationSaving[row.id]"
              :aria-label="
                row.translate_to_chinese
                  ? `关闭 ${row.name} 的中文翻译`
                  : `开启 ${row.name} 的中文翻译`
              "
              @update:model-value="setTranslation(row, Boolean($event))"
            />
          </div>
          <USwitch
            :model-value="row.enabled"
            color="primary"
            :disabled="toggling[row.id]"
            :aria-label="row.enabled ? '暂停订阅' : '恢复订阅'"
            @update:model-value="setEnabled(row, Boolean($event))"
          />
          <UButton
            size="xs"
            variant="soft"
            :disabled="diagnosing"
            @click="diagnose(row)"
            >检测</UButton
          >
          <MoreActions
            label="订阅更多操作"
            :items="[
              {
                label: '删除订阅',
                description: '停止检查和推送',
                icon: 'i-tabler-trash',
                color: 'error',
                onSelect: () => askDelete(row),
              },
            ]"
          />
        </div>
      </div>
    </div>
    <EmptyState
      v-else
      icon="i-tabler-rss-off"
      title="当前范围没有订阅"
      description="添加信息源后，新内容会自动推送到群聊"
    />
  </section>
</template>
