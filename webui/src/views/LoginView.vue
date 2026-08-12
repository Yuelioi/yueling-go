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
    <div class="login-ambient login-ambient-one" />
    <div class="login-ambient login-ambient-two" />

    <section class="login-story">
      <div class="brand-lockup">
        <div class="brand-mark brand-mark-large">
          <UIcon name="i-tabler-moon-stars" class="size-7" />
        </div>
        <div class="min-w-0">
          <div class="brand-name brand-name-large">月灵控制台</div>
          <div class="brand-caption">YUELING · BOT OS</div>
        </div>
      </div>

      <div class="login-copy">
        <div class="eyebrow">Private command center</div>
        <h1>让月灵的每一次回应，<br><span>都在你的掌控中。</span></h1>
        <p>集中管理群聊能力、消息分发与 AI 关系状态。简单、即时，并且只属于你。</p>
      </div>

      <div class="login-features">
        <div><UIcon name="i-tabler-bolt" class="size-4" /><span>配置实时生效</span></div>
        <div><UIcon name="i-tabler-shield-lock" class="size-4" /><span>单管理员访问</span></div>
        <div><UIcon name="i-tabler-layout-dashboard" class="size-4" /><span>统一运维视图</span></div>
      </div>
    </section>

    <form class="login-card" @submit.prevent="submit">
      <div class="login-card-glow" />
      <div class="login-card-content">
        <div class="login-card-icon"><UIcon name="i-tabler-key" class="size-5" /></div>
        <div class="eyebrow">Welcome back</div>
        <h2>进入控制台</h2>
        <p>使用 <code>config.toml</code> 中配置的管理密码。</p>

        <UFormField label="管理密码" class="login-field">
          <UInput
            v-model="password"
            class="w-full"
            :ui="{ root: 'w-full' }"
            type="password"
            autofocus
            size="xl"
            placeholder="输入管理密码"
            icon="i-tabler-lock"
          />
        </UFormField>

        <UAlert
          v-if="error"
          class="error-banner"
          color="error"
          variant="subtle"
          icon="i-tabler-alert-circle"
          :description="error"
        />
        <UButton type="submit" block size="xl" :loading="loading" trailing-icon="i-tabler-arrow-right">
          安全登录
        </UButton>
        <div class="login-hint">
          <UIcon name="i-tabler-lock-check" class="size-4" />
          会话将在 24 小时后自动过期
        </div>
      </div>
    </form>
  </main>
</template>
