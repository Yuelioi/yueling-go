import { createRouter, createWebHistory } from 'vue-router'
import { api, UnauthorizedError } from './api'
import { configurationTarget } from './settings'

const LoginView = () => import('./views/LoginView.vue')
const DashboardView = () => import('./views/DashboardView.vue')
const PluginsView = () => import('./views/PluginsView.vue')
const CommandUsageView = () => import('./views/CommandUsageView.vue')
const GroupActionsView = () => import('./views/GroupActionsView.vue')
const AIView = () => import('./views/AIView.vue')
const DigestView = () => import('./views/DigestView.vue')
const SubscriptionsView = () => import('./views/SubscriptionsView.vue')
const KnowledgeView = () => import('./views/KnowledgeView.vue')
const AffinityView = () => import('./views/AffinityView.vue')
const MemoryView = () => import('./views/MemoryView.vue')
const BilibiliWorkspaceView = () => import('./views/BilibiliWorkspaceView.vue')
const SettingsView = () => import('./views/SettingsView.vue')

const loginPath = '/login'

export const router = createRouter({
  history: createWebHistory(),
  scrollBehavior(_to, _from, savedPosition) {
    return savedPosition ?? { top: 0 }
  },
  routes: [
    { path: loginPath, component: LoginView },
    { path: '/', component: DashboardView },
    {
      path: '/configuration',
      redirect: (to) => configurationTarget(to.query.section ?? to.query.tab),
    },
    { path: '/plugins', component: PluginsView },
    { path: '/command-usage', component: CommandUsageView },
    { path: '/group-actions', component: GroupActionsView },
    { path: '/ai', component: AIView },
    {
      path: '/ai-style',
      redirect: (to) => ({ path: '/ai', query: { ...to.query, tab: 'style' } }),
    },
    { path: '/digests', component: DigestView },
    { path: '/feeds', component: SubscriptionsView },
    { path: '/knowledge', component: KnowledgeView },
    { path: '/affinity', component: AffinityView },
    { path: '/memories', component: MemoryView },
    { path: '/bilibili', component: BilibiliWorkspaceView },
    {
      path: '/images',
      component: SettingsView,
      meta: {
        settingsArea: 'images',
        title: '图片与打包',
        description: '管理图片转换、素材类目与打包限制。',
        icon: 'i-tabler-photo',
      },
    },
    {
      path: '/tools',
      component: SettingsView,
      meta: {
        settingsArea: 'tools',
        title: '工具与服务',
        description: '配置天气、搜索、代理和表情包服务。',
        icon: 'i-tabler-tool',
      },
    },
    {
      path: '/connections',
      component: SettingsView,
      meta: {
        settingsArea: 'connections',
        title: '连接配置',
        description: '管理 QQ 连接和对外消息接口。',
        icon: 'i-tabler-plug',
      },
    },
    {
      path: '/system',
      component: SettingsView,
      meta: {
        settingsArea: 'system',
        title: '系统配置',
        description: '管理机器人、后台与数据库连接池。',
        icon: 'i-tabler-settings',
      },
    },
  ],
})

router.beforeEach(async (to) => {
  if (to.path === loginPath) {
    return true
  }

  try {
    await api.me()
    return true
  } catch (err) {
    if (err instanceof UnauthorizedError) {
      return { path: loginPath, query: { redirect: to.fullPath } }
    }
    return true
  }
})
