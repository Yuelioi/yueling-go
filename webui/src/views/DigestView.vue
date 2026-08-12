<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { api, type DailyDigest, type GroupInfo } from '../api'
import GroupPicker from '../components/GroupPicker.vue'
import MetricCard from '../components/MetricCard.vue'
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

const selectedGroup = computed(() => groups.value.find((group) => group.group_id === selectedGroupID.value))
const selectedDigest = computed(() => digests.value.find((row) => row.GroupID === selectedGroupID.value))
const configuredGroups = computed(() => digests.value.map((digest) => ({
  ...digest,
  group: groups.value.find((group) => group.group_id === digest.GroupID),
})))
const coverage = computed(() => groups.value.length ? `${Math.round(digests.value.length / groups.value.length * 100)}%` : '0%')
const averageCount = computed(() => digests.value.length
  ? Math.round(digests.value.reduce((sum, row) => sum + row.MessageCount, 0) / digests.value.length)
  : 0)

function syncForm() {
  sendTime.value = selectedDigest.value?.SendTime || '21:30'
  messageCount.value = selectedDigest.value?.MessageCount || 80
  saved.value = ''
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const [groupRes, digestRes] = await Promise.all([api.groups(), api.digests()])
    groups.value = groupRes.groups
    digests.value = digestRes.digests
    if (!selectedGroupID.value) {
      selectedGroupID.value = digests.value[0]?.GroupID || groups.value[0]?.group_id || null
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
    const res = await api.setDigest(selectedGroupID.value, sendTime.value, count)
    const index = digests.value.findIndex((row) => row.GroupID === selectedGroupID.value)
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
    digests.value = digests.value.filter((row) => row.GroupID !== selectedGroupID.value)
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
      <UButton icon="i-tabler-refresh" :loading="loading" @click="load">刷新设置</UButton>
    </PageHeader>

    <div class="metrics-grid">
      <MetricCard label="已开启群聊" :value="digests.length" detail="每日自动执行" icon="i-tabler-calendar-check" tone="violet" />
      <MetricCard label="群聊覆盖率" :value="coverage" :detail="`${groups.length} 个可用群聊`" icon="i-tabler-chart-donut" tone="cyan" />
      <MetricCard label="平均采样" :value="averageCount" detail="每次读取消息条数" icon="i-tabler-messages" tone="amber" />
    </div>

    <UAlert v-if="error" class="error-banner" color="error" variant="subtle" icon="i-tabler-alert-circle" :description="error" />

    <div class="grid gap-4 lg:grid-cols-[292px_minmax(0,1fr)]">
      <GroupPicker v-model="selectedGroupID" :groups="groups" title="日报群聊" description="选择要配置的群" />

      <div class="space-y-4">
        <section class="surface-panel overflow-hidden">
          <div class="panel-header">
            <div class="min-w-0">
              <div class="truncate font-medium text-white">{{ selectedGroup?.group_name || '未选择群' }}</div>
              <div class="text-xs text-zinc-500">{{ selectedGroupID ? `群号 ${selectedGroupID}` : '选择群聊后可配置' }}</div>
            </div>
            <UBadge :color="selectedDigest ? 'success' : 'neutral'" variant="subtle">{{ selectedDigest ? '已开启' : '未开启' }}</UBadge>
          </div>

          <div class="digest-editor">
            <div class="digest-editor-copy">
              <span class="digest-editor-icon"><UIcon name="i-tabler-moon-stars" class="size-6" /></span>
              <div>
                <h3>每日自动摘要</h3>
                <p>月灵会读取设置数量的最新群消息，生成不超过 500 字的结构化日报。</p>
              </div>
            </div>

            <div class="digest-form-grid">
              <UFormField label="发送时间" description="使用 Bot 配置的时区">
                <UInput v-model="sendTime" type="time" icon="i-tabler-clock" :disabled="!selectedGroupID" />
              </UFormField>
              <UFormField label="消息条数" description="可设置 10 到 100 条">
                <UInput v-model.number="messageCount" type="number" min="10" max="100" icon="i-tabler-messages" :disabled="!selectedGroupID" />
              </UFormField>
            </div>

            <div class="digest-preview surface-inset">
              <div class="digest-preview-head"><span>🌙 群聊日报</span><small>发送预览</small></div>
              <div class="digest-preview-block"><strong>今日话题</strong><span>提炼群内最主要的讨论方向</span></div>
              <div class="digest-preview-block"><strong>重要信息</strong><span>保留结论、通知和有价值的信息</span></div>
              <div class="digest-preview-block"><strong>待跟进</strong><span>整理聊天中尚未完成的事项</span></div>
            </div>

            <div class="digest-editor-actions">
              <div class="min-h-5 text-sm">
                <span v-if="saved" class="inline-status"><UIcon name="i-tabler-circle-check" class="size-4" />{{ saved }}</span>
                <span v-else class="text-zinc-500">保存后立即加入调度，Bot 重启后仍会恢复。</span>
              </div>
              <div class="flex gap-2">
                <UButton v-if="selectedDigest" color="error" variant="soft" icon="i-tabler-player-stop" @click="confirmOpen = true">关闭日报</UButton>
                <UButton icon="i-tabler-device-floppy" :loading="saving" :disabled="!selectedGroupID" @click="save">保存设置</UButton>
              </div>
            </div>
          </div>
        </section>

        <section class="surface-panel overflow-hidden">
          <div class="panel-header">
            <div><div class="section-title">已开启的群聊</div><div class="section-caption">当前全部日报调度</div></div>
            <span class="count-pill">{{ configuredGroups.length }}</span>
          </div>
          <button v-for="row in configuredGroups" :key="row.ID" type="button" class="digest-schedule-row" @click="selectedGroupID = row.GroupID">
            <span class="schedule-time">{{ row.SendTime }}</span>
            <span class="min-w-0 flex-1 text-left"><strong>{{ row.group?.group_name || row.GroupID }}</strong><small>最近 {{ row.MessageCount }} 条消息</small></span>
            <UIcon name="i-tabler-chevron-right" class="size-4 text-zinc-600" />
          </button>
          <div v-if="!loading && configuredGroups.length === 0" class="empty-state overview-empty">
            <UIcon name="i-tabler-calendar-off" class="size-6" /><span>还没有群聊开启日报</span>
          </div>
        </section>
      </div>
    </div>

    <UModal v-model:open="confirmOpen" title="关闭这个群的日报？" description="保存的时间和消息数量设置会被删除。" :ui="{ overlay: 'z-40 bg-black/70 backdrop-blur-sm', content: 'z-50 bg-zinc-900 text-zinc-100 ring ring-rose-500/30 divide-zinc-800 shadow-2xl', header: 'border-b border-zinc-800', body: 'bg-zinc-900', footer: 'border-t border-zinc-800 bg-zinc-900', title: 'text-white', description: 'text-zinc-400' }">
      <template #body><div class="surface-inset p-4 text-sm text-zinc-300">{{ selectedGroup?.group_name || selectedGroupID }} 将不再收到每日 AI 群聊摘要。</div></template>
      <template #footer><div class="flex w-full justify-end gap-2"><UButton color="neutral" variant="ghost" @click="confirmOpen = false">取消</UButton><UButton color="error" icon="i-tabler-player-stop" :loading="deleting" @click="remove">确认关闭</UButton></div></template>
    </UModal>
  </section>
</template>
