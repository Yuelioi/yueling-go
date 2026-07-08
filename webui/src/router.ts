import { createRouter, createWebHistory } from 'vue-router'
import { api, UnauthorizedError } from './api'
import LoginView from './views/LoginView.vue'
import PluginGroupsView from './views/PluginGroupsView.vue'
import AffinityView from './views/AffinityView.vue'
import GroupActionsView from './views/GroupActionsView.vue'

const loginPath = '/login'

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: loginPath, component: LoginView },
    { path: '/', component: PluginGroupsView },
    { path: '/group-actions', component: GroupActionsView },
    { path: '/affinity', component: AffinityView },
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
