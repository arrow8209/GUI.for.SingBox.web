import { createRouter, createWebHashHistory } from 'vue-router'

import { getStoredCsrf } from '@/stores/auth'

import routes from './routes'

const router = createRouter({
  history: createWebHashHistory(import.meta.env.BASE_URL),
  routes,
})

router.beforeEach((to, _from, next) => {
  // 用 CSRF token 是否存在判断登录态（cookie 是 HttpOnly JS 看不到，CSRF cookie 同生命周期可读）
  const authed = !!getStoredCsrf()
  if (!to.meta.public && !authed) {
    next({ path: '/login', query: { redirect: to.fullPath } })
    return
  }
  if (to.path === '/login' && authed) {
    next({ path: '/' })
    return
  }
  next()
})

export default router
