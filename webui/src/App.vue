<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { RouterView, useRoute, useRouter } from 'vue-router'
import { api } from './api'

const route = useRoute()
const router = useRouter()
const inLogin = computed(() => route.path === '/login')
const loggingOut = ref(false)
const shellError = ref('')
const navigationOpen = ref(false)
watch(
  () => route.path,
  () => {
    navigationOpen.value = false
  },
)
const navItems = [
  { to: '/', label: '运行总览', desc: '查看月灵当前状态', icon: 'i-tabler-layout-dashboard' },
  { to: '/join-review', label: '入群审核', desc: '全局名单与单群规则', icon: 'i-tabler-user-check' },
  { to: '/groups', label: '群管理总览', desc: '全部群配置与异常', icon: 'i-tabler-users-group' },
  { to: '/plugins', label: '插件管理', desc: '全局开关与群级策略', icon: 'i-tabler-components' },
  { to: '/command-usage', label: '调用统计', desc: '查看每群命令使用情况', icon: 'i-tabler-chart-bar' },
  { to: '/chat-insights', label: '聊天洞察', desc: '词云与群友常说的话', icon: 'i-tabler-message-circle-star' },
  { to: '/group-actions', label: '消息中心', desc: '向指定群发送消息', icon: 'i-tabler-send' },
  { to: '/ai/models', label: 'AI 模型', desc: '账号、模型与能力测试', icon: 'i-tabler-cpu' },
  { to: '/ai', label: 'AI 配置', desc: '功能分配与对话设置', icon: 'i-tabler-sparkles' },
  { to: '/digests', label: '群聊日报', desc: '管理每日 AI 摘要', icon: 'i-tabler-notes' },
  { to: '/feeds', label: '订阅中心', desc: '聚合 RSS 与 Atom 更新', icon: 'i-tabler-rss' },
  { to: '/knowledge', label: '群知识库', desc: '管理群资料与问答来源', icon: 'i-tabler-books' },
  { to: '/affinity', label: '关系引擎', desc: '管理 AI 好感度', icon: 'i-tabler-heart-handshake' },
  { to: '/memories', label: '长期记忆', desc: '审阅 AI 用户偏好', icon: 'i-tabler-brain' },
  { to: '/bilibili', label: 'B站管理', desc: '账号配置与私信桥接', icon: 'i-tabler-brand-bilibili' },
  { to: '/images', label: '图片与打包', desc: '浏览素材、删除图片与配置', icon: 'i-tabler-photo' },
  { to: '/tools', label: '工具与服务', desc: '天气、搜索与代理配置', icon: 'i-tabler-tool' },
  { to: '/connections', label: '连接配置', desc: 'QQ 与消息接口', icon: 'i-tabler-plug' },
  { to: '/system', label: '系统配置', desc: '机器人与后台设置', icon: 'i-tabler-settings' },
]
const menuItems = computed(() => {
  const items = navItems.map((item) => ({
    ...item,
    active: route.path === item.to,
    exact: true,
  }))
  const systemPaths = ['/connections', '/system']
  return [
    items.filter((item) => !systemPaths.includes(item.to)),
    [
      { label: '系统', type: 'label' as const },
      ...items.filter((item) => systemPaths.includes(item.to)),
    ],
  ]
})
const currentNav = computed(
  () => navItems.find((item) => item.to === route.path) ?? navItems[0],
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
    <div v-else class="min-h-dvh lg:grid lg:grid-cols-[15rem_minmax(0,1fr)]">
      <aside
        class="sticky top-0 hidden h-dvh flex-col border-r border-default bg-muted/40 p-4 lg:flex"
      >
        <div class="mb-6 flex items-center gap-3 px-2 py-2">
          <UIcon name="i-tabler-moon-stars" class="size-7 text-primary" />
          <div>
            <p class="font-semibold text-highlighted">月灵控制台</p>
            <p class="mt-0.5 text-xs text-muted">机器人管理后台</p>
          </div>
        </div>
        <UNavigationMenu
          :items="menuItems"
          orientation="vertical"
          class="min-h-0 flex-1 overflow-y-auto"
        />
        <div class="mt-4 border-t border-default pt-4">
          <UButton
            color="neutral"
            variant="ghost"
            block
            icon="i-tabler-logout"
            :loading="loggingOut"
            @click="logout"
            >退出登录</UButton
          >
        </div>
      </aside>
      <div class="min-w-0">
        <header
          class="sticky top-0 z-30 flex h-16 items-center justify-between gap-3 border-b border-default bg-default px-4 sm:px-6 lg:px-8"
        >
          <div class="flex min-w-0 items-center gap-3">
            <USlideover
              v-model:open="navigationOpen"
              side="left"
              title="月灵控制台"
              description="选择管理页面"
              :ui="{ content: 'max-w-xs' }"
            >
              <UButton
                class="lg:hidden"
                color="neutral"
                variant="ghost"
                icon="i-tabler-menu-2"
                aria-label="打开导航"
              />
              <template #body
                ><UNavigationMenu :items="menuItems" orientation="vertical"
              /></template>
            </USlideover>
            <UBreadcrumb
              :items="[{ label: '月灵', to: '/' }, { label: currentNav.label }]"
            />
          </div>
          <UButton
            class="lg:hidden"
            color="neutral"
            variant="ghost"
            icon="i-tabler-logout"
            aria-label="退出登录"
            :loading="loggingOut"
            @click="logout"
          />
          <UBadge
            class="hidden lg:inline-flex"
            color="neutral"
            variant="soft"
            icon="i-tabler-shield-lock"
            >管理后台</UBadge
          >
        </header>
        <main class="mx-auto w-full max-w-7xl min-w-0 p-4 sm:p-6 lg:p-8">
          <UAlert
            v-if="shellError"
            class="mb-5"
            color="error"
            variant="soft"
            icon="i-tabler-alert-circle"
            :description="shellError"
          />
          <RouterView />
        </main>
      </div>
    </div>
  </UApp>
</template>
