<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Bookmark, Heart, Trash2 } from 'lucide-vue-next'
import { api } from '../api/client'
import type { FavoriteItem } from '../types'
import { favoriteTypeLabels, formatDate } from '../lib/format'
import { toast } from '../stores/toast'
import CoverArt from '../components/CoverArt.vue'
import EmptyState from '../components/EmptyState.vue'
import LoadingBlock from '../components/LoadingBlock.vue'

const items = ref<FavoriteItem[]>([])
const loading = ref(true)
const error = ref('')
const activeType = ref<'all' | 'content' | 'work' | 'character'>('all')
const removing = ref('')

const tabs: { value: typeof activeType.value; label: string }[] = [
  { value: 'all', label: '全部' },
  { value: 'content', label: '动态' },
  { value: 'work', label: '作品' },
  { value: 'character', label: '角色' },
]

const filtered = computed(() =>
  activeType.value === 'all' ? items.value : items.value.filter((item) => item.target_type === activeType.value),
)

function countFor(type: typeof activeType.value) {
  return type === 'all' ? items.value.length : items.value.filter((i) => i.target_type === type).length
}

function targetRoute(item: FavoriteItem) {
  if (item.unavailable) return null
  switch (item.target_type) {
    case 'content':
      return { name: 'content', params: { id: item.target_id } }
    case 'work':
      return { name: 'work', params: { id: item.target_id } }
    default:
      return { name: 'character', params: { id: item.target_id } }
  }
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await api.get<{ items: FavoriteItem[] }>('/favorites')
    items.value = data.items
  } catch (err) {
    error.value = err instanceof Error ? err.message : '收藏加载失败'
  } finally {
    loading.value = false
  }
}

async function remove(item: FavoriteItem) {
  if (removing.value) return
  removing.value = item.id
  try {
    await api.del(`/favorites/${item.id}`)
    items.value = items.value.filter((i) => i.id !== item.id)
    toast.success('已取消收藏')
  } catch (err) {
    toast.error(err instanceof Error ? err.message : '操作失败')
  } finally {
    removing.value = ''
  }
}

onMounted(load)
</script>

<template>
  <div class="shell pb-16 pt-8 md:pb-24 md:pt-12">
    <header class="mb-7 md:mb-10">
      <div class="eyebrow mb-3">Saved</div>
      <div class="flex flex-col gap-4 md:flex-row md:items-end md:justify-between">
        <div>
          <h1 class="display-title text-[36px] md:text-[48px]">我的收藏</h1>
          <p class="mt-2 text-sm text-muted">收藏内容会保留标题与来源快照；原内容下架后仍可识别。</p>
        </div>
        <div class="flex items-center gap-1 self-start border border-line bg-white/40 p-0.5 md:self-auto">
          <button
            v-for="tab in tabs"
            :key="tab.value"
            class="px-3 py-1.5 text-[11px] transition"
            :class="activeType === tab.value ? 'bg-ink text-white' : 'text-muted hover:text-ink'"
            @click="activeType = tab.value"
          >
            {{ tab.label }}
            <span class="ml-0.5 opacity-60">{{ countFor(tab.value) }}</span>
          </button>
        </div>
      </div>
    </header>

    <LoadingBlock v-if="loading" :rows="4" />

    <EmptyState
      v-else-if="error"
      title="收藏加载失败"
      :description="error"
      action-label="重新加载"
      @action="load"
    />

    <EmptyState
      v-else-if="!filtered.length"
      :icon="Heart"
      :title="activeType === 'all' ? '还没有收藏任何内容' : '这个分类下还没有收藏'"
      description="在内容、作品或角色页面点击收藏，就会出现在这里。"
      action-label="去发现内容"
      to="/discover"
    />

    <div v-else class="border-t border-line">
      <div v-for="item in filtered" :key="item.id" class="group flex items-center gap-4 border-b border-line py-4">
        <component
          :is="targetRoute(item) ? 'RouterLink' : 'div'"
          :to="targetRoute(item) || undefined"
          class="flex min-w-0 flex-1 items-center gap-4"
          :class="item.unavailable ? 'opacity-55' : ''"
        >
          <CoverArt
            :title="item.title"
            :subtitle="favoriteTypeLabels[item.target_type]"
            :src="item.cover"
            class="w-16 shrink-0 md:w-20"
            aspect="square"
          />
          <div class="min-w-0 flex-1">
            <div class="flex flex-wrap items-center gap-2 text-[9px] uppercase tracking-[0.14em] text-muted">
              <span>{{ favoriteTypeLabels[item.target_type] }}</span>
              <span class="h-2.5 w-px bg-line" />
              <span>{{ formatDate(item.created_at) }} 收藏</span>
              <span v-if="item.unavailable" class="tag !border-accent/25 !py-0.5 !text-[9px] text-accent">已下架</span>
            </div>
            <h2 class="mt-1.5 line-clamp-1 font-display text-[16px] tracking-tight md:text-lg">
              {{ item.title }}
            </h2>
            <p v-if="item.subtitle" class="mt-1 line-clamp-1 text-[11px] text-muted">{{ item.subtitle }}</p>
          </div>
        </component>
        <a
          v-if="item.source_url && !item.unavailable"
          :href="item.source_url"
          target="_blank"
          rel="noopener noreferrer nofollow"
          class="hidden shrink-0 text-[10px] text-muted underline underline-offset-4 hover:text-ink sm:block"
        >
          来源
        </a>
        <button
          class="flex h-9 w-9 shrink-0 items-center justify-center text-muted transition hover:bg-blush/40 hover:text-accent"
          :disabled="removing === item.id"
          aria-label="取消收藏"
          @click="remove(item)"
        >
          <Trash2 class="h-4 w-4" :class="removing === item.id ? 'animate-pulse' : ''" />
        </button>
      </div>
    </div>

    <div v-if="items.length && !loading" class="mt-6 flex items-center gap-2 text-[10px] text-muted">
      <Bookmark class="h-3 w-3" />
      共 {{ items.length }} 条收藏 · 快照仅用于站内识别，不随原内容更新
    </div>
  </div>
</template>
