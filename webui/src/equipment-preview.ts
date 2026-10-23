import './assets/main.css'
import { createApp, h } from 'vue'
import ui from '@nuxt/ui/vue-plugin'
import WuxiaWeaponGallery from './components/WuxiaWeaponGallery.vue'
document.documentElement.classList.add('dark')
createApp({
  render: () =>
    h('main', { class: 'mx-auto max-w-6xl px-5 py-10' }, [
      h(
        'a',
        {
          href: '/wuxia-preview.html',
          class: 'mb-6 inline-block text-sm text-amber-200',
        },
        '← 查看全部江湖界面',
      ),
      h(WuxiaWeaponGallery),
    ]),
})
  .use(ui)
  .mount('#app')
