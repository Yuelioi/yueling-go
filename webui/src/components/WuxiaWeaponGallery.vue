<script setup lang="ts">
import { computed, ref } from 'vue'
import weapons from '../data/weapon-gallery.json'
import universalEquipment from '../data/universal-equipment-gallery.json'
const category = ref('armor')
const categories = [
  { id: 'weapon', name: '兵器' },
  { id: 'armor', name: '通用护具' },
  { id: 'accessory', name: '通用饰品' },
]
const visibleItems = computed(() =>
  category.value === 'weapon'
    ? weapons
    : universalEquipment.filter(
        (i) => i.kind === (category.value === 'armor' ? '护具' : '饰品'),
      ),
)
function kindLabel(kind: string) {
  return ['护具', '饰品'].includes(kind) ? `全职业通用 · ${kind}` : kind
}
const selected = ref<(typeof weapons)[number] | null>(null)
const open = computed({
  get: () => selected.value !== null,
  set: (value: boolean) => {
    if (!value) selected.value = null
  },
})
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
  <div class="weapon-gallery space-y-12">
    <header class="space-y-3 border-b border-default pb-7">
      <p class="text-xs tracking-[0.35em] text-amber-200/60">
        月灵江湖 · 百器录
      </p>
      <h1 class="font-serif text-3xl text-stone-100 sm:text-4xl">一器一风骨</h1>
      <p class="max-w-2xl text-sm leading-7 text-stone-400">
        兵器、通用护具、通用饰品：每类五个品级，每级三件。点击查看大图与来历。此页为美术图鉴，不代表已获得装备。
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
      <p v-if="category !== 'weapon'" class="text-xs text-stone-400">
        全职业通用 · 以材质、形制和品级区分，不设职业专属
      </p>
      <nav class="flex flex-wrap gap-2" aria-label="品级导航">
        <a
          v-for="q in qualities"
          :key="q.tier"
          :href="`#quality-${q.tier}`"
          :style="{ color: q.color }"
          class="rounded-full border border-current/20 px-4 py-1.5 text-sm"
          >{{ q.name }}</a
        >
      </nav>
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
          </h2>
        </div>
        <p class="text-xs text-stone-400">{{ q.description }}</p>
      </header>
      <div class="grid gap-5 sm:grid-cols-3">
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
              :src="w.image"
              :alt="w.name"
              :loading="q.tier === 1 ? 'eager' : 'lazy'"
              class="relative h-full w-full object-contain transition-transform duration-500 group-hover:scale-[1.035] motion-reduce:transform-none motion-reduce:transition-none"
            />
            <span
              class="absolute left-3 top-3 rounded-sm border border-current/40 bg-black/75 px-2.5 py-1 text-xs"
              :style="{ color: q.color }"
              >{{ q.name }}</span
            >
          </div>
          <div class="relative space-y-2 bg-[#151511] p-4">
            <div class="flex items-center justify-between gap-2">
              <h3 class="font-serif text-xl text-stone-100">{{ w.name }}</h3>
              <span class="text-xs text-stone-400">{{
                kindLabel(w.kind)
              }}</span>
            </div>
            <p
              class="min-h-12 whitespace-pre-line text-xs leading-6 text-stone-400"
              :class="w.tier === 5 ? 'font-serif tracking-wider' : ''"
            >
              {{ w.description }}
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
              :src="selected.image"
              :alt="selected.name"
              class="relative h-full w-full object-contain"
            />
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
