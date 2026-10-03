<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { api, type DailyDigest, type GroupInfo } from '../api'
import GroupScopeSelect from '../components/GroupScopeSelect.vue'
import MetricCard from '../components/MetricCard.vue'
import MoreActions from '../components/MoreActions.vue'
import PageHeader from '../components/PageHeader.vue'

const groups = ref<GroupInfo[]>([])
const digests = ref<DailyDigest[]>([])
const selectedGroupID = ref<number | null>(null)
const sendTime = ref('21:30')
const messageCount = ref(80)
const loading = ref(false)
const saving = ref(false)
const deleting = ref(false)
const confirmOpen = ref(false)
const error = ref('')
const saved = ref('')

const selectedGroup = computed(() =>
  groups.value.find((group) => group.group_id === selectedGroupID.value),
)
const selectedDigest = computed(() =>
  digests.value.find((row) => row.GroupID === selectedGroupID.value),
)
const configuredGroups = computed(() =>
  digests.value.map((digest) => ({
    ...digest,
    group: groups.value.find((group) => group.group_id === digest.GroupID),
  })),
)
const coverage = computed(() =>
  groups.value.length
    ? `${Math.round((digests.value.length / groups.value.length) * 100)}%`
    : '0%',
)
const averageCount = computed(() =>
  digests.value.length
    ? Math.round(
        digests.value.reduce((sum, row) => sum + row.MessageCount, 0) /
          digests.value.length,
      )
    : 0,
)

function syncForm() {
  sendTime.value = selectedDigest.value?.SendTime || '21:30'
  messageCount.value = selectedDigest.value?.MessageCount || 80
  saved.value = ''
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const [groupRes, digestRes] = await Promise.all([
      api.groups(),
      api.digests(),
    ])
    groups.value = groupRes.groups
    digests.value = digestRes.digests
    if (!selectedGroupID.value) {
      selectedGroupID.value =
        digests.value[0]?.GroupID || groups.value[0]?.group_id || null
    }
    syncForm()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '日报设置加载失败'
  } finally {
    loading.value = false
  }
}

async function save() {
  if (!selectedGroupID.value) {
    error.value = '请选择群聊'
    return
  }
  if (!/^([01]\d|2[0-3]):[0-5]\d$/.test(sendTime.value)) {
    error.value = '请输入有效时间，例如 21:30'
    return
  }
  const count = Number(messageCount.value)
  if (!Number.isInteger(count) || count < 10 || count > 100) {
    error.value = '消息条数应为 10 到 100'
    return
  }
  saving.value = true
  error.value = ''
  saved.value = ''
  try {
    const res = await api.setDigest(
      selectedGroupID.value,
      sendTime.value,
      count,
    )
    const index = digests.value.findIndex(
      (row) => row.GroupID === selectedGroupID.value,
    )
    if (index >= 0) digests.value[index] = res.digest
    else digests.value.push(res.digest)
    saved.value = `每天 ${res.digest.SendTime} 自动发送`
  } catch (err) {
    error.value = err instanceof Error ? err.message : '保存失败'
  } finally {
    saving.value = false
  }
}

async function remove() {
  if (!selectedGroupID.value) return
  deleting.value = true
  error.value = ''
  try {
    await api.deleteDigest(selectedGroupID.value)
    digests.value = digests.value.filter(
      (row) => row.GroupID !== selectedGroupID.value,
    )
    confirmOpen.value = false
    syncForm()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '关闭失败'
  } finally {
    deleting.value = false
  }
}

watch(selectedGroupID, syncForm)
onMounted(load)
</script>

