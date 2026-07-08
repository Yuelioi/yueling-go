<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterLink, RouterView, useRoute, useRouter } from 'vue-router'
import { api } from './api'

const route = useRoute()
const router = useRouter()
const inLogin = computed(() => route.path === '/login')
const loggingOut = ref(false)
const shellError = ref('')

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
    <div v-else class="min-h-screen bg-neutral-950 text-neutral-100">
      <header class="border-b border-neutral-800 bg-neutral-900/90">
        <div class="mx-auto flex max-w-7xl items-center gap-3 px-4 py-3">
          <div class="flex min-w-0 items-center gap-2 font-semibold">
            <UIcon name="i-tabler-moon-stars" class="size-5 text-primary" />
            <span>月灵 WebUI</span>
          </div>
          <nav class="ml-4 flex gap-1 text-sm">
            <RouterLink
              class="rounded-md px-3 py-1.5 text-neutral-300 hover:bg-neutral-800 hover:text-white"
              active-class="bg-neutral-800 text-white"
              to="/"
            >
              群插件
            </RouterLink>
            <RouterLink
              class="rounded-md px-3 py-1.5 text-neutral-300 hover:bg-neutral-800 hover:text-white"
              active-class="bg-neutral-800 text-white"
              to="/affinity"
            >
              AI 好感度
            </RouterLink>
          </nav>
          <UButton
            class="ml-auto"
            color="neutral"
            variant="ghost"
            icon="i-tabler-logout"
            :loading="loggingOut"
            @click="logout"
          >
            退出
          </UButton>
        </div>
      </header>
      <main class="mx-auto max-w-7xl px-4 py-5">
        <UAlert
          v-if="shellError"
          class="mb-4"
          color="error"
          icon="i-tabler-alert-circle"
          :description="shellError"
        />
        <RouterView />
      </main>
    </div>
  </UApp>
</template>
