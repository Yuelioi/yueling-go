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
  { to: '/', label: '群插件', desc: '按群控制插件', icon: 'i-tabler-puzzle' },
  { to: '/group-actions', label: '群操作', desc: '发送群消息', icon: 'i-tabler-message-2-share' },
  { to: '/affinity', label: 'AI 好感度', desc: '修正回复态度', icon: 'i-tabler-heart-cog' },
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
      <div class="admin-layout">
        <aside class="admin-sidebar">
          <div class="flex items-center gap-3">
            <div class="brand-mark">
              <UIcon name="i-tabler-moon-stars" class="size-5" />
            </div>
            <div class="min-w-0">
              <div class="font-semibold text-white">月灵 WebUI</div>
              <div class="text-xs text-zinc-500">Bot Admin Console</div>
            </div>
          </div>

          <nav class="mt-8 space-y-1 text-sm">
            <RouterLink
              v-for="item in navItems"
              :key="item.to"
              class="nav-link"
              active-class="nav-link-active"
              :to="item.to"
            >
              <UIcon :name="item.icon" class="size-4 shrink-0" />
              <span class="min-w-0">
                <span class="block font-medium">{{ item.label }}</span>
                <span class="block truncate text-xs text-zinc-500">{{ item.desc }}</span>
              </span>
            </RouterLink>
          </nav>

          <div class="surface-inset mt-auto p-3 text-xs text-zinc-400">
            <div class="mb-2 flex items-center justify-between">
              <span>WebUI</span>
              <UBadge color="success" variant="subtle" size="xs">online</UBadge>
            </div>
            <div class="truncate">使用 config.toml 管理访问密码</div>
          </div>

          <UButton
            class="mt-3 justify-center"
            color="primary"
            variant="soft"
            icon="i-tabler-logout"
            :loading="loggingOut"
            @click="logout"
          >
            退出
          </UButton>
        </aside>

        <div class="admin-main">
          <header class="admin-topbar">
            <div class="min-w-0">
              <div class="eyebrow">Yueling Admin</div>
              <h1 class="truncate text-xl font-semibold text-white">{{ currentNav.label }}</h1>
            </div>
            <UBadge color="primary" variant="subtle">
              {{ currentNav.desc }}
            </UBadge>
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
