<script setup lang="ts">
import { nextTick, ref, watch } from 'vue'
import { Home, Compass, Film, Heart, UserRound, Search, ArrowUpRight } from 'lucide-vue-next'
import { useRoute, useRouter } from 'vue-router'
import { auth } from '../stores/auth'
import BottomNav from './BottomNav.vue'
import Modal from './Modal.vue'

const route = useRoute()
const router = useRouter()
const nav = [
  { label: '首页', to: '/', icon: Home, paths: ['/'] },
  { label: '发现', to: '/discover', icon: Compass, paths: ['/discover', '/contents'] },
  { label: '作品', to: '/works', icon: Film, paths: ['/works', '/characters'] },
  { label: '收藏', to: '/favorites', icon: Heart, paths: ['/favorites'] },
  { label: '我的', to: '/profile', icon: UserRound, paths: ['/profile'] },
]
const quickLinks = [
  { label: '全部动态', to: '/discover' },
  { label: '作品档案', to: '/works' },
  { label: '我的收藏', to: '/favorites' },
]

const searchOpen = ref(false)
const keyword = ref('')
const searchInput = ref<HTMLInputElement | null>(null)

function active(paths: string[]) {
  return paths.some((p) => (p === '/' ? route.path === '/' : route.path.startsWith(p)))
}

function openSearch() {
  searchOpen.value = true
  nextTick(() => searchInput.value?.focus())
}

function closeSearch() {
  searchOpen.value = false
}

function submitSearch() {
  const q = keyword.value.trim()
  searchOpen.value = false
  router.push({ name: 'discover', query: q ? { keyword: q } : {} })
}

watch(
  () => route.fullPath,
  () => {
    searchOpen.value = false
  },
)
</script>

<template>
  <div class="min-h-dvh">
    <header class="safe-top sticky top-0 z-40 border-b border-line/80 bg-canvas/85 backdrop-blur-xl">
      <div class="shell flex h-14 items-center justify-between md:h-[74px]">
        <RouterLink to="/" class="group flex items-baseline gap-2.5">
          <span class="font-display text-lg tracking-[0.02em] md:text-xl">曦薇空间</span>
          <span class="hidden text-[10px] uppercase tracking-widest2 text-muted sm:inline">Xiwei Space</span>
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

        <div class="flex items-center gap-2 md:gap-3">
          <span class="hidden text-[10px] uppercase tracking-widest2 text-muted xl:inline">Archive · 2026</span>
          <button
            class="hidden h-9 items-center gap-2 border border-line bg-white/45 px-3 text-[11px] tracking-wide text-muted transition hover:border-ink/25 hover:bg-white hover:text-ink md:inline-flex"
            aria-label="打开搜索"
            aria-haspopup="dialog"
            @click="openSearch"
          >
            <Search class="h-3.5 w-3.5" />
            搜索
          </button>
          <button
            class="btn-icon h-9 w-9 md:hidden"
            aria-label="打开搜索"
            aria-haspopup="dialog"
            @click="openSearch"
          >
            <Search class="h-4 w-4" />
          </button>
          <RouterLink v-if="auth.user" to="/profile" class="flex items-center gap-2.5">
            <span class="flex h-8 w-8 items-center justify-center rounded-full bg-blush text-xs font-medium text-accent-deep">
              {{ auth.user.nickname.slice(0, 1) || '我' }}
            </span>
            <span class="hidden max-w-24 truncate text-xs text-muted lg:block">{{ auth.user.nickname }}</span>
          </RouterLink>
          <RouterLink v-else to="/login" class="btn-ghost !min-h-9 !px-3.5 !py-1.5 text-xs">登录</RouterLink>
        </div>
      </div>
    </header>

    <Modal
      :open="searchOpen"
      title="搜索这个空间"
      description="按标题、来源、作品或角色查找；结果会出现在发现页。"
      size="sm"
      @close="closeSearch"
    >
      <form @submit.prevent="submitSearch">
        <label class="field-label" for="global-search">关键词</label>
        <input
          id="global-search"
          ref="searchInput"
          v-model="keyword"
          class="field"
          type="search"
          placeholder="例如：剧集、采访、角色名"
          autocomplete="off"
        />
        <div class="mt-3 flex items-center gap-2">
          <button type="submit" class="btn-primary !min-h-10 flex-1 text-xs">
            <Search class="h-3.5 w-3.5" />
            搜索
          </button>
          <RouterLink to="/discover" class="btn-ghost !min-h-10 text-xs" @click="closeSearch">
            浏览全部
          </RouterLink>
        </div>
      </form>
      <div class="mt-6 border-t border-line pt-4">
        <p class="text-[11px] uppercase tracking-widest2 text-muted">快速前往</p>
        <div class="mt-3 flex flex-wrap gap-2">
          <RouterLink
            v-for="link in quickLinks"
            :key="link.to"
            :to="link.to"
            class="tag gap-1.5 transition hover:border-ink/25 hover:text-ink"
            @click="closeSearch"
          >
            {{ link.label }}
            <ArrowUpRight class="h-3 w-3" />
          </RouterLink>
        </div>
      </div>
    </Modal>

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
            单艺人专属信息空间。内容来自公开渠道，站内保留摘要与来源链接；图片与人物、作品对应，缺失时使用站内抽象封面。
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