<template>
  <section class="space-y-5">
    <PageHeader
      eyebrow="Daily intelligence"
      title="群聊日报"
      description="让月灵每天自动整理群聊话题、重要信息和待跟进事项。"
      icon="i-tabler-notes"
    >
      <UButton
        color="neutral"
        variant="soft"
        icon="i-tabler-refresh"
        :loading="loading"
        @click="load"
        >刷新设置</UButton
      >
    </PageHeader>

    <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
      <MetricCard
        label="已开启群聊"
        :value="digests.length"
        detail="每日自动执行"
        icon="i-tabler-calendar-check"
        tone="violet"
      />
      <MetricCard
        label="群聊覆盖率"
        :value="coverage"
        :detail="`${groups.length} 个可用群聊`"
        icon="i-tabler-chart-donut"
        tone="cyan"
      />
      <MetricCard
        label="平均采样"
        :value="averageCount"
        detail="每次读取消息条数"
        icon="i-tabler-messages"
        tone="amber"
      />
    </div>

    <UAlert
      v-if="error"
      color="error"
      variant="subtle"
      icon="i-tabler-alert-circle"
      :description="error"
    />

    <div class="space-y-4">
      <GroupScopeSelect
        v-model="selectedGroupID"
        :groups="groups"
        title="日报群聊"
        description="选择要配置的群"
      />

      <div class="space-y-4">
        <UCard as="section" class="min-w-0" :ui="{ body: 'p-0 sm:p-0' }"
          ><div class="overflow-hidden">
            <div
              class="flex flex-wrap items-center justify-between gap-3 border-b border-default px-5 py-4"
            >
              <div class="min-w-0">
                <div class="truncate font-medium text-highlighted">
                  {{ selectedGroup?.group_name || '未选择群' }}
                </div>
                <div class="text-xs text-dimmed">
                  {{
                    selectedGroupID
                      ? `群号 ${selectedGroupID}`
                      : '选择群聊后可配置'
                  }}
                </div>
              </div>
              <UBadge
                :color="selectedDigest ? 'success' : 'neutral'"
                variant="subtle"
                >{{ selectedDigest ? '已开启' : '未开启' }}</UBadge
              >
            </div>

            <div class="space-y-5 p-5">
              <div class="flex items-start gap-3">
                <span
                  class="flex size-11 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary"
                  ><UIcon name="i-tabler-moon-stars" class="size-6"
                /></span>
                <div>
                  <h3 class="text-sm font-semibold text-highlighted">
                    每日自动摘要
                  </h3>
                  <p class="mt-1 text-sm leading-6 text-muted">
                    月灵会读取设置数量的最新群消息，生成不超过 500
                    字的结构化日报。
                  </p>
                </div>
              </div>

              <div class="grid gap-4 sm:grid-cols-2">
                <UFormField label="发送时间" description="使用 Bot 配置的时区">
                  <UInput
                    v-model="sendTime"
                    class="w-full"
                    type="time"
                    icon="i-tabler-clock"
                    :disabled="!selectedGroupID"
                  />
                </UFormField>
                <UFormField label="消息条数" description="可设置 10 到 100 条">
                  <UInput
                    v-model.number="messageCount"
                    class="w-full"
                    type="number"
                    min="10"
                    max="100"
                    icon="i-tabler-messages"
                    :disabled="!selectedGroupID"
                  />
                </UFormField>
              </div>

              <div
                class="flex flex-wrap items-center justify-between gap-3 border-t border-default pt-4"
              >
                <div class="min-h-5 text-sm">
                  <span
                    v-if="saved"
                    class="inline-flex items-center gap-1.5 text-success"
                    ><UIcon name="i-tabler-circle-check" class="size-4" />{{
                      saved
                    }}</span
                  >
                  <span v-else class="text-dimmed"
                    >保存后立即加入调度，Bot 重启后仍会恢复。</span
                  >
                </div>
                <div class="flex items-center gap-2">
                  <MoreActions
                    v-if="selectedDigest"
                    label="日报更多操作"
                    show-label
                    :items="[
                      {
                        label: '关闭日报',
                        description: '删除当前群的调度设置',
                        icon: 'i-tabler-player-stop',
                        color: 'error',
                        onSelect: () => {
                          confirmOpen = true
                        },
                      },
                    ]"
                  />
                  <UButton
                    icon="i-tabler-device-floppy"
                    :loading="saving"
                    :disabled="!selectedGroupID"
                    @click="save"
                    >保存设置</UButton
                  >
                </div>
              </div>
            </div>
          </div></UCard
        >

        <UCard as="section" class="min-w-0" :ui="{ body: 'p-0 sm:p-0' }"
          ><div class="overflow-hidden">
            <div
              class="flex flex-wrap items-center justify-between gap-3 border-b border-default px-5 py-4"
            >
              <div>
                <div class="text-sm font-semibold text-highlighted">
                  已开启的群聊
                </div>
                <div class="mt-1 text-xs leading-5 text-muted">
                  当前全部日报调度
                </div>
              </div>
              <span
                class="inline-flex shrink-0 items-center rounded-md bg-muted px-2 py-1 text-xs font-medium text-muted"
                >{{ configuredGroups.length }}</span
              >
            </div>
            <UButton
              color="neutral"
              variant="ghost"
              v-for="row in configuredGroups"
              :key="row.ID"
              type="button"
              class="w-full justify-start gap-3 rounded-none border-t border-default px-5 py-4"
              @click="selectedGroupID = row.GroupID"
            >
              <span class="font-mono text-sm text-primary">{{
                row.SendTime
              }}</span>
              <span class="min-w-0 flex-1 text-left"
                ><strong class="block truncate text-sm font-medium">{{
                  row.group?.group_name || row.GroupID
                }}</strong
                ><small class="mt-1 block text-xs font-normal text-muted"
                  >最近 {{ row.MessageCount }} 条消息</small
                ></span
              >
              <UIcon name="i-tabler-chevron-right" class="size-4 text-dimmed" />
            </UButton>
            <div
              v-if="!loading && configuredGroups.length === 0"
              class="flex min-h-40 flex-col items-center justify-center gap-2 px-5 py-8 text-center text-sm text-muted"
            >
              <UIcon name="i-tabler-calendar-off" class="size-6" /><span
                >还没有群聊开启日报</span
              >
            </div>
          </div></UCard
        >
      </div>
    </div>

    <UModal
      v-model:open="confirmOpen"
      title="关闭这个群的日报？"
      description="保存的时间和消息数量设置会被删除。"
    >
      <template #body
        ><div
          class="rounded-lg bg-muted ring ring-default p-4 text-sm text-toned"
        >
          {{ selectedGroup?.group_name || selectedGroupID }} 将不再收到每日 AI
          群聊摘要。
        </div></template
      >
      <template #footer
        ><div class="flex w-full justify-end gap-2">
          <UButton color="neutral" variant="ghost" @click="confirmOpen = false"
            >取消</UButton
          ><UButton
            color="error"
            icon="i-tabler-player-stop"
            :loading="deleting"
            @click="remove"
            >确认关闭</UButton
          >
        </div></template
      >
    </UModal>
  </section>
</template>
