<script setup lang="ts">
import { computed, ref } from 'vue'
import WuxiaContentCoverage from './WuxiaContentCoverage.vue'
import artManifest from '../data/wuxia-art.json'
import catalog from '../../../services/wuxia/internal/content/six_professions.json'
const skills = catalog.skills
const filter = ref('全部')
const tier = ref(0)
const filters = ['全部', ...catalog.professions.map((p) => p.name), '心法']
const profession = (s: (typeof skills)[number]) =>
  s.kind === 'mind'
    ? '心法'
    : (catalog.professions.find((p) => p.weapon_kind === s.weapon_kind)?.name ??
      '')
const visible = computed(() =>
  skills
    .filter(
      (s) =>
        (filter.value === '全部' || profession(s) === filter.value) &&
        (tier.value === 0 || s.tier === tier.value),
    )
    .sort((a, b) => b.tier - a.tier),
)
const names = ['入门', '精修', '上乘', '绝学', '传世']
const colors = ['#A8A29E', '#6FAE8A', '#6FA7C9', '#B393D0', '#E4B75F']
const selected = ref<(typeof skills)[number] | null>(null)
const open = computed({
  get: () => selected.value !== null,
  set: (v: boolean) => {
    if (!v) selected.value = null
  },
})
const martialArt: Record<string, string> = artManifest.martial
const art = (id: string) => martialArt[id] || ''
const background = (tier: number) => ({
  backgroundImage: `url('/equipment-v2/quality-${tier}-background.webp')`,
  backgroundSize: 'cover',
})
function mechanics(s: (typeof skills)[number]) {
  if (s.kind === 'mind') {
    const labels: Record<string, string> = {
      hp: '气血',
      attack: '攻击',
      defense: '防御',
      speed: '身法',
    }
    const bonus = Object.entries(s.bonus ?? {})
      .map(([k, v]) => `${labels[k]} +${v}%`)
      .join(' · ')
    const effects: Record<string, string> = {
      recover: '第三次自身出手回复 5% 最大气血，每场一次。',
      opening: '首次武技伤害提高 10%。',
      protect: '首次有效受击伤害降低 20%。',
      focus: '首次武技额外穿透 15% 防御。',
    }
    return `${bonus}；${effects[s.mind_effect ?? ''] ?? ''}`
  }
  return `${s.ready_turn && s.ready_turn > 1 ? `第 ${s.ready_turn} 次自身出手起，` : ''}${s.every ? `每第 ${s.every} 次出手` : `每次出手独立判定，${Math.round((s.chance ?? 0) * 100)}% 概率`}施展${s.guarantee_turn ? `，最迟第 ${s.guarantee_turn} 次自身出手首发` : ''}；${s.hits} 段，每段 ${Math.round((s.damage ?? 0) * 100)}% 伤害${s.pierce ? `，穿透 ${Math.round(s.pierce * 100)}% 防御` : ''}。${s.limit ? `每场最多 ${s.limit} 次。` : ''}${s.limit === 1 || !s.cooldown ? '' : `再次施展前需普攻 ${s.cooldown} 次，期间正常出手。`}`
}
</script>
<template>
  <section class="space-y-8">
    <header class="space-y-3">
      <p class="text-xs tracking-[0.35em] text-amber-200/60">
        月灵江湖 · 武学藏经阁
      </p>
      <h1 class="font-serif text-3xl text-stone-100">一招见天地</h1>
      <p class="text-sm leading-7 text-stone-400">
        六职业武技与通用心法。修习属性与实战效果直接列出，点击原画可放大查看。当前为候选内容预览。
      </p>
      <nav class="flex flex-wrap gap-3" aria-label="武学分类">
        <button
          v-for="f in filters"
          :key="f"
          :aria-pressed="filter === f"
          class="rounded-lg border px-5 py-2 text-sm"
          :class="
            filter === f
              ? 'border-amber-200/50 bg-amber-200/10 text-amber-100'
              : 'border-stone-700 text-stone-400'
          "
          @click="filter = f"
        >
          {{ f }}
        </button>
      </nav>
      <nav class="flex flex-wrap gap-2" aria-label="武学品级">
        <button
          v-for="(label, n) in ['全部品级', ...names]"
          :key="label"
          :aria-pressed="tier === n"
          class="rounded-full border px-4 py-1.5 text-xs"
          :class="tier === n ? 'border-current bg-white/5' : 'border-stone-700'"
          :style="{ color: n ? colors[n - 1] : '#a8a29e' }"
          @click="tier = n"
        >
          {{ label }}
        </button>
      </nav>
      <p class="text-sm text-amber-100" aria-live="polite">
        {{ filter }} · {{ visible.length }} 门武学
      </p>
      <WuxiaContentCoverage />
    </header>
    <p v-if="visible.length === 0" class="py-12 text-center text-stone-400">
      此品级尚无收录
    </p>
    <div class="grid gap-6 sm:grid-cols-2 lg:grid-cols-3">
      <button
        v-for="s in visible"
        :key="s.id"
        :aria-label="`查看${s.name}`"
        class="overflow-hidden rounded-sm border bg-[#151511] text-left transition hover:-translate-y-1 motion-reduce:transform-none"
        :style="{ borderColor: colors[s.tier - 1] + '66' }"
        @click="selected = s"
      >
        <div class="relative aspect-square" :style="background(s.tier)">
          <div
            v-if="!art(s.id)"
            class="flex aspect-square items-center justify-center text-sm text-stone-400"
          >
            原画待绘
          </div>
          <img
            v-if="art(s.id)"
            :src="art(s.id)"
            :alt="s.name"
            class="h-full w-full object-contain p-5"
            loading="lazy"
          />
          <span
            class="absolute left-4 top-4 border border-current/40 bg-black/70 px-3 py-1 text-xs"
            :style="{ color: colors[s.tier - 1] }"
            >{{ names[s.tier - 1] }}</span
          >
        </div>
        <div class="space-y-3 p-6">
          <div class="flex items-center justify-between gap-2">
            <h2 class="font-serif text-2xl text-stone-100">{{ s.name }}</h2>
            <span class="text-xs text-stone-400">{{
              profession(s) === '心法' ? '通用心法' : profession(s) + '武技'
            }}</span>
          </div>
          <p
            class="min-h-14 whitespace-pre-line font-serif text-sm leading-7 text-stone-400"
            :class="s.tier === 5 ? 'tracking-widest' : ''"
          >
            {{ s.description }}
          </p>
          <p class="border-t border-stone-700/70 pt-3 text-sm leading-7 text-stone-200" data-testid="skill-mechanics">
            {{ mechanics(s) }}
          </p>
          <p v-if="s.affinity" class="text-sm leading-6 text-amber-200/80">
            {{ s.affinity }}兵刃相合 · 武技伤害 +10%
          </p>
        </div>
      </button>
    </div>
    <UModal
      v-model:open="open"
      :title="selected?.name"
      description="武学详情"
      :ui="{ content: 'sm:max-w-4xl' }"
    >
      <template #body>
        <div v-if="selected" class="grid gap-8 sm:grid-cols-2">
          <div
            class="aspect-square rounded-sm"
            :style="background(selected.tier)"
          >
            <div
              v-if="!art(selected.id)"
              class="flex h-full items-center justify-center text-stone-400"
            >
              原画待绘
            </div>
            <img
              v-if="art(selected.id)"
              :src="art(selected.id)"
              :alt="selected.name"
              class="h-full w-full object-contain p-4"
            />
          </div>
          <div class="space-y-6 self-center">
            <p
              class="text-sm tracking-widest"
              :style="{ color: colors[selected.tier - 1] }"
            >
              {{ names[selected.tier - 1] }} ·
              {{
                profession(selected) === '心法'
                  ? '全职业通用'
                  : profession(selected) + '专属'
              }}
            </p>
            <h2 class="font-serif text-3xl">{{ selected.name }}</h2>
            <div
              v-if="art(selected.id)"
              class="flex items-center gap-4 rounded-lg border border-stone-700/60 bg-[#132330] p-4"
            >
              <img
                v-if="art(selected.id)"
                :src="art(selected.id)"
                :alt="`${selected.name}战斗图标`"
                class="h-12 w-12 object-contain"
              />
              <div>
                <p class="text-sm text-stone-200">实战图标</p>
                <p class="mt-1 text-xs text-stone-400">同一招式，同一幅画</p>
              </div>
            </div>
            <p
              class="whitespace-pre-line font-serif text-lg leading-9 tracking-wider text-stone-300"
            >
              {{ selected.description }}
            </p>
            <p
              class="border-t border-stone-700 pt-5 text-sm leading-7 text-stone-400"
            >
              {{ mechanics(selected) }}
            </p>
            <p v-if="selected.affinity" class="text-sm text-amber-200/80">
              {{ selected.affinity }}兵刃相合 · 武技伤害 +10%
            </p>
          </div>
        </div>
      </template>
    </UModal>
  </section>
</template>
