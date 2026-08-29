<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterLink, RouterView, useRoute, useRouter } from 'vue-router'
import { api } from './api'

const route = useRoute()
const router = useRouter()
const inLogin = computed(() => route.path === '/login')
const loggingOut = ref(false)
const shellError = ref('')
const navItems = [
  { to: '/', label: '运行总览', desc: '查看月灵当前状态', icon: 'i-tabler-layout-dashboard' },
  { to: '/plugins', label: '插件策略', desc: '为每个群配置能力', icon: 'i-tabler-components' },
  { to: '/command-usage', label: '调用统计', desc: '查看每群命令使用情况', icon: 'i-tabler-chart-bar' },
  { to: '/chat-insights', label: '聊天洞察', desc: '词云与群友常说的话', icon: 'i-tabler-message-circle-star' },
  { to: '/group-actions', label: '消息中心', desc: '向指定群发送消息', icon: 'i-tabler-send' },
  { to: '/ai-style', label: 'AI 对话风格', desc: '设置默认与群级覆盖', icon: 'i-tabler-sparkles' },
  { to: '/digests', label: '群聊日报', desc: '管理每日 AI 摘要', icon: 'i-tabler-notes' },
  { to: '/feeds', label: '订阅中心', desc: '聚合 RSS 与 Atom 更新', icon: 'i-tabler-rss' },
  { to: '/knowledge', label: '群知识库', desc: '管理群资料与问答来源', icon: 'i-tabler-books' },
  { to: '/affinity', label: '关系引擎', desc: '管理 AI 好感度', icon: 'i-tabler-heart-handshake' },
  { to: '/memories', label: '长期记忆', desc: '审阅 AI 用户偏好', icon: 'i-tabler-brain' },
]
const navActiveClass = 'border-[rgba(125,108,200,0.24)] bg-[linear-gradient(90deg,rgba(125,108,200,0.15),rgba(125,108,200,0.05))] text-[var(--ink)] shadow-[inset_2px_0_var(--violet)] [&>span:first-child]:bg-[var(--violet-soft)] [&>span:first-child]:text-[var(--violet-bright)] [&>svg:last-child]:text-[var(--dim)]'
const currentNav = computed(() =>
  navItems.find((item) => item.to === route.path) ?? navItems[0],
)

async function logout() {
  shellError.value = ''
  loggingOut.value = true
  try {
    await api.logout()
    await router.push('/login')
  } catch (err) {
    shellError.value = err instanceof Error ? err.message : '退出失败'
  } finally {
    loggingOut.value = false
  }
}
</script>

