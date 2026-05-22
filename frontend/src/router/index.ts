import { createRouter, createWebHistory } from 'vue-router'
import AppLayout from '@/layouts/AppLayout.vue'
import ConfigView from '@/views/ConfigView.vue'
import OpenListView from '@/views/OpenListView.vue'
import LocalBrowseView from '@/views/LocalBrowseView.vue'
import ScanResultsView from '@/views/ScanResultsView.vue'
import TaskQueueView from '@/views/TaskQueueView.vue'
import TaskDetailView from '@/views/TaskDetailView.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
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
          path: 'openlist',
          name: 'openlist',
          component: OpenListView,
        },
        {
          path: 'local',
          name: 'local',
          component: LocalBrowseView,
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

export default router
