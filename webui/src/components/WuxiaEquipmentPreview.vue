<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { watchDebounced } from '@vueuse/core'
import {
  wuxiaPreviewApi,
  type PreviewBuild,
  type PreviewData,
  type PreviewItem,
  type PreviewResult,
} from '../api/wuxiaPreview'
import { useRequestScope } from '../composables/useRequestScope'

const data = ref<PreviewData | null>(null)
const result = ref<PreviewResult | null>(null)
const error = ref('')
const busy = ref(false)
const opponent = ref(2)
const showCard = ref(false)
const detail = ref<PreviewItem | null>(null)
const drawn = ref<PreviewItem[]>([])
const drawing = ref(false)
const drawError = ref('')
const cardError = ref(false)
const cardURL = ref('')
const requests = useRequestScope()
const build = ref<PreviewBuild>({
  weapon_level: 10,
  profession: 'swordsman',
  level: 10,
  mastery: 10,
  weapon: '',
  armor: '',
  charm: '',
  skill: '',
})
const quality = ['', '凡品', '精制', '上品', '名器', '传世']
const qualityColor = [
  '',
  'text-muted',
  'text-emerald-500',
  'text-sky-500',
  'text-amber-500',
  'text-rose-400',
]
const currentProfession = computed(() =>
  data.value?.catalog.professions.find((p) => p.id === build.value.profession),
)
function usable(item: PreviewItem) {
  return (
    item.min_level <= build.value.level &&
    (!item.professions?.length ||
      item.professions.includes(build.value.profession))
  )
}
const weapons = computed(() => data.value?.catalog.weapons.filter(usable) ?? [])
const armors = computed(
  () =>
    data.value?.catalog.items.filter((i) => i.kind === 'armor' && usable(i)) ??
    [],
)
const charms = computed(
  () =>
    data.value?.catalog.items.filter((i) => i.kind === 'charm' && usable(i)) ??
    [],
)
const skills = computed(
  () =>
    data.value?.catalog.skills.filter(
      (s) =>
        s.weapon_kind === currentProfession.value?.weapon_kind &&
        s.min_level <= build.value.level,
    ) ?? [],
)
const items = computed(() => [
  ...weapons.value,
  ...armors.value,
  ...charms.value,
])
const selectedEquipment = computed(() =>
  items.value.filter((i) =>
    [build.value.weapon, build.value.armor, build.value.charm].includes(i.id),
  ),
)
async function draw(count: number) {
  drawing.value = true
  drawError.value = ''
  try {
    drawn.value = (
      await wuxiaPreviewApi.draw(build.value.profession, count)
    ).items
  } catch (e) {
    drawError.value = e instanceof Error ? e.message : '试抽失败'
  } finally {
    drawing.value = false
  }
}
const presets = computed(() => data.value?.balance.builds ?? [])
const fields = [
  { key: 'hp', label: '气血' },
  { key: 'attack', label: '攻击' },
  { key: 'defense', label: '防御' },
  { key: 'speed', label: '身法' },
] as const
const traits = [
  { key: 'critical', label: '出招暴击' },
  { key: 'pierce', label: '出招破防' },
  { key: 'guard', label: '受击招架' },
  { key: 'dodge', label: '受击卸力' },
] as const
function percent(n: number) {
  return `${(n * 100).toFixed(1)}%`
}
function label(i: PreviewItem) {
  return `${i.name} · ${i.max_weapon_level ? '无人物等级限制' : `${i.min_level}级`} · ${quality[i.tier]}`
}
function applyPreset(index: number) {
  const preset = presets.value[index]
  if (preset)
    build.value = {
      ...preset.build,
      weapon_level: preset.build.weapon_level ?? 10,
      skill: preset.build.skill ?? '',
    }
}
function sanitize() {
  build.value.mastery = Math.min(build.value.mastery, build.value.level)
  if (!weapons.value.some((i) => i.id === build.value.weapon))
    build.value.weapon = weapons.value[0]?.id ?? ''
  if (!armors.value.some((i) => i.id === build.value.armor))
    build.value.armor = ''
  if (!charms.value.some((i) => i.id === build.value.charm))
    build.value.charm = ''
  if (!skills.value.some((i) => i.id === build.value.skill))
    build.value.skill = ''
}
async function preview() {
  const target = presets.value[opponent.value]
  if (!data.value || !target || !build.value.weapon) return
  const current = requests.begin('preview')
  busy.value = true
  error.value = ''
  result.value = null
  const snapshot = { ...build.value }
  // The opponent is explicitly the selected published test build, including its level.
  try {
    const value = await wuxiaPreviewApi.build(
      snapshot,
      { ...target.build, skill: target.build.skill ?? '' },
      current.signal,
    )
    if (current()) {
      result.value = value
      cardURL.value = wuxiaPreviewApi.card(snapshot)
      cardError.value = false
    }
  } catch (err) {
    if (current()) error.value = err instanceof Error ? err.message : '预览失败'
  } finally {
    if (current()) busy.value = false
  }
}
watch(() => [build.value.profession, build.value.level], sanitize, {
  flush: 'sync',
})
watch(
  [build, opponent],
  () => {
    requests.begin('preview')
    result.value = null
    showCard.value = false
  },
  { deep: true, flush: 'sync' },
)
watchDebounced(
  [build, opponent],
  () => {
    showCard.value = false
    void preview()
  },
  { deep: true, debounce: 250 },
)
onMounted(async () => {
  try {
    data.value = await wuxiaPreviewApi.catalog()
    applyPreset(0)
    const featured = weapons.value.find((w) => w.tier === 5)
    if (featured) {
      build.value.weapon = featured.id
      build.value.weapon_level = 10
    }
    sanitize()
    await preview()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '读取装备设计失败'
  }
})
</script>