<template>
  <UApp>
    <RouterView v-if="inLogin" />
    <div v-else class="admin-shell relative min-h-screen overflow-clip bg-[radial-gradient(circle_at_46%_-12%,rgba(122,92,255,0.12),transparent_31rem),linear-gradient(180deg,#0c0b18_0%,var(--page)_42%)] before:pointer-events-none before:fixed before:inset-0 before:bg-[linear-gradient(rgba(255,255,255,0.018)_1px,transparent_1px),linear-gradient(90deg,rgba(255,255,255,0.018)_1px,transparent_1px)] before:opacity-24 before:content-[''] before:[background-size:44px_44px] before:[mask-image:linear-gradient(to_bottom,black,transparent_70%)]">
      <div class="pointer-events-none fixed right-[-13rem] top-[16%] size-[28rem] rounded-full bg-[radial-gradient(circle,rgba(98,217,236,0.08),transparent_68%)] blur-[1px]" />
      <div class="pointer-events-none fixed bottom-[-18rem] left-[20%] size-[38rem] rounded-full bg-[radial-gradient(circle,rgba(125,108,200,0.08),transparent_68%)] blur-[1px]" />
      <div class="relative z-[1] grid min-h-screen grid-cols-[268px_minmax(0,1fr)] max-[1060px]:grid-cols-[224px_minmax(0,1fr)] max-[860px]:block">
        <aside class="sticky top-0 flex h-screen flex-col border-r border-[var(--border)] bg-[rgba(10,9,19,0.78)] px-[18px] pb-[18px] pt-6 backdrop-blur-3xl max-[860px]:relative max-[860px]:h-auto max-[860px]:border-b max-[860px]:border-r-0 max-[860px]:p-4">
          <div class="flex items-center gap-3">
            <div class="relative grid size-[42px] shrink-0 place-items-center rounded-[14px] border border-[rgba(185,175,230,0.3)] bg-[linear-gradient(145deg,rgba(167,146,255,0.22),rgba(83,64,163,0.12))] text-[var(--violet-bright)] shadow-[inset_0_1px_rgba(255,255,255,0.08),0_12px_28px_rgba(81,57,179,0.18)] after:absolute after:inset-1.5 after:rounded-[9px] after:border after:border-white/[0.05] after:content-['']">
              <UIcon name="i-tabler-moon-stars" class="size-6" />
            </div>
            <div class="min-w-0">
              <div class="text-[0.98rem] font-[720] tracking-[0.01em] text-[var(--ink)]">月灵控制台</div>
              <div class="mt-0.5 text-[0.65rem] font-bold tracking-[0.16em] text-[var(--dim)] max-[520px]:hidden">YUELING · BOT OS</div>
            </div>
          </div>

          <div class="mx-2.5 mb-2.5 mt-[38px] text-[0.65rem] font-[750] uppercase tracking-[0.14em] text-[var(--dim)] max-[860px]:hidden">工作空间</div>
          <div class="min-h-0 min-w-0 flex-1 overflow-y-auto pr-[3px] max-[860px]:relative max-[860px]:mx-[-4px] max-[860px]:mt-4 max-[860px]:overflow-y-visible max-[860px]:pr-0 max-[860px]:before:pointer-events-none max-[860px]:before:absolute max-[860px]:before:bottom-2.5 max-[860px]:before:left-0 max-[860px]:before:top-0 max-[860px]:before:z-[2] max-[860px]:before:w-[18px] max-[860px]:before:bg-[linear-gradient(90deg,rgba(10,9,19,0.96),transparent)] max-[860px]:before:content-[''] max-[860px]:after:pointer-events-none max-[860px]:after:absolute max-[860px]:after:bottom-2.5 max-[860px]:after:right-0 max-[860px]:after:top-0 max-[860px]:after:z-[2] max-[860px]:after:w-[18px] max-[860px]:after:bg-[linear-gradient(270deg,rgba(10,9,19,0.96),transparent)] max-[860px]:after:content-['']">
            <nav class="grid gap-1.5 max-[860px]:flex max-[860px]:snap-x max-[860px]:snap-proximity max-[860px]:overflow-x-auto max-[860px]:overscroll-x-contain max-[860px]:px-4 max-[860px]:pb-[9px] max-[860px]:[scroll-padding-inline:10px] max-[860px]:[scrollbar-color:rgba(125,108,200,0.62)_var(--scrollbar-track)] max-[860px]:[scrollbar-width:thin] max-[860px]:[&::-webkit-scrollbar]:h-[7px] max-[860px]:[&::-webkit-scrollbar-thumb]:border max-[860px]:[&::-webkit-scrollbar-thumb]:border-transparent max-[860px]:[&::-webkit-scrollbar-thumb]:bg-[linear-gradient(90deg,rgba(118,102,199,0.72),rgba(103,216,236,0.58))] max-[860px]:[&::-webkit-scrollbar-thumb]:[background-clip:padding-box] max-[860px]:[&::-webkit-scrollbar-thumb]:shadow-[0_0_12px_rgba(125,108,200,0.2)] max-[860px]:[&::-webkit-scrollbar-thumb:hover]:bg-[linear-gradient(90deg,rgba(166,154,224,0.92),rgba(103,216,236,0.78))] max-[860px]:[&::-webkit-scrollbar-thumb:hover]:[background-clip:padding-box] max-[860px]:[&::-webkit-scrollbar-track]:rounded-full max-[860px]:[&::-webkit-scrollbar-track]:border max-[860px]:[&::-webkit-scrollbar-track]:border-[rgba(184,172,224,0.055)] max-[860px]:[&::-webkit-scrollbar-track]:bg-[var(--scrollbar-track)]">
              <RouterLink
                v-for="item in navItems"
                :key="item.to"
                class="group grid grid-cols-[34px_minmax(0,1fr)_18px] items-center gap-2.5 rounded-xl border border-transparent px-2.5 py-[9px] text-[var(--muted)] transition duration-160 hover:border-[var(--border)] hover:bg-white/[0.025] hover:text-[var(--ink-soft)] max-[860px]:min-w-max max-[860px]:snap-start max-[860px]:grid-cols-[30px_auto] max-[860px]:px-2.5 max-[860px]:py-[7px]"
                :active-class="navActiveClass"
                :to="item.to"
              >
                <span class="grid size-8 place-items-center rounded-[9px] bg-white/[0.035] text-[var(--muted)]"><UIcon :name="item.icon" class="size-4" /></span>
                <span class="min-w-0">
                  <span class="block text-[0.84rem] font-[650]">{{ item.label }}</span>
                  <span class="mt-px block truncate text-[0.68rem] text-[var(--dim)] max-[860px]:hidden">{{ item.desc }}</span>
                </span>
                <UIcon name="i-tabler-chevron-right" class="size-4 text-transparent transition duration-160 group-hover:text-[var(--dim)] max-[860px]:hidden" />
              </RouterLink>
            </nav>
          </div>

          <div class="h-3.5 shrink-0 max-[860px]:hidden" />
          <div class="rounded-[13px] border border-[var(--border)] bg-white/[0.025] p-[13px] max-[860px]:hidden">
            <div class="flex items-center gap-2 text-[0.76rem] font-[650] text-[var(--ink-soft)]">
              <span class="size-[7px] rounded-full bg-[var(--success)] shadow-[0_0_0_4px_rgba(100,215,165,0.09),0_0_14px_rgba(100,215,165,0.45)]" />
              <span>控制台在线</span>
            </div>
            <p class="m-0 mt-[9px] text-[0.68rem] leading-[1.55] text-[var(--dim)]">所有修改会即时同步到机器人运行状态。</p>
          </div>

          <UButton
            class="mt-2.5 justify-center !text-[var(--muted)] max-[860px]:hidden"
            color="neutral"
            variant="ghost"
            icon="i-tabler-logout"
            :loading="loggingOut"
            @click="logout"
          >
            退出
          </UButton>
        </aside>

        <div class="min-w-0">
          <header class="sticky top-0 z-20 flex min-h-16 items-center justify-between border-b border-[var(--border)] bg-[rgba(9,9,18,0.72)] px-8 backdrop-blur-[22px] max-[860px]:relative max-[860px]:min-h-[54px] max-[860px]:px-[18px]">
            <div class="flex items-center gap-2 text-[0.76rem] text-[var(--dim)]">
              <span class="max-[520px]:hidden">月灵</span>
              <UIcon name="i-tabler-chevron-right" class="size-3.5 max-[520px]:hidden" />
              <strong class="font-[650] text-[var(--ink-soft)]">{{ currentNav.label }}</strong>
            </div>
            <div class="flex items-center gap-[9px] text-[0.76rem] text-[var(--muted)] max-[860px]:hidden">
              <span class="size-[7px] rounded-full bg-[var(--success)] shadow-[0_0_0_4px_rgba(100,215,165,0.09),0_0_14px_rgba(100,215,165,0.45)]" />
              <span>控制台在线</span>
            </div>
            <UButton
              class="!hidden max-[860px]:!inline-flex"
              color="neutral"
              variant="ghost"
              icon="i-tabler-logout"
              aria-label="退出控制台"
              :loading="loggingOut"
              @click="logout"
            />
          </header>

          <main class="w-[min(1440px,100%)] px-9 pb-14 pt-[34px] max-[1060px]:px-6 max-[860px]:px-4 max-[860px]:pb-10 max-[860px]:pt-6">
            <UAlert
              v-if="shellError"
              class="error-banner mb-4"
              color="error"
              variant="subtle"
              icon="i-tabler-alert-circle"
              :description="shellError"
            />
            <RouterView />
          </main>
        </div>
      </div>
    </div>
  </UApp>
</template>
