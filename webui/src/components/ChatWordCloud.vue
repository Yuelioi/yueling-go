<script setup lang="ts">
import cloud from 'd3-cloud'
import type { CSSProperties } from 'vue'
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type { ChatInsightWord } from '../api'

interface PlacedCloudWord extends ChatInsightWord {
  size: number
  weight: number
  tone: number
  x?: number
  y?: number
  rotate?: number
}

const props = defineProps<{
  words: ChatInsightWord[]
  periodLabel: string
}>()

const host = ref<HTMLElement | null>(null)
const placedWords = ref<PlacedCloudWord[]>([])
let layoutVersion = 0
let activeLayout: { stop: () => unknown } | null = null
let resizeObserver: ResizeObserver | null = null

const cloudToneClasses = [
  'text-[var(--violet-bright)]',
  'text-[#7fc7d4]',
  'text-[#c897a6]',
  'text-[#d2ad70]',
  'text-[#80c3a4]',
] as const

function cloudWordStyle(word: PlacedCloudWord): CSSProperties {
  return {
    left: `calc(50% + ${word.x || 0}px)`,
    top: `calc(50% + ${word.y || 0}px)`,
    fontSize: `${word.size}px`,
    fontWeight: String(word.weight),
    transform: `translate(-50%, -50%) rotate(${word.rotate || 0}deg)`,
  }
}

function seededRandom(seed: number) {
  let state = seed >>> 0 || 1
  return () => {
    state = (state * 1664525 + 1013904223) >>> 0
    return state / 4294967296
  }
}

function wordCloudSeed(words: ChatInsightWord[]) {
  let hash = 2166136261
  for (const word of words) {
    for (const char of `${word.text}:${word.count}|`) {
      hash ^= char.codePointAt(0) || 0
      hash = Math.imul(hash, 16777619)
    }
  }
  return hash >>> 0
}

function layoutWordCloud() {
  activeLayout?.stop()
  const version = ++layoutVersion
  if (!host.value || !props.words.length) {
    placedWords.value = []
    return
  }

  const width = Math.max(280, Math.floor(host.value.clientWidth))
  const height = Math.max(260, Math.floor(host.value.clientHeight))
  const maxCount = Math.max(1, ...props.words.map((word) => word.count))
  const compact = width < 520
  const sizeRange = compact ? Math.min(34, width / 14) : Math.min(43, width / 13)
  const horizontalInset = compact ? 72 : 36
  const words: PlacedCloudWord[] = props.words.map((word, index) => {
    const scale = Math.sqrt(word.count / maxCount)
    return {
      ...word,
      size: Math.round(14 + scale * sizeRange),
      weight: Math.round(580 + scale * 150),
      tone: index % cloudToneClasses.length,
      rotate: 0,
    }
  })

  activeLayout = cloud<PlacedCloudWord>()
    .size([width - horizontalInset, height - 28])
    .words(words)
    .padding((word) => word.size >= 42 ? 7 : 5)
    .rotate(0)
    .font('Inter, ui-sans-serif, system-ui, sans-serif')
    .fontWeight((word) => word.weight)
    .fontSize((word) => word.size)
    .spiral('archimedean')
    .random(seededRandom(wordCloudSeed(props.words)))
    .on('end', (result) => {
      if (version === layoutVersion) placedWords.value = result
    })
    .start()
}

function scheduleLayout() {
  void nextTick(layoutWordCloud)
}

watch(() => props.words, scheduleLayout)

watch(host, (current, previous) => {
  if (previous) resizeObserver?.unobserve(previous)
  if (current) {
    resizeObserver?.observe(current)
    scheduleLayout()
  }
})

onMounted(() => {
  resizeObserver = new ResizeObserver(scheduleLayout)
  if (host.value) resizeObserver.observe(host.value)
  scheduleLayout()
})

onBeforeUnmount(() => {
  layoutVersion++
  activeLayout?.stop()
  resizeObserver?.disconnect()
})
</script>

<template>
  <section class="surface-panel overflow-hidden">
    <div class="panel-header">
      <div>
        <div class="section-title">{{ periodLabel }}词云</div>
      </div>
      <UBadge color="primary" variant="subtle">{{ words.length }} 个热词</UBadge>
    </div>
    <div
      v-if="words.length"
      ref="host"
      class="relative h-[330px] overflow-hidden bg-[radial-gradient(circle_at_25%_25%,rgba(125,108,200,0.13),transparent_15rem),radial-gradient(circle_at_80%_72%,rgba(103,216,236,0.075),transparent_14rem),rgba(8,8,17,0.22)] max-[860px]:h-[280px]"
    >
      <div aria-hidden="true" class="pointer-events-none absolute inset-0 bg-[radial-gradient(rgba(255,255,255,0.16)_0.6px,transparent_0.6px)] opacity-[0.16] [background-size:23px_23px] [mask-image:radial-gradient(circle,black,transparent_78%)]" />
      <span
        v-for="word in placedWords"
        :key="word.text"
        class="absolute z-[1] max-w-full origin-center whitespace-nowrap tracking-[-0.035em] leading-[1.05] [text-shadow:0_5px_24px_color-mix(in_srgb,currentColor_18%,transparent)] transition-[color,filter] duration-[160ms] hover:brightness-[1.18]"
        :class="cloudToneClasses[word.tone] || cloudToneClasses[0]"
        :style="cloudWordStyle(word)"
        :title="`${word.text} · ${word.count} 条消息`"
      >
        {{ word.text }}
      </span>
    </div>
    <div v-else class="empty-state overview-empty">
      <UIcon name="i-tabler-cloud-off" class="size-6" />
      <span>文字还不够生成词云</span>
    </div>
  </section>
</template>
