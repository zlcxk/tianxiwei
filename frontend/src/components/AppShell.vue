<script setup lang="ts">
import { Home, Compass, Film, Heart, UserRound, Search } from 'lucide-vue-next'
import { useRoute } from 'vue-router'
import { auth } from '../stores/auth'
import BottomNav from './BottomNav.vue'

const route = useRoute()
const nav = [
  { label: '首页', to: '/', icon: Home, paths: ['/'] },
  { label: '发现', to: '/discover', icon: Compass, paths: ['/discover', '/contents'] },
  { label: '作品', to: '/works', icon: Film, paths: ['/works', '/characters'] },
  { label: '收藏', to: '/favorites', icon: Heart, paths: ['/favorites'] },
  { label: '我的', to: '/profile', icon: UserRound, paths: ['/profile'] },
]

function active(paths: string[]) {
  return paths.some((p) => (p === '/' ? route.path === '/' : route.path.startsWith(p)))
}
</script>

<template>
  <div class="min-h-dvh">
    <header class="safe-top sticky top-0 z-40 border-b border-line/80 bg-canvas/85 backdrop-blur-xl">
      <div class="shell flex h-14 items-center justify-between md:h-[74px]">
        <RouterLink to="/" class="group flex items-baseline gap-2.5">
          <span class="font-display text-lg tracking-[0.02em] md:text-xl">曦薇空间</span>
          <span class="hidden text-[9px] uppercase tracking-widest2 text-muted sm:inline">Xiwei Space</span>
        </RouterLink>

        <nav class="hidden items-center gap-8 md:flex">
          <RouterLink
            v-for="item in nav"
            :key="item.to"
            :to="item.to"
            class="relative py-2 text-[13px] tracking-wide transition"
            :class="active(item.paths) ? 'text-ink' : 'text-muted hover:text-ink'"
          >
            {{ item.label }}
            <span v-if="active(item.paths)" class="absolute inset-x-0 -bottom-[1px] h-px bg-accent" />
          </RouterLink>
        </nav>

        <div class="flex items-center gap-2">
          <RouterLink to="/discover" class="btn-icon hidden md:inline-flex" aria-label="搜索内容">
            <Search class="h-4 w-4" />
          </RouterLink>
          <RouterLink v-if="auth.user" to="/profile" class="flex items-center gap-2.5">
            <span class="flex h-8 w-8 items-center justify-center rounded-full bg-blush text-xs font-medium text-[#8f5e66]">
              {{ auth.user.nickname.slice(0, 1) || '我' }}
            </span>
            <span class="hidden max-w-24 truncate text-xs text-muted lg:block">{{ auth.user.nickname }}</span>
          </RouterLink>
          <RouterLink v-else to="/login" class="btn-ghost !min-h-9 !px-3.5 !py-1.5 text-xs">登录</RouterLink>
        </div>
      </div>
    </header>

    <main class="pb-24 md:pb-0">
      <RouterView v-slot="{ Component }">
        <Transition name="page" mode="out-in">
          <component :is="Component" />
        </Transition>
      </RouterView>
    </main>

    <footer class="hidden border-t border-line md:block">
      <div class="shell flex flex-col gap-6 py-10 lg:flex-row lg:items-end lg:justify-between">
        <div>
          <div class="font-display text-2xl">曦薇空间</div>
          <p class="mt-2 max-w-md text-xs leading-6 text-muted">
            单艺人专属信息空间。内容来自公开渠道，站内仅保留摘要与来源链接；图片素材将在获得授权后接入。
          </p>
        </div>
        <div class="flex items-center gap-6 text-[11px] text-muted">
          <RouterLink to="/works" class="hover:text-ink">作品档案</RouterLink>
          <RouterLink to="/discover" class="hover:text-ink">发现</RouterLink>
          <span>© 2026 Xiwei Space</span>
        </div>
      </div>
    </footer>

    <BottomNav />
  </div>
</template>
