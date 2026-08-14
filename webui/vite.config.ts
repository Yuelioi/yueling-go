import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import ui from '@nuxt/ui/vite'

const ignoredRolldownWarnings = new Set(['INVALID_ANNOTATION'])

export default defineConfig({
  plugins: [
    vue(),
    ui({
      theme: {
        defaultVariants: {
          size: 'sm',
        },
      },
      ui: {
        colors: {
          primary: 'violet',
          neutral: 'zinc',
        },
        button: {
          slots: {
            base: 'rounded-lg tracking-[-0.01em]',
          },
          compoundVariants: [
            {
              color: 'primary',
              variant: 'solid',
              class: 'bg-[#7464c7] text-[#fbfaff] hover:bg-[#8170d2] active:bg-[#6959ba] disabled:bg-[#7464c7]',
            },
            {
              color: 'primary',
              variant: ['soft', 'subtle'],
              class: 'bg-[#7464c7]/15 text-[#eeeafd] hover:bg-[#7464c7]/23 active:bg-[#7464c7]/26 ring-[#9b8ed8]/20',
            },
            {
              color: 'neutral',
              variant: 'ghost',
              class: '!text-[#aaa5b8] hover:!bg-white/6 hover:!text-[#ddd8e5]',
            },
            {
              color: 'neutral',
              variant: ['soft', 'subtle'],
              class: '!bg-white/5 !text-[#c4bece] hover:!bg-white/8 !ring-white/8',
            },
            {
              color: 'success',
              variant: ['soft', 'subtle'],
              class: '!bg-[#3f8a69]/14 !text-[#86d4b2] hover:!bg-[#3f8a69]/22 active:!bg-[#3f8a69]/26 !ring-[#63b28d]/15',
            },
            {
              color: 'warning',
              variant: ['soft', 'subtle'],
              class: '!bg-[#94713c]/15 !text-[#dfb978] hover:!bg-[#94713c]/24 active:!bg-[#94713c]/28 !ring-[#c99a53]/15',
            },
            {
              color: 'error',
              variant: 'solid',
              class: '!bg-[#985267] !text-[#fff8fa] hover:!bg-[#aa5e74] active:!bg-[#87475a]',
            },
            {
              color: 'error',
              variant: ['soft', 'subtle'],
              class: '!bg-[#8d4960]/18 !text-[#dc91a6] hover:!bg-[#8d4960]/28 active:!bg-[#8d4960]/32 !ring-[#b9667e]/16',
            },
            {
              color: 'error',
              variant: 'ghost',
              class: '!text-[#c98296] hover:!bg-[#8d4960]/16 hover:!text-[#e2a0b2]',
            },
          ],
        },
        badge: {
          slots: {
            base: 'rounded-md border font-semibold shadow-none',
          },
          compoundVariants: [
            {
              color: 'primary',
              variant: ['soft', 'subtle'],
              class: '!border-[#9b8ed8]/20 !bg-[#7464c7]/12 !text-[#d8d1f2] !ring-0',
            },
            {
              color: 'neutral',
              variant: ['soft', 'subtle'],
              class: '!border-white/8 !bg-white/4 !text-[#aaa5b8] !ring-0',
            },
            {
              color: 'success',
              variant: ['soft', 'subtle'],
              class: '!border-[#63b28d]/18 !bg-[#3f8a69]/10 !text-[#86d4b2] !ring-0',
            },
            {
              color: 'warning',
              variant: ['soft', 'subtle'],
              class: '!border-[#c99a53]/18 !bg-[#94713c]/10 !text-[#dfb978] !ring-0',
            },
            {
              color: 'error',
              variant: ['soft', 'subtle'],
              class: '!border-[#b9667e]/18 !bg-[#8d4960]/10 !text-[#dc91a6] !ring-0',
            },
          ],
        },
        dropdownMenu: {
          slots: {
            content: 'rounded-xl border border-[#b8ace0]/15 bg-[#151426] p-1 shadow-[0_20px_55px_rgba(0,0,0,0.42)] ring-0',
            item: 'min-h-8 rounded-lg text-[#c4bece] data-highlighted:bg-white/6 data-highlighted:text-[#f4f1fa]',
            itemDescription: 'text-[0.62rem] text-[#777185]',
            separator: 'bg-white/6',
          },
        },
      },
    }),
  ],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  build: {
    rolldownOptions: {
      onLog(level, log, handler) {
        if (
          level === 'warn' &&
          ignoredRolldownWarnings.has(log.code ?? '') &&
          log.id?.includes('@vueuse/core')
        ) {
          return
        }
        handler(level, log)
      },
    },
  },
  server: {
    proxy: {
      '/api/webui': 'http://127.0.0.1:9080',
    },
  },
})
