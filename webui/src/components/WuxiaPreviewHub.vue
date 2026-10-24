<script setup lang="ts">
import { computed, ref } from 'vue'
import catalog from '../../../services/wuxia/internal/content/six_professions.json'
import { useEventListener } from '@vueuse/core'
import WuxiaCommandGuide from './WuxiaCommandGuide.vue'
import WuxiaWeaponGallery from './WuxiaWeaponGallery.vue'
import WuxiaMartialGallery from './WuxiaMartialGallery.vue'
import WuxiaProfessionDesign from './WuxiaProfessionDesign.vue'
const sections = [
  { id: 'screens', name: '游戏界面' },
  { id: 'equipment', name: '装备图鉴' },
  { id: 'martial', name: '武学图鉴' },
  { id: 'professions', name: '职业与成长' },
  { id: 'balance', name: '对战验证' },
]
const hashSection = () =>
  sections.some((s) => s.id === window.location.hash.slice(1))
    ? window.location.hash.slice(1)
    : 'screens'
const section = ref(hashSection())
useEventListener(window, 'hashchange', () => {
  section.value = hashSection()
})
const selectSection = (id: string) => {
  section.value = id
  window.location.hash = id
}
const group = ref('全部')
const groups = ['全部', '侠客', '历练', '切磋', '寻宝', '赌坊', '行囊与指引']
const screens = [
  ...[
    ['candidate-inventory', '江湖背包 · 物品与数量'],
    ['candidate-bag-empty', '江湖背包 · 空行囊'],
    ['candidate-mind-list', '江湖心法 · 所学详情'],
    ['candidate-help', '江湖帮助 · 常用指令'],
  ].map(([id, name]) => ({
    id: id!,
    name: name!,
    group: '行囊与指引',
    note: '与 QQ 回复共用卡片渲染，长列表自动分页',
  })),
  ...catalog.professions.flatMap((p) => [
    {
      id: `candidate-profile-${p.id}`,
      name: `${p.name} · 全部配齐`,
      group: '侠客',
      note: '传世兵器、护具、饰品与武学',
    },
    {
      id: `candidate-empty-${p.id}`,
      name: `${p.name} · 初入江湖`,
      group: '侠客',
      note: '初始兵器，空装备与武学槽',
    },
    {
      id: `candidate-duel-${p.id}`,
      name: `${p.name} · 群侠切磋`,
      group: '切磋',
      note: '使用当前候选属性与武技的真实对战',
    },
    {
      id: `candidate-adventure-${p.id}`,
      name: `${p.name} · 江湖历练`,
      group: '历练',
      note: '装备收益饰品后的历练结算',
    },
    {
      id: `candidate-draw-${p.id}`,
      name: `${p.name} · 五品兵器`,
      group: '寻宝',
      note: '各品级展示样例，不代表一次抽取必得五品',
    },
  ]),
  {
    id: 'candidate-casino',
    name: '江湖赌场 · 豹子',
    group: '赌坊',
    note: '赌金100，666返10倍，饰品不改变赌场返还',
  },
]

const visible = computed(() =>
  screens.filter((s) => group.value === '全部' || s.group === group.value),
)
const selected = ref<(typeof screens)[number] | null>(null)
const open = computed({
  get: () => selected.value !== null,
  set: (value: boolean) => {
    if (!value) selected.value = null
  },
})
const source = (id: string) =>
  `/wuxia-preview/${id}.webp?v=${catalog.version}-collection9`
</script>

<template>
  <main class="mx-auto max-w-7xl space-y-8 px-4 py-8 sm:px-8">
    <header class="space-y-4 border-b border-stone-700/60 pb-6">
      <p class="text-xs tracking-[0.3em] text-amber-200/60">
        月灵江湖 · 本地预览
      </p>
      <h1 class="font-serif text-3xl text-stone-100">江湖画卷</h1>
      <p class="max-w-3xl text-sm leading-7 text-stone-400">
        在这里一起查看游戏界面、装备武学、职业成长与对战验证。界面样张由当前卡片渲染器生成，使用示例角色，不连接群聊、不消耗银两。
      </p>
      <nav class="flex flex-wrap gap-3" aria-label="预览内容">
        <button
          v-for="item in sections"
          :key="item.id"
          :aria-pressed="section === item.id"
          class="rounded-lg border px-5 py-2 text-sm"
          :class="
            section === item.id
              ? 'border-amber-200/50 bg-amber-200/10 text-amber-100'
              : 'border-stone-700 text-stone-400'
          "
          @click="selectSection(item.id)"
        >
          {{ item.name }}
        </button>
      </nav>
    </header>
    <template v-if="section === 'screens'">
      <div class="flex flex-wrap gap-2" aria-label="界面分类">
        <button
          v-for="item in groups"
          :key="item"
          :aria-pressed="group === item"
          class="rounded-full border px-4 py-1.5 text-sm"
          :class="
            group === item
              ? 'border-amber-200/50 text-amber-100'
              : 'border-stone-700 text-stone-400'
          "
          @click="group = item"
        >
          {{ item }}
        </button>
      </div>
      <p class="text-xs leading-6 text-stone-400">
        点击卡片查看大图。六职业使用同一套候选装备、武学与成长数据，页面由实际卡片渲染器生成。
      </p>
      <div class="grid gap-6 md:grid-cols-2">
        <button
          v-for="screen in visible"
          :key="screen.id"
          class="overflow-hidden rounded-xl border border-stone-700/60 bg-[#14151b] text-left transition hover:border-amber-200/50"
          :aria-label="`查看${screen.name}`"
          @click="selected = screen"
        >
          <img
            :src="source(screen.id)"
            :alt="screen.name"
            class="aspect-[6/5] w-full bg-[#0d1018] object-contain"
            loading="lazy"
          />
          <div class="space-y-2 p-5">
            <h2 class="font-serif text-xl text-stone-100">{{ screen.name }}</h2>
            <p class="text-sm text-stone-400">{{ screen.note }}</p>
          </div>
        </button>
      </div>
    </template>
    <WuxiaMartialGallery v-else-if="section === 'martial'" />
    <WuxiaWeaponGallery v-else-if="section === 'equipment'" />
    <WuxiaProfessionDesign v-else :balance="section === 'balance'" />
    <WuxiaCommandGuide />
    <UModal
      v-model:open="open"
      :title="selected?.name"
      :description="selected?.note"
      :ui="{ content: 'sm:max-w-6xl' }"
    >
      <template #body
        ><template v-if="selected"
          ><img
            :src="source(selected.id)"
            :alt="selected.name"
            class="mx-auto max-h-[calc(100dvh-230px)] w-auto max-w-full rounded-lg object-contain"
          /><a
            :href="source(selected.id)"
            target="_blank"
            rel="noopener"
            class="mt-4 inline-block text-sm text-amber-200"
            >打开原尺寸图片 ↗</a
          ></template
        ></template
      >
    </UModal>
  </main>
</template>
