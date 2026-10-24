<script setup lang="ts">
import { computed, ref } from 'vue'
import WuxiaContentCoverage from './WuxiaContentCoverage.vue'
import weaponArt from '../data/weapon-gallery.json'
import universalArt from '../data/universal-equipment-gallery.json'
import catalog from '../../../services/wuxia/internal/content/six_professions.json'
const category = ref('weapon')
const profession = ref('全部')
const overview = computed(
  () => category.value === 'weapon' && profession.value === '全部',
)
const categories = [
  { id: 'weapon', name: '兵器' },
  { id: 'armor', name: '通用护具' },
  { id: 'accessory', name: '通用饰品' },
]
const labels: Record<string, string> = {
  hp: '气血',
  attack: '攻击',
  defense: '防御',
  speed: '身法',
  critical: '暴击',
  critical_damage: '暴伤',
  dodge: '闪避',
  guard: '格挡',
  pierce: '穿透',
  combo: '连击',
  poison: '附毒',
  life_steal: '吸血',
}
const describe = (stats: object, traits: object) =>
  [
    ...Object.entries(stats)
      .filter(([, v]) => v)
      .map(([k, v]) => `${labels[k]} +${v}`),
    ...Object.entries(traits)
      .filter(([, v]) => v)
      .map(([k, v]) => `${labels[k]} +${+(v * 100).toFixed(2)}%`),
  ].join(' · ')
const weapons = catalog.weapons.map((w) => {
  const art = weaponArt.find((a) => a.name === w.name)
  return {
    ...w,
    image: art?.image || ('image' in w ? String(w.image) : ''),
    background: `/equipment-v2/quality-${w.tier}-background.webp`,
    attributes: describe({ ...w.stats, attack: w.attack }, w.traits),
  }
})
const universalEquipment = catalog.items
  .filter(
    (i) =>
      (i.kind === 'armor' || i.kind === 'charm') && 'image' in i && i.image,
  )
  .map((i) => ({
    id: i.id,
    name: i.name,
    tier: i.tier,
    kind: i.kind === 'armor' ? '护具' : '饰品',
    description: i.description || '',
    image: String(i.image),
    background:
      universalArt.find((a) => a.name === i.name)?.background ||
      `/equipment-v2/quality-${i.tier}-background.webp`,
    attributes: [
      describe(i.stats || {}, i.traits || {}),
      ...('adventure_bonus' in i && i.adventure_bonus
        ? [
            `历练银两 +${i.adventure_bonus.silver_percent}%`,
            `历练经验 +${i.adventure_bonus.experience_percent}%`,
          ]
        : []),
    ]
      .filter(Boolean)
      .join(' · '),
  }))
const weaponRank = new Map<string, number>()
const seenWeaponGroups = new Map<string, number>()
for (const w of weapons) {
  const key = `${w.kind}:${w.tier}`
  const variant = seenWeaponGroups.get(key) ?? 0
  seenWeaponGroups.set(key, variant + 1)
  weaponRank.set(
    w.id,
    variant * catalog.professions.length +
      catalog.professions.findIndex((p) => p.weapon_kind === w.kind),
  )
}
const visibleItems = computed(() =>
  category.value === 'weapon'
    ? weapons
        .filter(
          (w) => profession.value === '全部' || w.kind === profession.value,
        )
        .sort(
          (a, b) => (weaponRank.get(a.id) ?? 0) - (weaponRank.get(b.id) ?? 0),
        )
    : universalEquipment.filter(
        (i) => i.kind === (category.value === 'armor' ? '护具' : '饰品'),
      ),
)
function kindLabel(kind: string) {
  return ['护具', '饰品'].includes(kind) ? `全职业通用 · ${kind}` : kind
}
const selected = ref<(typeof visibleItems.value)[number] | null>(null)
const open = computed({
  get: () => selected.value !== null,
  set: (v: boolean) => {
    if (!v) selected.value = null
  },
})
function scrollQuality(tier: number) {
  document
    .getElementById(`quality-${tier}`)
    ?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}
const qualities = [
  {
    tier: 1,
    name: '凡品',
    color: '#A8A29E',
    title: '朴材素作 · 寻常器物',
    description: '朴素的材料，实用的形制。',
  },
  {
    tier: 2,
    name: '精制',
    color: '#6FAE8A',
    title: '百炼精工 · 匠心所成',
    description: '看得见的用料与工艺，不靠繁饰。',
  },
  {
    tier: 3,
    name: '上品',
    color: '#6FA7C9',
    title: '各有所长 · 自成风骨',
    description: '从轮廓到材质，各具风骨。',
  },
  {
    tier: 4,
    name: '名器',
    color: '#B393D0',
    title: '器有其名 · 江湖有闻',
    description: '独有的形制，和一段值得记住的来历。',
  },
  {
    tier: 5,
    name: '传世',
    color: '#E4B75F',
    title: '山河入器 · 传说留形',
    description: '器物自有气象，古金只作点缀。',
  },
]
function quality(tier: number) {
  return qualities[tier - 1]!
}
</script>

