<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api'

const router = useRouter()
const password = ref('')
const loading = ref(false)
const error = ref('')

async function submit() {
  error.value = ''
  loading.value = true
  try {
    await api.login(password.value)
    await router.push('/')
  } catch (err) {
    error.value = err instanceof Error ? err.message : '登录失败'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <main class="flex min-h-screen items-center justify-center bg-neutral-950 px-4">
    <form
      class="w-full max-w-sm space-y-4 rounded-lg border border-neutral-800 bg-neutral-900 p-6"
      @submit.prevent="submit"
    >
      <div>
        <h1 class="text-xl font-semibold text-white">月灵 WebUI</h1>
        <p class="mt-1 text-sm text-neutral-400">输入管理密码继续</p>
      </div>
      <UInput
        v-model="password"
        type="password"
        autofocus
        placeholder="Password"
        icon="i-tabler-lock"
      />
      <UAlert
        v-if="error"
        color="error"
        variant="subtle"
        icon="i-tabler-alert-circle"
        :description="error"
      />
      <UButton type="submit" block :loading="loading" icon="i-tabler-login-2">
        登录
      </UButton>
    </form>
  </main>
</template>
