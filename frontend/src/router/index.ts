import { createRouter, createWebHistory } from 'vue-router'
import AppShell from '../components/AppShell.vue'
import AdminLayout from '../views/admin/AdminLayout.vue'
import HomeView from '../views/HomeView.vue'
import DiscoverView from '../views/DiscoverView.vue'
import ContentDetailView from '../views/ContentDetailView.vue'
import EventView from '../views/EventView.vue'
import WorksView from '../views/WorksView.vue'
import WorkDetailView from '../views/WorkDetailView.vue'
import CharacterDetailView from '../views/CharacterDetailView.vue'
import FavoritesView from '../views/FavoritesView.vue'
import ProfileView from '../views/ProfileView.vue'
import LoginView from '../views/LoginView.vue'
import SetupView from '../views/SetupView.vue'
import NotFoundView from '../views/NotFoundView.vue'
import AdminOverviewView from '../views/admin/AdminOverviewView.vue'
import AdminContentsView from '../views/admin/AdminContentsView.vue'
import AdminReviewsView from '../views/admin/AdminReviewsView.vue'
import AdminSourcesView from '../views/admin/AdminSourcesView.vue'
import AdminDiscoveryView from '../views/admin/AdminDiscoveryView.vue'
import AdminEventsView from '../views/admin/AdminEventsView.vue'
import AdminJobsView from '../views/admin/AdminJobsView.vue'
import AdminBaseDataView from '../views/admin/AdminBaseDataView.vue'
import { auth, ensureBootstrap, isAdmin } from '../stores/auth'

const router = createRouter({
  history: createWebHistory(),
  scrollBehavior(to, from, savedPosition) {
    if (savedPosition) return savedPosition
    if (to.hash) return { el: to.hash, behavior: 'smooth' }
    return { top: 0 }
  },
  routes: [
    {
      path: '/',
      component: AppShell,
      children: [
        { path: '', name: 'home', component: HomeView, meta: { title: '首页' } },
        { path: 'discover', name: 'discover', component: DiscoverView, meta: { title: '发现' } },
        { path: 'contents/:id', name: 'content', component: ContentDetailView, meta: { title: '内容详情' } },
        { path: 'events/:id', name: 'event', component: EventView, meta: { title: '事件' } },
        { path: 'works', name: 'works', component: WorksView, meta: { title: '作品档案' } },
        { path: 'works/:id', name: 'work', component: WorkDetailView, meta: { title: '作品详情' } },
        { path: 'characters/:id', name: 'character', component: CharacterDetailView, meta: { title: '角色档案' } },
        { path: 'favorites', name: 'favorites', component: FavoritesView, meta: { title: '收藏', requiresAuth: true } },
        { path: 'profile', name: 'profile', component: ProfileView, meta: { title: '我的' } },
        { path: ':pathMatch(.*)*', name: 'not-found', component: NotFoundView, meta: { title: '未找到页面' } },
      ],
    },
    { path: '/login', name: 'login', component: LoginView, meta: { title: '登录', guestOnly: true } },
    { path: '/setup', name: 'setup', component: SetupView, meta: { title: '初始化' } },
    {
      path: '/admin',
      component: AdminLayout,
      meta: { requiresAdmin: true },
      children: [
        { path: '', name: 'admin-overview', component: AdminOverviewView, meta: { title: '管理概览', requiresAdmin: true } },
        { path: 'contents', name: 'admin-contents', component: AdminContentsView, meta: { title: '内容管理', requiresAdmin: true } },
        { path: 'reviews', name: 'admin-reviews', component: AdminReviewsView, meta: { title: '待审核', requiresAdmin: true } },
        { path: 'sources', name: 'admin-sources', component: AdminSourcesView, meta: { title: '数据源', requiresAdmin: true } },
        { path: 'discovery', name: 'admin-discovery', component: AdminDiscoveryView, meta: { title: '内容发现', requiresAdmin: true } },
        { path: 'events', name: 'admin-events', component: AdminEventsView, meta: { title: '事件管理', requiresAdmin: true } },
        { path: 'jobs', name: 'admin-jobs', component: AdminJobsView, meta: { title: '采集任务', requiresAdmin: true } },
        { path: 'base', name: 'admin-base', component: AdminBaseDataView, meta: { title: '基础资料', requiresAdmin: true } },
      ],
    },
  ],
})

router.beforeEach(async (to) => {
  try {
    await ensureBootstrap()
  } catch {
    return true
  }
  if (auth.needsSetup && to.name !== 'setup') {
    return { name: 'setup' }
  }
  if (!auth.needsSetup && to.name === 'setup') {
    return { name: auth.user ? 'home' : 'login' }
  }
  if (to.meta.requiresAdmin) {
    if (!auth.user) return { name: 'login', query: { redirect: to.fullPath } }
    if (!isAdmin.value) return { name: 'home' }
    return true
  }
  if (to.meta.requiresAuth && !auth.user) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  if (to.meta.guestOnly && auth.user) {
    const redirect = typeof to.query.redirect === 'string' && to.query.redirect.startsWith('/')
      ? to.query.redirect
      : '/'
    return redirect
  }
  return true
})

router.afterEach((to) => {
  const title = (to.meta.title as string) || ''
  document.title = title ? `${title} · Xiwei Space` : 'Xiwei Space · 曦薇空间'
})

export default router
