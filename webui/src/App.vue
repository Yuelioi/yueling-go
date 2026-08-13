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
  { to: '/group-actions', label: '消息中心', desc: '向指定群发送消息', icon: 'i-tabler-send' },
  { to: '/digests', label: '群聊日报', desc: '管理每日 AI 摘要', icon: 'i-tabler-notes' },
  { to: '/feeds', label: '订阅中心', desc: '聚合 RSS 与 Atom 更新', icon: 'i-tabler-rss' },
  { to: '/knowledge', label: '群知识库', desc: '管理群资料与问答来源', icon: 'i-tabler-books' },
  { to: '/affinity', label: '关系引擎', desc: '管理 AI 好感度', icon: 'i-tabler-heart-handshake' },
  { to: '/memories', label: '长期记忆', desc: '审阅 AI 用户偏好', icon: 'i-tabler-brain' },
]
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
    <div v-else class="admin-shell">
      <div class="ambient ambient-one" />
      <div class="ambient ambient-two" />
      <div class="admin-layout">
        <aside class="admin-sidebar">
          <div class="brand-lockup">
            <div class="brand-mark">
              <UIcon name="i-tabler-moon-stars" class="size-6" />
            </div>
            <div class="min-w-0">
              <div class="brand-name">月灵控制台</div>
              <div class="brand-caption">YUELING · BOT OS</div>
            </div>
          </div>

          <div class="sidebar-label">工作空间</div>
          <div class="admin-nav-viewport">
            <nav class="admin-nav">
              <RouterLink
                v-for="item in navItems"
                :key="item.to"
                class="nav-link"
                active-class="nav-link-active"
                :to="item.to"
              >
                <span class="nav-icon"><UIcon :name="item.icon" class="size-4" /></span>
                <span class="min-w-0">
                  <span class="nav-title">{{ item.label }}</span>
                  <span class="nav-caption">{{ item.desc }}</span>
                </span>
                <UIcon name="i-tabler-chevron-right" class="nav-arrow size-4" />
              </RouterLink>
            </nav>
          </div>

          <div class="sidebar-spacer" />
          <div class="status-card">
            <div class="status-card-top">
              <span class="status-dot" />
              <span>控制台在线</span>
            </div>
            <p>所有修改会即时同步到机器人运行状态。</p>
          </div>

          <UButton
            class="logout-button"
            color="neutral"
            variant="ghost"
            icon="i-tabler-logout"
            :loading="loggingOut"
            @click="logout"
          >
            退出
          </UButton>
        </aside>

        <div class="admin-main">
          <header class="admin-topbar">
            <div class="topbar-breadcrumb">
              <span>月灵</span>
              <UIcon name="i-tabler-chevron-right" class="size-3.5" />
              <strong>{{ currentNav.label }}</strong>
            </div>
            <div class="topbar-state">
              <span class="topbar-state-dot" />
              <span>控制台在线</span>
            </div>
            <UButton
              class="mobile-logout"
              color="neutral"
              variant="ghost"
              icon="i-tabler-logout"
              aria-label="退出控制台"
              :loading="loggingOut"
              @click="logout"
            />
          </header>

          <main class="admin-content">
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
