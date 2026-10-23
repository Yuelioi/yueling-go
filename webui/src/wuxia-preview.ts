import './assets/main.css'
import { createApp } from 'vue'
import ui from '@nuxt/ui/vue-plugin'
import WuxiaPreviewHub from './components/WuxiaPreviewHub.vue'
document.documentElement.classList.add('dark')
createApp(WuxiaPreviewHub).use(ui).mount('#app')
