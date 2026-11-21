import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'

const routes: RouteRecordRaw[] = [
  {
    path: '/',
    name: 'home',
    redirect: { name: 'stage', params: { stage: 'uploaded' } },
  },
  {
    path: '/stage/:stage',
    name: 'stage',
    component: () => import('./views/StageView.vue'),
  },
  {
    path: '/books/:id',
    name: 'book',
    component: () => import('./views/BookView.vue'),
  },
  {
    path: '/books/:id/pages/:index',
    name: 'reader',
    component: () => import('./views/ReaderView.vue'),
  },
  {
    path: '/:pathMatch(.*)*',
    redirect: { name: 'home' },
  },
]

export const router = createRouter({
  history: createWebHistory(),
  routes,
})
