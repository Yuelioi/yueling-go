import './assets/main.css'

import { createApp } from 'vue'
import ui from '@nuxt/ui/vue-plugin'
import App from './App.vue'
import { router } from './router'
import { pinia, resetPrivateData } from './stores'
import { useSessionStore } from './stores/session'
import { onUnauthorized } from './api/http'

document.documentElement.classList.add('dark')

const app = createApp(App)
app.use(pinia)
onUnauthorized(() => {
  useSessionStore(pinia).expire()
  resetPrivateData()
})
app.use(router)
app.use(ui)
app.mount('#app')
