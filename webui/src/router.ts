import { createRouter, createWebHistory } from 'vue-router'
import { api, UnauthorizedError } from './api'

const LoginView = () => import('./views/LoginView.vue')
const DashboardView = () => import('./views/DashboardView.vue')
const PluginGroupsView = () => import('./views/PluginGroupsView.vue')
const CommandUsageView = () => import('./views/CommandUsageView.vue')
const GroupActionsView = () => import('./views/GroupActionsView.vue')
const AIStyleView = () => import('./views/AIStyleView.vue')
const DigestView = () => import('./views/DigestView.vue')
const FeedView = () => import('./views/FeedView.vue')
const KnowledgeView = () => import('./views/KnowledgeView.vue')
const AffinityView = () => import('./views/AffinityView.vue')
const MemoryView = () => import('./views/MemoryView.vue')

const loginPath = '/login'

export const router = createRouter({
  history: createWebHistory(),
  scrollBehavior(_to, _from, savedPosition) {
    return savedPosition ?? { top: 0 }
  },
  routes: [
    { path: loginPath, component: LoginView },
    { path: '/', component: DashboardView },
    { path: '/plugins', component: PluginGroupsView },
    { path: '/command-usage', component: CommandUsageView },
    { path: '/group-actions', component: GroupActionsView },
    { path: '/ai-style', component: AIStyleView },
    { path: '/digests', component: DigestView },
    { path: '/feeds', component: FeedView },
    { path: '/knowledge', component: KnowledgeView },
    { path: '/affinity', component: AffinityView },
    { path: '/memories', component: MemoryView },
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
