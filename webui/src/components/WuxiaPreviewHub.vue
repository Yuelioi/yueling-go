<script setup lang="ts">
import { computed, ref } from 'vue'
import WuxiaWeaponGallery from './WuxiaWeaponGallery.vue'
const section = ref('screens')
const group = ref('全部')
const groups = ['全部', '侠客', '历练', '切磋', '寻宝', '赌坊']
const screens = [
  {id:'casino-win',name:'赌坊 · 顺子',group:'赌坊',note:'顺子三倍与净收益'},
  {id:'casino-lose',name:'赌坊 · 未中',group:'赌坊',note:'下注失利的结算'},
  {id:'casino-triple',name:'赌坊 · 豹子',group:'赌坊',note:'六六六十倍返还'},
  {id:'casino-half',name:'赌坊 · 退半',group:'赌坊',note:'三奇三偶退半'},
  {id:'casino-pair',name:'赌坊 · 回本',group:'赌坊',note:'对子原银奉还'},
  {id: 'adventure-variety', name: '江湖奇遇 · 有得有失', group: '历练', note: '高人指点、拾到钱袋与钱袋遭窃'},
  {
    id: 'profile',
    name: '我的侠客',
    group: '侠客',
    note: '属性、兵刃、武学与常用指令',
  },
  {
    id: 'upgrade-swordsman',
    name: '剑客 · 十级',
    group: '侠客',
    note: '剑客成长与装备展示',
  },
  {
    id: 'upgrade-blade',
    name: '刀客 · 十级',
    group: '侠客',
    note: '刀客成长与装备展示',
  },
  {
    id: 'upgrade-boxer',
    name: '拳师 · 十级',
    group: '侠客',
    note: '拳师成长与装备展示',
  },
  {
    id: 'inventory-profile',
    name: '装备齐备',
    group: '侠客',
    note: '当前护具、饰品与所习武学',
  },
  {
    id: 'onboarding',
    name: '初入江湖',
    group: '历练',
    note: '首次加入、签到与历练结算',
  },
  {
    id: 'adventure',
    name: '江湖历练',
    group: '历练',
    note: '连续遭遇与奖励结算',
  },
  {
    id: 'adventure-1',
    name: '单次历练',
    group: '历练',
    note: '单个遭遇的卡面布局',
  },
  {
    id: 'adventure-0',
    name: '换装回执',
    group: '侠客',
    note: '没有历练遭遇时的操作结果',
  },
  {
    id: 'replay',
    name: '今日已历练',
    group: '历练',
    note: '重复进入时的回顾状态',
  },
  {
    id: 'duel-yueli',
    name: '群友切磋',
    group: '切磋',
    note: '胜负、剩余气血与完整交锋',
  },
  {
    id: 'duel',
    name: '跨职业切磋',
    group: '切磋',
    note: '剑客与刀客的胜负及完整交锋',
  },
  {
    id: 'draw-five',
    name: '江湖宝匣',
    group: '寻宝',
    note: '现有五连抽结算；尚未替换新版装备美术',
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
const source = (id: string) => `/wuxia-preview/${id}.webp`
</script>

<template>
  <main class="mx-auto max-w-7xl space-y-8 px-4 py-8 sm:px-8">
    <header class="space-y-4 border-b border-stone-700/60 pb-6">
      <p class="text-xs tracking-[0.3em] text-amber-200/60">
        月灵江湖 · 本地预览
      </p>
      <h1 class="font-serif text-3xl text-stone-100">江湖画卷</h1>
      <p class="max-w-3xl text-sm leading-7 text-stone-400">
        在这里一起查看游戏界面与装备图鉴。界面样张由当前卡片渲染器生成，使用示例角色，不连接群聊、不消耗银两。
      </p>
      <nav class="flex gap-3" aria-label="预览内容">
        <button
          v-for="item in [
            { id: 'screens', name: '游戏界面' },
            { id: 'equipment', name: '装备图鉴' },
          ]"
          :key="item.id"
          :aria-pressed="section === item.id"
          class="rounded-lg border px-5 py-2 text-sm"
          :class="
            section === item.id
              ? 'border-amber-200/50 bg-amber-200/10 text-amber-100'
              : 'border-stone-700 text-stone-400'
          "
          @click="section = item.id"
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
        点击卡片查看大图。这里展示现有游戏界面；新百器录美术与文案尚未接入这些玩法卡片。
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
    <WuxiaWeaponGallery v-else />
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
