import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import ui from '@nuxt/ui/vite'
import { uiTheme } from './src/theme.ts'

const ignoredRolldownWarnings = new Set(['INVALID_ANNOTATION'])

export default defineConfig({
  plugins: [
    vue(),
    ui({
      colorMode: false,
      ui: uiTheme,
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
