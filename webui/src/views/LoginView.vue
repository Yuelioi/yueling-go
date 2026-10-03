<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api'

const route = useRoute()
const router = useRouter()
const password = ref('')
const loading = ref(false)
const error = ref('')

function redirectTarget() {
  const redirect = route.query.redirect
  if (
    typeof redirect === 'string' &&
    redirect.startsWith('/') &&
    !redirect.startsWith('//')
  ) {
    return redirect
  }
  return '/'
}

async function submit() {
  error.value = ''
  loading.value = true
  try {
    await api.login(password.value)
    await router.push(redirectTarget())
  } catch (err) {
    error.value = err instanceof Error ? err.message : '登录失败'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <main class="grid min-h-dvh place-items-center p-4 sm:p-8">
    <div class="w-full max-w-sm space-y-6">
      <div class="text-center">
        <UIcon name="i-tabler-moon-stars" class="mb-3 size-9 text-primary" />
        <h1 class="text-2xl font-semibold text-highlighted">月灵控制台</h1>
        <p class="mt-2 text-sm text-muted">登录后管理机器人、AI 和自动任务。</p>
      </div>
      <UCard as="form" :ui="{ body: 'space-y-5' }" @submit.prevent="submit">
        <UFormField label="管理密码"
          ><UInput
            v-model="password"
            class="w-full"
            type="password"
            autofocus
            autocomplete="current-password"
            size="lg"
            placeholder="输入管理密码"
            icon="i-tabler-lock"
        /></UFormField>
        <UAlert
          v-if="error"
          color="error"
          variant="soft"
          icon="i-tabler-alert-circle"
          :description="error"
        />
        <UButton
          type="submit"
          block
          size="lg"
          :loading="loading"
          :disabled="!password"
          trailing-icon="i-tabler-arrow-right"
          >登录控制台</UButton
        >
      </UCard>
      <p class="flex items-center justify-center gap-2 text-xs text-dimmed">
        <UIcon name="i-tabler-shield-lock" class="size-4" />仅供管理员访问 ·
        会话有效期 24 小时
      </p>
    </div>
  </main>
</template>
