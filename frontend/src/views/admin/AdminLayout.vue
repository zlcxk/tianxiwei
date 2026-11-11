<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  ArrowUpRight,
  CalendarDays,
  ClipboardCheck,
  Compass,
  Database,
  Files,
  LayoutDashboard,
  LogOut,
  RefreshCw,
  Star,
  X,
} from 'lucide-vue-next'
import { api } from '../../api/client'
import { auth, logout } from '../../stores/auth'
import type { Overview } from '../../types'

const route = useRoute()
const router = useRouter()
const pending = ref(0)
const loggingOut = ref(false)

const nav = [
  { to: '/admin', label: '管理概览', icon: LayoutDashboard, exact: true },
  { to: '/admin/contents', label: '内容管理', icon: Files, exact: false },
  { to: '/admin/reviews', label: '待审核', icon: ClipboardCheck, exact: false, badge: true },
  { to: '/admin/discovery', label: '内容发现', icon: Compass, exact: false },
  { to: '/admin/events', label: '事件管理', icon: CalendarDays, exact: false },
  { to: '/admin/editorial', label: '编辑精选', icon: Star, exact: false },
  { to: '/admin/sources', label: '数据源', icon: Database, exact: false },
  { to: '/admin/jobs', label: '采集任务', icon: RefreshCw, exact: false },
  { to: '/admin/base', label: '基础资料', icon: Files, exact: false },
]

function isActive(item: (typeof nav)[number]) {
  return item.exact ? route.path === item.to : route.path.startsWith(item.to)
}

async function loadPending() {
  try {
    const data = await api.get<Overview>('/admin/overview')
    pending.value = data.pending_reviews
  } catch {
    /* badge is optional */
  }
}

async function signOut() {
  loggingOut.value = true
  await logout()
  loggingOut.value = false
  router.push('/login')
}

onMounted(loadPending)
watch(() => route.path, loadPending)
</script>

<template>
  <div class="min-h-dvh bg-[#f5f1eb]">
    <!-- Desktop sidebar -->
    <aside class="fixed inset-y-0 left-0 z-40 hidden w-60 flex-col border-r border-line bg-canvas md:flex">
      <div class="border-b border-line px-6 py-6">
        <RouterLink to="/" class="block">
          <div class="font-display text-xl tracking-tight">曦薇空间</div>
          <div class="mt-1 text-[9px] uppercase tracking-widest2 text-muted">Editorial Console</div>
        </RouterLink>
      </div>

      <nav class="flex-1 space-y-0.5 overflow-y-auto px-3 py-5">
        <RouterLink
          v-for="item in nav"
          :key="item.to"
          :to="item.to"
          class="group flex items-center justify-between gap-2 px-3 py-2.5 text-[12px] transition"
          :class="isActive(item) ? 'bg-white text-ink shadow-sm' : 'text-muted hover:bg-white/50 hover:text-ink'"
        >
          <span class="flex items-center gap-2.5">
            <component :is="item.icon" class="h-3.5 w-3.5" :stroke-width="1.6" />
            {{ item.label }}
          </span>
          <span
            v-if="item.badge && pending > 0"
            class="flex h-4 min-w-4 items-center justify-center rounded-full bg-accent px-1 text-[9px] text-white"
          >
            {{ pending > 99 ? '99+' : pending }}
          </span>
        </RouterLink>
      </nav>

      <div class="border-t border-line p-4">
        <RouterLink to="/" class="mb-3 flex items-center justify-between px-2 text-[10px] text-muted transition hover:text-ink">
          返回前台
          <ArrowUpRight class="h-3 w-3" />
        </RouterLink>
        <div class="flex items-center gap-2.5 px-2">
          <span class="flex h-8 w-8 items-center justify-center rounded-full bg-blush text-[11px] text-[#8f5e66]">
            {{ auth.user?.nickname?.slice(0, 1) || '管' }}
          </span>
          <div class="min-w-0 flex-1">
            <div class="truncate text-[11px]">{{ auth.user?.nickname }}</div>
            <div class="text-[9px] uppercase tracking-wider text-muted">{{ auth.user?.role }}</div>
          </div>
          <button class="text-muted transition hover:text-accent" :disabled="loggingOut" aria-label="退出登录" @click="signOut">
            <LogOut class="h-3.5 w-3.5" />
          </button>
        </div>
      </div>
    </aside>

    <!-- Mobile header -->
    <header class="sticky top-0 z-40 border-b border-line bg-canvas/95 backdrop-blur md:hidden">
      <div class="flex h-14 items-center justify-between px-4">
        <RouterLink to="/admin" class="flex items-baseline gap-2">
          <span class="font-display text-base">曦薇空间</span>
          <span class="text-[8px] uppercase tracking-widest2 text-muted">Admin</span>
        </RouterLink>
        <div class="flex items-center gap-1">
          <RouterLink to="/" class="btn-icon !h-8 !w-8 !border-0 !bg-transparent" aria-label="返回前台">
            <ArrowUpRight class="h-4 w-4" />
          </RouterLink>
          <button class="btn-icon !h-8 !w-8 !border-0 !bg-transparent" aria-label="退出登录" @click="signOut">
            <LogOut class="h-4 w-4" />
          </button>
        </div>
      </div>
      <nav class="no-scrollbar flex gap-1 overflow-x-auto px-3 pb-2">
        <RouterLink
          v-for="item in nav"
          :key="item.to"
          :to="item.to"
          class="flex shrink-0 items-center gap-1.5 border px-3 py-1.5 text-[11px] transition"
          :class="isActive(item) ? 'border-ink bg-ink text-white' : 'border-line bg-white/50 text-muted'"
        >
          {{ item.label }}
          <span v-if="item.badge && pending" class="rounded-full bg-accent px-1 text-[8px] text-white">{{ pending }}</span>
        </RouterLink>
      </nav>
    </header>

    <div class="md:pl-60">
      <main class="mx-auto w-full max-w-[1240px] px-4 py-6 pb-20 md:px-8 md:py-10">
        <RouterView v-slot="{ Component }">
          <Transition name="page" mode="out-in">
            <component :is="Component" />
          </Transition>
        </RouterView>
      </main>
    </div>
  </div>
</template>
