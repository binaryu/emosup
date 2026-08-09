import { createRouter, createWebHistory } from 'vue-router'
import AppLayout from '@/layouts/AppLayout.vue'
import ConfigView from '@/views/ConfigView.vue'
import BrowseView from '@/views/BrowseView.vue'
import BTView from '@/views/BTView.vue'
import CacheView from '@/views/CacheView.vue'
import ScanResultsView from '@/views/ScanResultsView.vue'
import TaskQueueView from '@/views/TaskQueueView.vue'
import TaskDetailView from '@/views/TaskDetailView.vue'
import LoginView from '@/views/LoginView.vue'
import { getToken } from '@/utils/api'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/login',
      name: 'login',
      component: LoginView,
      meta: { public: true },
    },
    {
      path: '/',
      redirect: '/tasks',
    },
    {
      path: '/',
      component: AppLayout,
      children: [
        {
          path: 'config',
          name: 'config',
          component: ConfigView,
        },
        {
          path: 'browse',
          name: 'browse',
          component: BrowseView,
        },
        {
          path: 'bt',
          name: 'bt',
          component: BTView,
        },
        {
          path: 'cache',
          name: 'cache',
          component: CacheView,
        },
        {
          path: 'scans',
          name: 'scans',
          component: ScanResultsView,
        },
        {
          path: 'tasks',
          name: 'tasks',
          component: TaskQueueView,
        },
        {
          path: 'tasks/:id',
          name: 'task-detail',
          component: TaskDetailView,
          props: true,
        },
      ],
    },
    {
      path: '/:pathMatch(.*)*',
      name: 'not-found',
      component: () => import('@/views/NotFoundView.vue'),
    },
  ],
})

router.beforeEach((to) => {
  const isPublic = Boolean(to.meta.public)
  const token = getToken()

  if (!isPublic && !token) {
    return {
      name: 'login',
      query: { redirect: to.fullPath },
    }
  }

  if (to.name === 'login' && token) {
    return { name: 'tasks' }
  }

  return true
})

export default router
