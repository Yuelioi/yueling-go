<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useClipboard } from '@vueuse/core'
import { wuxiaApi, type WuxiaState, type WuxiaSettings } from '../api/wuxia'
import { useGroupsStore } from '../stores/groups'
import { useRequestScope } from '../composables/useRequestScope'
import type { GroupInfo } from '../api'
import PageHeader from '../components/PageHeader.vue'
import GroupScopeSelect from '../components/GroupScopeSelect.vue'
import DataTable from '../components/DataTable.vue'
import WuxiaEquipmentPreview from '../components/WuxiaEquipmentPreview.vue'
import EmptyState from '../components/EmptyState.vue'

const group = ref<number | null>(0)
const groups = ref<GroupInfo[]>([{ group_id: 0, group_name: '全局默认' }])
const state = ref<WuxiaState | null>(null)
const draft = ref<WuxiaSettings>({
  daily_runs: 3,
  sign_silver: 0,
  practice_base: 50,
})
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const warning = ref('')
const saved = ref(false)
const requests = useRequestScope()
const { copy, copied } = useClipboard({ legacy: true })
const invite = computed(() => {
  const settings = state.value?.config.settings
  if (!settings) return ''
  return `月灵江湖开玩啦！\n发「江湖」自动建角并连续完成每日3场随机历练。正向收益随机浮动50%～200%。\n发「我的侠客」看属性和装备；发「江湖武技」「江湖心法」查看并选择已学武学。\n发「切磋 @群友」双方自动建角、免费 PK，按实际属性与逐招战斗结算，不扣双方资源。\n「江湖转职」查看并选择可用职业；「江湖形象」换外观。\n「江湖背包」看收藏；「江湖装备 物品名」换装；「江湖学艺 秘籍名」学武。\n「江湖奖池」看价格和概率；「江湖寻宝」抽一次，「江湖寻宝 5」抽五次。\n「江湖战报」重看最近结果；「江湖帮助」查看全部指令。`
})
async function copyInvite() {
  warning.value = ''
  try {
    await copy(invite.value)
  } catch {
    warning.value = '复制失败，请直接选中下方说明复制。'
  }
}
const dirty = computed(
  () =>
    state.value &&
    JSON.stringify(draft.value) !== JSON.stringify(state.value.config.settings),
)
const valid = computed(
  () =>
    Number.isInteger(draft.value.daily_runs) &&
    draft.value.daily_runs >= 1 &&
    draft.value.daily_runs <= 12 &&
    Number.isInteger(draft.value.sign_silver) &&
    draft.value.sign_silver >= 0 &&
    draft.value.sign_silver <= 10000 &&
    Number.isInteger(draft.value.practice_base) &&
    draft.value.practice_base >= 1 &&
    draft.value.practice_base <= 10000,
)
const reasons: Record<string, string> = {
  daily: '一键日常',
  sign: '签到',
  adventure: '历练',
  train: '练武',
  draw: '寻宝',
}
function itemName(id: string) {
  return (
    [
      ...(state.value?.catalog.weapons ?? []),
      ...(state.value?.catalog.items ?? []),
    ].find((item) => item.id === id)?.name ?? id
  )
}

function apply(value: WuxiaState) {
  state.value = value
  draft.value = { ...value.config.settings }
}
async function load() {
  const scope = group.value
  const current = requests.begin('scope')
  state.value = null
  error.value = ''
  saved.value = false
  if (scope === null) {
    loading.value = false
    return
  }
  loading.value = true
  try {
    const value = await wuxiaApi.read(scope, current.signal)
    if (current()) apply(value)
  } catch (err) {
    if (current()) error.value = err instanceof Error ? err.message : '读取失败'
  } finally {
    if (current()) loading.value = false
  }
}
async function save(inherit = false) {
  const scope = group.value
  if (
    scope === null ||
    !state.value ||
    saving.value ||
    (!inherit && !valid.value)
  )
    return
  saving.value = true
  error.value = ''
  saved.value = false
  try {
    const value = inherit
      ? await wuxiaApi.inherit(scope)
      : await wuxiaApi.save(scope, draft.value)
    if (group.value === scope) {
      apply(value)
      saved.value = true
    }
  } catch (err) {
    error.value = err instanceof Error ? err.message : '保存失败'
  } finally {
    saving.value = false
  }
}
watch(group, load, { immediate: true })
onMounted(async () => {
  try {
    groups.value = [
      { group_id: 0, group_name: '全局默认' },
      ...(await useGroupsStore().loadManaged()).groups,
    ]
  } catch {
    warning.value =
      '群列表暂不可用，仍可设置全局数值；机器人在线后刷新可选择群聊。'
  }
})
</script>

