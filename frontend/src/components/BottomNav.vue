<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { Compass, Film, Heart, House, UserRound } from 'lucide-vue-next'

const route = useRoute()

const items = [
  { name: 'home', label: '首页', icon: House, match: (path: string) => path === '/' },
  { name: 'discover', label: '发现', icon: Compass, match: (path: string) => path.startsWith('/discover') || path.startsWith('/contents') },
  { name: 'works', label: '作品', icon: Film, match: (path: string) => path.startsWith('/works') || path.startsWith('/characters') },
  { name: 'favorites', label: '收藏', icon: Heart, match: (path: string) => path.startsWith('/favorites') },
  { name: 'profile', label: '我的', icon: UserRound, match: (path: string) => path.startsWith('/profile') },
]

const activeName = computed(() => items.find((item) => item.match(route.path))?.name || '')
</script>

<template>
  <nav class="safe-bottom fixed inset-x-0 bottom-0 z-50 border-t border-line bg-canvas/92 backdrop-blur-xl md:hidden">
    <div class="grid h-[62px] grid-cols-5">
      <RouterLink
        v-for="item in items"
        :key="item.name"
        :to="item.name === 'home' ? '/' : `/${item.name}`"
        class="relative flex flex-col items-center justify-center gap-1 text-[10px] tracking-wide transition"
        :class="activeName === item.name ? 'text-ink' : 'text-muted'"
      >
        <span
          v-if="activeName === item.name"
          class="absolute top-0 h-[2px] w-5 bg-accent"
        />
        <component :is="item.icon" class="h-[19px] w-[19px]" :stroke-width="activeName === item.name ? 2 : 1.6" />
        <span>{{ item.label }}</span>
      </RouterLink>
    </div>
  </nav>
</template>