<template>
  <section class="space-y-5 rounded-2xl border border-default bg-default p-5">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h2 class="font-semibold text-highlighted">装备试配</h2>
        <p class="mt-1 text-sm text-muted">
          待发布设计 · 不修改群友角色或正式游戏数值
        </p>
      </div>
      <span
        class="rounded-full bg-amber-500/10 px-3 py-1 text-xs text-amber-600"
        >{{ data?.catalog.version ?? '加载中' }}</span
      >
    </div>
    <p v-if="error" role="alert" class="text-sm text-error">{{ error }}</p>
    <template v-if="data">
      <div class="flex flex-wrap gap-2">
        <UButton
          v-for="(preset, i) in presets"
          :key="preset.name"
          size="sm"
          variant="soft"
          @click="applyPreset(i)"
          >{{ preset.name }}</UButton
        >
      </div>
      <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <label class="space-y-1 text-sm"
          >职业<select
            v-model="build.profession"
            class="block w-full rounded-lg border border-default bg-default p-2"
          >
            <option
              v-for="p in data.catalog.professions"
              :key="p.id"
              :value="p.id"
            >
              {{ p.name }}
            </option>
          </select></label
        >
        <label class="space-y-1 text-sm"
          >人物等级<select
            v-model.number="build.level"
            class="block w-full rounded-lg border border-default bg-default p-2"
          >
            <option
              v-for="level in data.catalog.max_level"
              :key="level"
              :value="level"
            >
              {{ level }}级
            </option>
          </select></label
        >
        <label class="space-y-1 text-sm"
          >修为<select
            v-model.number="build.mastery"
            class="block w-full rounded-lg border border-default bg-default p-2"
          >
            <option v-for="n in build.level + 1" :key="n" :value="n - 1">
              {{ n - 1 }}重
            </option>
          </select></label
        >
        <label class="space-y-1 text-sm"
          >武学<select
            v-model="build.skill"
            class="block w-full rounded-lg border border-default bg-default p-2"
          >
            <option value="">不装备</option>
            <option v-for="s in skills" :key="s.id" :value="s.id">
              {{ s.name }}
            </option>
          </select></label
        >
        <label class="space-y-1 text-sm"
          >专属兵器<select
            v-model="build.weapon"
            class="block w-full rounded-lg border border-default bg-default p-2"
          >
            <option v-for="i in weapons" :key="i.id" :value="i.id">
              {{ label(i) }}
            </option>
          </select></label
        >
        <label class="space-y-1 text-sm"
          >护具<select
            v-model="build.armor"
            class="block w-full rounded-lg border border-default bg-default p-2"
          >
            <option value="">不装备</option>
            <option v-for="i in armors" :key="i.id" :value="i.id">
              {{ label(i) }}
            </option>
          </select></label
        >
        <label class="space-y-1 text-sm"
          >佩饰<select
            v-model="build.charm"
            class="block w-full rounded-lg border border-default bg-default p-2"
          >
            <option value="">不装备</option>
            <option v-for="i in charms" :key="i.id" :value="i.id">
              {{ label(i) }}
            </option>
          </select></label
        >
        <label class="space-y-1 text-sm"
          >对手（10级满修为）<select
            v-model.number="opponent"
            class="block w-full rounded-lg border border-default bg-default p-2"
          >
            <option v-for="(p, i) in presets" :key="p.name" :value="i">
              {{ p.name }}
            </option>
          </select></label
        >
      </div>
      <p class="text-xs text-muted">
        兵器只展示品级，保留职业适配，无人物等级限制。
      </p>
      <div class="grid gap-4 sm:grid-cols-3">
        <button
          v-for="i in selectedEquipment"
          :key="i.id"
          class="overflow-hidden rounded-xl border border-default bg-elevated text-left transition hover:border-amber-500/60"
          @click="detail = data.catalog.weapons.find((w) => w.id === i.id) ?? i"
        >
          <img
            v-if="i.image"
            :src="i.image"
            :alt="i.name"
            class="aspect-square w-full object-cover"
          />
          <div class="p-3">
            <p :class="qualityColor[i.tier]" class="text-xs">
              {{ quality[i.tier] }} ·
              {{ i.max_weapon_level ? '兵器' : `${i.min_level}级可用` }}
            </p>
            <h3 class="mt-1 text-lg font-semibold">{{ i.name }}</h3>
            <p class="mt-1 text-xs text-muted">点击查看器物详情</p>
          </div>
        </button>
      </div>
      <p v-if="busy" class="text-sm text-muted" role="status">正在计算搭配…</p>
      <div v-if="result" class="space-y-4" aria-live="polite">
        <div class="grid grid-cols-2 gap-3 sm:grid-cols-4">
          <div
            v-for="f in fields"
            :key="f.key"
            class="rounded-xl bg-elevated p-4"
          >
            <span class="text-xs text-muted">{{ f.label }}</span>
            <p class="mt-1 text-xl font-semibold">
              {{ result.actor.stats[f.key] }}
            </p>
            <p class="text-xs text-muted">
              对手 {{ result.opponent.stats[f.key] }}
            </p>
          </div>
        </div>
        <div class="rounded-xl border border-default p-4">
          <p class="font-medium">
            对阵 {{ presets[opponent]?.name }} · 理论胜率
            {{ percent(result.win_probability) }}
          </p>
          <div class="mt-3 h-2 overflow-hidden rounded bg-elevated">
            <div
              class="h-full bg-primary transition-all"
              :style="{ width: percent(result.win_probability) }"
            />
          </div>
          <p class="mt-2 text-xs text-muted">
            使用当前PK概率模型计算，不是实时抽样结果；低等级挑战10级对手也保留翻盘机会。
          </p>
          <div class="mt-3 flex flex-wrap gap-4 text-sm">
            <span v-for="t in traits" :key="t.key"
              >{{ t.label }} {{ percent(result.events[0][t.key]) }}</span
            >
          </div>
        </div>
        <UButton variant="soft" @click="showCard = !showCard">{{
          showCard ? '收起角色卡' : '生成角色卡预览'
        }}</UButton>
        <img
          v-if="showCard && !cardError"
          :src="cardURL"
          alt="当前装备搭配角色卡"
          class="w-full max-w-4xl rounded-xl"
          @error="cardError = true"
        />
        <p v-if="showCard && cardError" class="text-sm text-error">
          角色卡生成失败，请重新选择搭配后再试。
        </p>
      </div>
      <details class="rounded-xl border border-default p-4">
        <summary class="cursor-pointer font-medium">
          查看当前可用装备与品质（{{ items.length }}件）
        </summary>
        <div class="mt-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
          <article
            v-for="i in items"
            :key="i.id"
            class="rounded-lg bg-elevated p-3"
          >
            <button
              v-if="i.image"
              class="mb-3 block w-full"
              @click="
                detail = data.catalog.weapons.find((w) => w.id === i.id) ?? i
              "
            >
              <img
                :src="i.image"
                :alt="i.name"
                loading="lazy"
                class="aspect-square w-full rounded-lg object-cover"
              />
            </button>
            <div class="flex justify-between gap-2">
              <button
                class="text-left font-medium hover:text-primary"
                @click="
                  detail = data.catalog.weapons.find((w) => w.id === i.id) ?? i
                "
              >
                {{ i.name }}
              </button>
              <span :class="qualityColor[i.tier]" class="text-xs"
                >{{ quality[i.tier] }} ·
                {{ i.max_weapon_level ? '兵器' : `${i.min_level}级` }}</span
              >
            </div>
            <div class="mt-2 flex flex-wrap gap-2 text-xs text-muted">
              <template v-for="f in fields" :key="f.key"
                ><span
                  v-if="
                    f.key === 'attack'
                      ? i.attack || i.stats?.attack
                      : i.stats?.[f.key]
                  "
                  >{{ f.label }} +{{
                    f.key === 'attack'
                      ? i.attack || i.stats?.attack
                      : i.stats?.[f.key]
                  }}</span
                ></template
              ><template v-for="t in traits" :key="t.key"
                ><span v-if="i.traits?.[t.key]"
                  >{{ t.label }}加成 {{ percent(i.traits[t.key]) }}</span
                ></template
              >
            </div>
          </article>
        </div>
      </details>
      <section
        class="space-y-4 rounded-xl border border-amber-500/25 bg-amber-500/5 p-4"
      >
        <h3 class="font-semibold">铸兵阁 · 武器试抽</h3>
        <p class="text-sm text-muted">
          仅模拟，不扣银两、不入背包；先抽品质，再在本职业同品质武器中等概率抽取。每次独立，暂无保底。
        </p>
        <div class="flex flex-wrap gap-3 text-sm">
          <span
            v-for="q in data.weapon_chances"
            :key="q.tier"
            :class="qualityColor[q.tier]"
            >{{ q.name }} {{ (q.weight / 100).toFixed(1) }}%</span
          >
        </div>
        <div class="flex gap-2">
          <UButton :disabled="drawing" @click="draw(1)">试抽一次</UButton
          ><UButton :disabled="drawing" variant="soft" @click="draw(10)"
            >试抽十次</UButton
          >
        </div>
        <p v-if="drawError" role="alert" class="text-error">{{ drawError }}</p>
        <div class="grid grid-cols-2 gap-3 sm:grid-cols-5">
          <button
            v-for="(i, n) in drawn"
            :key="n"
            class="overflow-hidden rounded-lg border border-default bg-default text-left"
            @click="
              detail = data.catalog.weapons.find((w) => w.id === i.id) ?? i
            "
          >
            <img
              :src="i.image"
              :alt="i.name"
              class="aspect-square w-full object-cover"
            />
            <div class="p-2">
              <p :class="qualityColor[i.tier]" class="text-xs">
                {{ quality[i.tier] }}
              </p>
              <p class="text-sm">{{ i.name }}</p>
            </div>
          </button>
        </div>
      </section>
      <details class="rounded-xl border border-default p-4">
        <summary class="cursor-pointer font-medium">
          平衡验证 · 9,000场固定种子切磋
        </summary>
        <p class="my-3 text-sm text-muted">
          {{ data.balance.scope }}。每组600场，不代表胜率精确到小数点后一位。
        </p>
        <div class="overflow-x-auto">
          <table class="w-full text-left text-sm">
            <thead>
              <tr>
                <th class="p-2">对局（左方胜率）</th>
                <th>理论</th>
                <th>实测</th>
                <th>样本</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="m in data.balance.matches"
                :key="`${m.left}-${m.right}`"
                class="border-t border-default"
              >
                <td class="p-2">
                  {{ presets[m.left]?.name }} / {{ presets[m.right]?.name }}
                </td>
                <td>{{ percent(m.expected) }}</td>
                <td>{{ percent(m.observed) }}</td>
                <td>{{ m.samples }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </details>
      <div
        v-if="detail"
        class="fixed inset-0 z-50 flex items-center justify-center overflow-y-auto bg-black/80 p-4"
        @click.self="detail = null"
        @keydown.esc="detail = null"
      >
        <section
          role="dialog"
          aria-modal="true"
          :aria-label="detail.name"
          class="relative max-h-[90vh] w-full max-w-3xl overflow-y-auto rounded-2xl border border-amber-500/30 bg-default p-5"
        >
          <button
            class="absolute right-4 top-4 rounded-lg bg-default px-3 py-2"
            aria-label="关闭装备详情"
            @click="detail = null"
          >
            关闭
          </button>
          <div class="grid gap-5 sm:grid-cols-2">
            <img
              :src="detail.image"
              :alt="detail.name"
              class="aspect-square w-full rounded-xl object-cover"
            />
            <div class="space-y-4 pt-10">
              <p :class="qualityColor[detail.tier]">
                {{ quality[detail.tier] }} ·
                {{
                  detail.max_weapon_level
                    ? '兵器 · 无人物等级限制'
                    : `${detail.min_level}级可用`
                }}
              </p>
              <h3 class="text-3xl font-semibold">{{ detail.name }}</h3>
              <p class="text-sm leading-7 text-muted">
                {{ detail.description }}
              </p>
              <p class="text-xs text-muted">
                {{
                  detail.max_weapon_level
                    ? '以下为该武器的设计属性。'
                    : '装备属性'
                }}
              </p>
              <p v-for="f in fields" :key="f.key">
                {{ f.label }} +{{
                  f.key === 'attack'
                    ? (detail.attack ?? detail.stats?.attack ?? 0)
                    : (detail.stats?.[f.key] ?? 0)
                }}
              </p>
              <p
                v-for="t in traits.filter((t) => detail?.traits?.[t.key])"
                :key="t.key"
              >
                {{ t.label }}加成 {{ percent(detail.traits[t.key]) }}
              </p>
            </div>
          </div>
        </section>
      </div>
    </template>
  </section>
</template>