<template>
  <section class="min-w-0 space-y-6">
    <PageHeader
      title="月灵江湖"
      description="一键日常、随机历练与群友切磋。统一管理数值，角色卡与战报在服务器本地合成。"
      icon="i-tabler-swords"
    >
      <UButton
        to="/plugins"
        color="neutral"
        variant="soft"
        icon="i-tabler-adjustments"
        >插件开关</UButton
      >
    </PageHeader>
    <GroupScopeSelect
      v-model="group"
      :groups="groups"
      :disabled="saving"
      zero-label="全局默认"
      description="各群角色和银两独立；未设置单群数值时使用全局默认。"
    />
    <UAlert v-if="warning" color="warning" :description="warning" />
    <UAlert v-if="error" color="error" :description="error" />
    <UAlert
      v-if="saved"
      color="success"
      description="已保存，后续新操作立即使用新数值。既有结算不会重新发奖。"
    />
    <div
      v-if="loading"
      class="rounded-2xl border border-default bg-default p-8 text-muted"
      role="status"
    >
      正在读取江湖数据…
    </div>
    <template v-if="state">
      <section
        class="rounded-2xl border border-default bg-default p-5 sm:p-6 space-y-4"
      >
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h2 class="font-semibold text-highlighted">群内试玩</h2>
            <p class="mt-1 text-sm text-muted">
              先在插件开关中启用「月灵江湖」，再把说明发到试玩群。
            </p>
          </div>
          <UButton
            color="neutral"
            variant="soft"
            :icon="copied ? 'i-tabler-check' : 'i-tabler-copy'"
            @click="copyInvite"
          >
            {{ copied ? '已复制' : '复制开玩说明' }}
          </UButton>
        </div>
        <pre
          class="whitespace-pre-wrap rounded-xl bg-elevated p-4 font-sans text-sm leading-7 text-muted"
          >{{ invite }}</pre>
        <p class="text-xs text-muted">
          说明使用已保存数值。直接发送「切磋
          @群友」即可开打，双方没有侠客会自动创建；日常自动装备当前兵刃类别的更高阶武器，练武由玩家自行发送指令。
        </p>
      </section>
      <section
        class="rounded-2xl border border-default bg-default p-5 sm:p-6 space-y-5"
      >
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h2 class="font-semibold text-highlighted">日常与经济</h2>
            <p class="mt-1 text-sm text-muted">
              {{
                group === 0
                  ? '设置所有群使用的默认数值。'
                  : state.config.custom
                    ? '本群使用独立配置。'
                    : '本群正在继承全局配置，保存后转为独立配置。'
              }}
            </p>
          </div>
          <UBadge color="neutral" variant="soft"
            >规则 {{ state.catalog.version }}</UBadge
          >
        </div>
        <div class="grid gap-5 sm:grid-cols-3">
          <p class="text-sm text-stone-400">每日历练3次，一条指令完成；首次主动切磋奖励100银两、20阅历。签到已取消。</p>
          <UFormField
            label="练武基础花费"
            description="每重递增20银两；武学受角色等级限制。"
            ><UInput
              v-model.number="draft.practice_base"
              type="number"
              :min="1"
              :max="10000"
              :disabled="saving"
              class="w-full"
          /></UFormField>
        </div>
        <div class="flex flex-wrap gap-3">
          <UButton
            :loading="saving"
            :disabled="!dirty || !valid || loading"
            icon="i-tabler-device-floppy"
            @click="save()"
            >保存数值</UButton
          >
          <UButton
            v-if="group !== 0 && state.config.custom"
            color="neutral"
            variant="soft"
            :disabled="saving"
            @click="save(true)"
            >恢复全局默认</UButton
          >
        </div>
      </section>
      <WuxiaEquipmentPreview />
      <div class="grid gap-4 md:grid-cols-3">
        <section
          v-for="profession in state.catalog.professions"
          :key="profession.id"
          class="rounded-2xl border border-default bg-default p-5"
        >
          <h2 class="font-semibold text-highlighted">{{ profession.name }}</h2>
          <p class="mt-2 text-sm leading-6 text-muted">
            {{ profession.description }}
          </p>
          <p class="mt-2 text-sm text-muted">
            专属兵器 · {{ profession.weapon_kind }}
          </p>
          <p v-if="profession.per_level" class="mt-2 text-sm text-muted">
            每级：气血 +{{ profession.per_level.hp }} / 攻击 +{{
              profession.per_level.attack
            }}
            / 防御 +{{ profession.per_level.defense }} / 身法 +{{
              profession.per_level.speed
            }}
          </p>
        </section>
      </div>
      <section
        class="rounded-2xl border border-default bg-default p-5 text-sm text-muted space-y-2"
      >
        <h2 class="font-semibold text-highlighted">群内入口</h2>
        <p>江湖 · 我的侠客 · 切磋 @群友 · 江湖练武 · 江湖榜</p>
        <p>江湖转职 · 江湖武器 · 江湖形象 · 江湖战报 · 江湖帮助</p>
        <p>
          {{ state.catalog.weapons.length }} 件兵刃、{{
            state.catalog.events.length
          }}
          种随机事件。切磋按兵器招式交锋，展现暴击、破防、招架和卸力；战斗规则保留弱者翻盘机会。
        </p>
      </section>
      <section
        class="rounded-2xl border border-default bg-default p-5 space-y-4"
      >
        <h2 class="font-semibold text-highlighted">寻宝奖池</h2>
        <p class="text-sm text-muted">
          本期等级上限
          {{ state.catalog.max_level ?? 10 }}
          级。先筛选当前职业和等级可用的奖励，再按权重抽取。以下展示原始权重；实际概率请在群内发送「江湖奖池」查看。重复物品保留，每次独立抽取，无保底。
        </p>
        <div
          v-for="pool in state.catalog.pools ?? []"
          :key="pool.id"
          class="space-y-3"
        >
          <h3 class="font-medium">
            {{ pool.name }} · 每抽 {{ pool.cost }}
            {{
              pool.cost_item === 'silver' ? '银两' : itemName(pool.cost_item)
            }}
          </h3>
          <div class="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
            <div
              v-for="entry in pool.entries"
              :key="entry.reward.item_id"
              class="flex justify-between gap-3 rounded-lg bg-elevated p-3 text-sm"
            >
              <span
                >{{ itemName(entry.reward.item_id) }} ×{{
                  entry.reward.quantity
                }}</span
              >
              <span class="text-muted">{{ `权重 ${entry.weight}` }}</span>
            </div>
          </div>
        </div>
      </section>
      <section
        class="rounded-2xl border border-default bg-default p-5 space-y-3"
      >
        <h2 class="font-semibold text-highlighted">武技与心法</h2>
        <div
          v-for="skill in state.catalog.skills ?? []"
          :key="skill.id"
          class="text-sm"
        >
          <span class="font-medium"
            >{{ skill.name }} ·
            {{
              skill.kind === 'mind' ? '通用心法' : skill.weapon_kind + '类武技'
            }}</span
          >
          <p class="whitespace-pre-line text-muted">{{ skill.description }}</p>
        </div>
      </section>
      <section
        v-if="group !== 0"
        class="overflow-hidden rounded-2xl border border-default bg-default"
      >
        <div class="p-5">
          <h2 class="font-semibold text-highlighted">本群侠客</h2>
          <p class="mt-1 text-sm text-muted">
            按当前规则战力排序，最多显示100位。
          </p>
        </div>
        <DataTable v-if="state.players.length" label="本群侠客"
          ><thead>
            <tr>
              <th>侠客</th>
              <th>职业</th>
              <th>等级 / 战力</th>
              <th>兵刃</th>
              <th>银两 / 武学</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="player in state.players" :key="player.character.user_id">
              <td>
                <p>{{ player.character.name }}</p>
                <p class="mt-1 text-xs text-muted">
                  {{ player.character.user_id }}
                </p>
              </td>
              <td>{{ player.profession.name }}</td>
              <td>Lv.{{ player.level }} / {{ player.power }}</td>
              <td>{{ player.weapon.name }}</td>
              <td>
                {{ player.character.silver }} / {{ player.character.mastery }}重
              </td>
            </tr>
          </tbody></DataTable
        >
        <EmptyState
          v-else
          title="还没有侠客"
          description="群友发送「江湖」即可自动创建角色并完成日常。"
          icon="i-tabler-swords"
        />
      </section>
      <section
        v-if="group !== 0"
        class="overflow-hidden rounded-2xl border border-default bg-default"
      >
        <div class="p-5">
          <h2 class="font-semibold text-highlighted">最近银两流水</h2>
          <p class="mt-1 text-sm text-muted">
            最多50笔，重复消息不会再次结算。
          </p>
        </div>
        <DataTable v-if="state.ledger.length" label="银两流水"
          ><thead>
            <tr>
              <th>玩家QQ</th>
              <th>事项</th>
              <th>变动</th>
              <th>结余</th>
              <th>时间</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="entry in state.ledger" :key="entry.id">
              <td>{{ entry.user_id }}</td>
              <td>{{ reasons[entry.reason] ?? entry.reason }}</td>
              <td :class="entry.delta > 0 ? 'text-success' : 'text-muted'">
                {{ entry.delta > 0 ? '+' : '' }}{{ entry.delta }}
              </td>
              <td>{{ entry.balance }}</td>
              <td>{{ new Date(entry.created_at).toLocaleString() }}</td>
            </tr>
          </tbody></DataTable
        >
        <EmptyState
          v-else
          title="暂无流水"
          description="历练、切磋和消费的银两变动会保存在这里。"
          icon="i-tabler-receipt"
        />
      </section>
    </template>
  </section>
</template>