<template>
  <div class="weapon-gallery space-y-8">
    <header class="space-y-3 border-b border-default pb-7">
      <p class="text-xs tracking-[0.35em] text-amber-200/60">
        月灵江湖 · 百器录
      </p>
      <h1 class="font-serif text-3xl text-stone-100 sm:text-4xl">一器一风骨</h1>
      <p class="max-w-2xl text-sm leading-7 text-stone-400">
        六职业兵器与通用护具、饰品。点击查看器物大图、来历与词条。当前为候选内容预览，尚未启用为正式玩法。
      </p>
      <nav class="flex flex-wrap gap-2 py-2" aria-label="装备分类">
        <button
          v-for="c in categories"
          :key="c.id"
          :aria-pressed="category === c.id"
          class="rounded-lg border px-5 py-2 text-sm transition"
          :class="
            category === c.id
              ? 'border-amber-300/50 bg-amber-300/10 text-amber-100'
              : 'border-stone-700 text-stone-400 hover:text-stone-100'
          "
          @click="category = c.id"
        >
          {{ c.name }}
        </button>
      </nav>
      <nav
        v-if="category === 'weapon'"
        class="flex flex-wrap gap-2"
        aria-label="兵器职业"
      >
        <button
          v-for="p in [
            { name: '全部', weapon_kind: '全部' },
            ...catalog.professions,
          ]"
          :key="p.weapon_kind"
          :aria-pressed="profession === p.weapon_kind"
          class="rounded-lg border border-stone-700 px-4 py-2 text-sm"
          :class="
            profession === p.weapon_kind
              ? 'bg-amber-200/10 text-amber-100'
              : 'text-stone-400'
          "
          @click="profession = p.weapon_kind"
        >
          {{ p.name }}
        </button>
      </nav>
      <p v-if="category !== 'weapon'" class="text-xs text-stone-400">
        全职业通用 · 以材质、形制和品级区分，不设职业专属
      </p>
      <nav class="flex flex-wrap gap-2" aria-label="品级导航">
        <button
          v-for="q in qualities"
          :key="q.tier"
          @click="scrollQuality(q.tier)"
          :style="{ color: q.color }"
          class="rounded-full border border-current/20 px-4 py-1.5 text-sm"
        >
          {{ q.name }}
        </button>
      </nav>
      <p class="text-sm text-amber-100" aria-live="polite">
        {{
          category === 'weapon'
            ? profession === '全部'
              ? `六职业 · ${visibleItems.length} 件兵器`
              : `${catalog.professions.find((p) => p.weapon_kind === profession)?.name} · ${visibleItems.length} 件兵器`
            : `全职业通用 · ${visibleItems.length} 件${category === 'armor' ? '护具' : '饰品'}`
        }}
      </p>
      <WuxiaContentCoverage />
    </header>
    <section
      v-for="q in qualities"
      :id="`quality-${q.tier}`"
      :key="q.tier"
      class="scroll-mt-6 space-y-5"
    >
      <header class="flex flex-wrap items-end justify-between gap-2">
        <div>
          <p :style="{ color: q.color }" class="mb-2 text-xs tracking-[0.2em]">
            {{ q.title }}
          </p>
          <h2 :style="{ color: q.color }" class="font-serif text-2xl">
            {{ q.name }}
            <span class="ml-2 text-sm font-sans"
              >{{
                visibleItems.filter((w) => w.tier === q.tier).length
              }}
              件</span
            >
          </h2>
        </div>
        <p class="text-xs text-stone-400">{{ q.description }}</p>
      </header>
      <div
        class="grid grid-cols-2 gap-3"
        :class="overview ? 'sm:grid-cols-6' : 'sm:grid-cols-3'"
      >
        <button
          v-for="w in visibleItems.filter((w) => w.tier === q.tier)"
          :key="w.id"
          :aria-label="`查看${w.name}，${q.name}`"
          class="weapon-card group overflow-hidden text-left"
          :class="`tier-${q.tier}`"
          :style="{ '--quality-color': q.color }"
          @click="selected = w"
        >
          <div
            class="weapon-scene relative aspect-square overflow-hidden"
            :style="{ backgroundImage: `url('${w.background}')` }"
          >
            <div class="absolute inset-0 bg-black/15" />
            <img
              v-if="w.image"
              :src="w.image"
              :alt="w.name"
              :loading="q.tier === 1 ? 'eager' : 'lazy'"
              class="relative h-full w-full object-contain transition-transform duration-500 group-hover:scale-[1.035] motion-reduce:transform-none motion-reduce:transition-none"
            />
            <div
              v-else
              class="relative flex h-full flex-col items-center justify-center gap-3 text-stone-400"
            >
              <span class="font-serif text-2xl">{{ w.name }}</span
              ><span class="text-xs">原画待绘</span>
            </div>
            <span
              class="absolute left-3 top-3 rounded-sm border border-current/40 bg-black/75 px-2.5 py-1 text-xs"
              :style="{ color: q.color }"
              >{{ q.name }}</span
            >
          </div>
          <div
            class="relative space-y-2 bg-[#151511]"
            :class="overview ? 'p-2' : 'p-4'"
          >
            <div
              class="flex gap-2"
              :class="overview ? 'flex-col' : 'items-center justify-between'"
            >
              <h3
                class="font-serif text-stone-100"
                :class="overview ? 'text-sm' : 'text-xl'"
              >
                {{ w.name }}
              </h3>
              <span class="text-xs text-stone-400">{{
                category === 'weapon'
                  ? catalog.professions.find((p) => p.weapon_kind === w.kind)
                      ?.name
                  : kindLabel(w.kind)
              }}</span>
            </div>
            <p
              v-if="!overview"
              class="min-h-12 whitespace-pre-line text-xs leading-6 text-stone-400"
              :class="w.tier === 5 ? 'font-serif tracking-wider' : ''"
            >
              {{ w.description }}
            </p>
            <p v-if="!overview" class="text-xs leading-6 text-amber-100/70">
              {{ w.attributes }}
            </p>
          </div>
        </button>
      </div>
    </section>
    <UModal
      v-model:open="open"
      :title="selected?.name"
      :description="
        selected
          ? `${quality(selected.tier).name} · ${kindLabel(selected.kind)}`
          : ''
      "
      :ui="{ content: 'sm:max-w-4xl' }"
    >
      <template #body>
        <div v-if="selected" class="grid gap-6 sm:grid-cols-[1.4fr_1fr]">
          <div
            class="weapon-scene relative aspect-square overflow-hidden rounded-lg"
            :style="{ backgroundImage: `url('${selected.background}')` }"
          >
            <div class="absolute inset-0 bg-black/15" />
            <img
              v-if="selected.image"
              :src="selected.image"
              :alt="selected.name"
              class="relative h-full w-full object-contain"
            />
            <div
              v-else
              class="relative flex h-full items-center justify-center text-stone-400"
            >
              原画待绘
            </div>
          </div>
          <div class="space-y-5 self-center">
            <p
              class="text-xs tracking-widest"
              :style="{ color: quality(selected.tier).color }"
            >
              {{ quality(selected.tier).name }} · {{ kindLabel(selected.kind) }}
            </p>
            <h2 class="font-serif text-3xl">{{ selected.name }}</h2>
            <p
              class="whitespace-pre-line text-sm leading-8 text-muted"
              :class="selected.tier === 5 ? 'font-serif tracking-wider' : ''"
            >
              {{ selected.description }}
            </p>
            <p class="text-sm leading-7 text-amber-100/80">
              {{ selected.attributes }}
            </p>
            <p class="text-xs leading-6 text-stone-400">
              概率与暴伤词条为额外加成，基础暴伤175%；附毒和吸血按实际直接伤害计算。通用装备不限制职业。
            </p>
          </div>
        </div>
      </template>
    </UModal>
  </div>
</template>

<style scoped>
.weapon-scene {
  background-size: cover;
  background-position: center;
}
.weapon-card {
  border: 1px solid color-mix(in srgb, var(--quality-color) 45%, transparent);
  border-radius: 4px;
}
.weapon-card:focus-visible {
  outline: 2px solid var(--quality-color);
  outline-offset: 4px;
}
.tier-2 {
  border: 3px double color-mix(in srgb, var(--quality-color) 55%, transparent);
}
.tier-3 {
  border-top: 3px solid var(--quality-color);
}
.tier-4 {
  border-width: 2px;
  box-shadow: inset 0 0 0 3px #151511;
  border-radius: 10px 2px;
}
.tier-5 {
  border: 3px double var(--quality-color);
  border-radius: 12px 2px;
}
</style>
