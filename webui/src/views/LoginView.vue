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
  <main class="relative grid min-h-screen grid-cols-[minmax(0,1.05fr)_minmax(380px,0.72fr)] items-center gap-[clamp(3rem,8vw,8rem)] overflow-hidden bg-[radial-gradient(circle_at_20%_22%,rgba(126,91,255,0.15),transparent_29rem),radial-gradient(circle_at_82%_78%,rgba(72,196,221,0.08),transparent_25rem),linear-gradient(145deg,#0d0b1a,#080811_64%)] p-[clamp(32px,7vw,96px)] before:pointer-events-none before:absolute before:inset-0 before:bg-[radial-gradient(rgba(255,255,255,0.11)_0.6px,transparent_0.6px)] before:opacity-35 before:content-[''] before:[background-size:28px_28px] before:[mask-image:radial-gradient(circle_at_35%_40%,black,transparent_68%)] max-[1060px]:gap-12 max-[1060px]:p-12 max-[860px]:block max-[860px]:px-5 max-[860px]:py-7">
    <div class="pointer-events-none fixed rounded-full blur-[1px]" />
    <div class="pointer-events-none fixed rounded-full blur-[1px]" />

    <section class="relative z-[1] max-w-[650px] max-[860px]:mx-auto max-[860px]:mb-8">
      <div class="flex items-center gap-3">
        <div class="relative grid size-[52px] shrink-0 place-items-center rounded-[17px] border border-[rgba(185,175,230,0.3)] bg-[linear-gradient(145deg,rgba(167,146,255,0.22),rgba(83,64,163,0.12))] text-[var(--violet-bright)] shadow-[inset_0_1px_rgba(255,255,255,0.08),0_12px_28px_rgba(81,57,179,0.18)] after:absolute after:inset-1.5 after:rounded-[9px] after:border after:border-white/[0.05] after:content-['']">
          <UIcon name="i-tabler-moon-stars" class="size-7" />
        </div>
        <div class="min-w-0">
          <div class="text-[1.08rem] font-[720] tracking-[0.01em] text-[var(--ink)]">月灵控制台</div>
          <div class="mt-0.5 text-[0.65rem] font-bold tracking-[0.16em] text-[var(--dim)] max-[520px]:hidden">YUELING · BOT OS</div>
        </div>
      </div>

      <div class="mt-[clamp(70px,13vh,130px)] max-[860px]:mt-14">
        <div class="text-[0.65rem] font-[780] uppercase tracking-[0.15em] text-[var(--violet-bright)]">Private command center</div>
        <h1 class="m-0 mt-3 text-[clamp(2.5rem,5vw,4.65rem)] font-[730] leading-[1.08] tracking-[-0.065em] text-[var(--ink)] max-[860px]:text-[clamp(2.3rem,12vw,3.3rem)]">让月灵的每一次回应，<br><span class="text-[var(--violet-bright)]">都在你的掌控中。</span></h1>
        <p class="m-0 mt-6 max-w-[520px] text-[0.95rem] leading-[1.8] text-[var(--muted)] max-[520px]:text-[0.84rem]">集中管理群聊能力、消息分发与 AI 关系状态。简单、即时，并且只属于你。</p>
      </div>

      <div class="mt-[52px] flex flex-wrap gap-5 max-[860px]:hidden">
        <div class="flex items-center gap-[7px] text-[0.72rem] text-[var(--muted)]"><UIcon name="i-tabler-bolt" class="size-4 text-[var(--violet-bright)]" /><span>配置实时生效</span></div>
        <div class="flex items-center gap-[7px] text-[0.72rem] text-[var(--muted)]"><UIcon name="i-tabler-shield-lock" class="size-4 text-[var(--violet-bright)]" /><span>单管理员访问</span></div>
        <div class="flex items-center gap-[7px] text-[0.72rem] text-[var(--muted)]"><UIcon name="i-tabler-layout-dashboard" class="size-4 text-[var(--violet-bright)]" /><span>统一运维视图</span></div>
      </div>
    </section>

    <form class="relative z-[1] w-[min(430px,100%)] justify-self-end overflow-hidden rounded-[22px] border border-[var(--border-strong)] bg-[rgba(21,19,38,0.8)] shadow-[0_34px_100px_rgba(0,0,0,0.5),inset_0_1px_rgba(255,255,255,0.05)] backdrop-blur-[26px] max-[860px]:mx-auto" @submit.prevent="submit">
      <div class="absolute -top-36 left-1/2 h-60 w-80 -translate-x-1/2 rounded-full bg-[rgba(143,119,255,0.18)] blur-[55px]" />
      <div class="relative p-[38px] max-[520px]:px-[22px] max-[520px]:py-7">
        <div class="mb-7 grid size-[42px] place-items-center rounded-[13px] border border-[rgba(125,108,200,0.24)] bg-[var(--violet-soft)] text-[var(--violet-bright)]"><UIcon name="i-tabler-key" class="size-5" /></div>
        <div class="text-[0.65rem] font-[780] uppercase tracking-[0.15em] text-[var(--violet-bright)]">Welcome back</div>
        <h2 class="m-0 mt-[7px] text-[1.65rem] font-[720] tracking-[-0.04em]">进入控制台</h2>
        <p class="m-0 mt-[9px] text-[0.78rem] leading-[1.6] text-[var(--muted)]">使用 <code class="text-[var(--violet-bright)]">config.toml</code> 中配置的管理密码。</p>

        <UFormField label="管理密码" class="mb-[18px] mt-7">
          <UInput
            v-model="password"
            class="w-full"
            :ui="{ root: 'w-full', base: '!bg-[var(--field)] !text-[var(--ink)] !shadow-[inset_0_0_0_1px_var(--border)] placeholder:!text-[var(--dim)] focus:!shadow-[inset_0_0_0_1px_var(--violet),0_0_0_3px_var(--violet-soft)]' }"
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
        <div class="mt-[18px] flex items-center justify-center gap-1.5 text-[0.68rem] text-[var(--dim)]">
          <UIcon name="i-tabler-lock-check" class="size-4" />
          会话将在 24 小时后自动过期
        </div>
      </div>
    </form>
  </main>
</template>
