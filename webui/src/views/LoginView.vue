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
  if (typeof redirect === 'string' && redirect.startsWith('/') && !redirect.startsWith('//')) {
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
  <main class="login-shell">
    <form
      class="login-card space-y-5 p-6"
      @submit.prevent="submit"
    >
      <div class="flex items-start gap-3">
        <div class="brand-mark">
          <UIcon name="i-tabler-moon-stars" class="size-5" />
        </div>
        <div>
          <div class="eyebrow">Admin Console</div>
          <h1 class="mt-1 text-xl font-semibold text-white">月灵 WebUI</h1>
          <p class="mt-1 text-sm text-zinc-400">输入 config.toml 中的管理密码</p>
        </div>
      </div>
      <UInput
        v-model="password"
        class="w-full"
        :ui="{ root: 'w-full' }"
        type="password"
        autofocus
        size="xl"
        placeholder="管理密码"
        icon="i-tabler-lock"
      />
      <UAlert
        v-if="error"
        class="error-banner"
        color="error"
        variant="subtle"
        icon="i-tabler-alert-circle"
        :description="error"
      />
      <UButton type="submit" block size="xl" :loading="loading" icon="i-tabler-login-2">
        登录
      </UButton>
    </form>
  </main>
</template>
