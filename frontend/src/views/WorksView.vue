<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Film, RefreshCw } from 'lucide-vue-next'
import { api } from '../api/client'
import type { Work } from '../types'
import { formatYear, workStatusLabels, workTypeLabels } from '../lib/format'
import CoverArt from '../components/CoverArt.vue'
import EmptyState from '../components/EmptyState.vue'
import LoadingBlock from '../components/LoadingBlock.vue'

const works = ref<Work[]>([])
const loading = ref(true)
const error = ref('')
const filter = ref<'all' | 'drama' | 'movie' | 'variety' | 'other'>('all')

const typeFilters: { value: typeof filter.value; label: string }[] = [
  { value: 'all', label: '全部' },
  { value: 'drama', label: '剧集' },
  { value: 'movie', label: '电影' },
  { value: 'variety', label: '综艺' },
  { value: 'other', label: '其他' },
]

const filtered = computed(() =>
  filter.value === 'all' ? works.value : works.value.filter((w) => w.type === filter.value),
)

const groups = computed(() => {
  const map = new Map<string, Work[]>()
  for (const work of filtered.value) {
    const year = formatYear(work.release_date) || '待定'
    if (!map.has(year)) map.set(year, [])
    map.get(year)!.push(work)
  }
  return Array.from(map.entries())
})

async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await api.get<{ items: Work[] }>('/works')
    works.value = data.items
  } catch (err) {
    error.value = err instanceof Error ? err.message : '作品加载失败'
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<template>
  <div class="shell pb-16 pt-8 md:pb-24 md:pt-12">
    <header class="mb-8 border-b border-line pb-7 md:mb-12">
      <div class="eyebrow mb-3">Filmography</div>
      <div class="flex flex-col gap-5 md:flex-row md:items-end md:justify-between">
        <div>
          <h1 class="display-title text-[36px] md:text-[48px]">作品档案</h1>
          <p class="mt-2 max-w-xl text-sm leading-7 text-muted">
            按时间整理已公开的影视作品与角色。资料以公开来源为准，持续补充。
          </p>
        </div>
        <div class="flex items-center gap-1 border border-line bg-white/40 p-0.5">
          <button
            v-for="option in typeFilters"
            :key="option.value"
            class="px-3.5 py-1.5 text-[11px] transition"
            :class="filter === option.value ? 'bg-ink text-white' : 'text-muted hover:text-ink'"
            @click="filter = option.value"
          >
            {{ option.label }}
          </button>
        </div>
      </div>
    </header>

    <LoadingBlock v-if="loading" type="grid" :rows="8" />

    <EmptyState
      v-else-if="error"
      title="作品档案加载失败"
      :description="error"
      action-label="重新加载"
      @action="load"
    />

    <EmptyState
      v-else-if="!filtered.length"
      :icon="Film"
      title="暂时没有这类作品"
      description="切换分类，或稍后再来看看。"
      action-label="查看全部"
      @action="filter = 'all'"
    />

    <div v-else class="space-y-12 md:space-y-16">
      <section v-for="[year, items] in groups" :key="year" class="grid gap-5 md:grid-cols-[110px_1fr] md:gap-8">
        <div class="flex items-baseline gap-3 md:sticky md:top-24 md:block md:self-start">
          <h2 class="font-display text-[38px] leading-none tracking-tight md:text-[44px]">{{ year }}</h2>
          <span class="text-[10px] uppercase tracking-widest2 text-muted md:mt-2 md:block">{{ items.length }} 部作品</span>
          <span class="hidden h-px w-full bg-line md:mt-4 md:block" />
        </div>
        <div class="grid grid-cols-2 gap-x-4 gap-y-8 sm:grid-cols-3 lg:grid-cols-4">
          <RouterLink
            v-for="work in items"
            :key="work.id"
            :to="{ name: 'work', params: { id: work.id } }"
            class="group"
          >
            <div class="overflow-hidden">
              <CoverArt
                :title="work.title"
                :subtitle="workTypeLabels[work.type] || '作品'"
                :meta="formatYear(work.release_date)"
                :src="work.cover"
                aspect="portrait"
                class="transition duration-500 group-hover:scale-[1.02]"
              />
            </div>
            <div class="mt-3">
              <div class="flex items-center gap-1.5 text-[9px] uppercase tracking-[0.14em] text-muted">
                <span>{{ workTypeLabels[work.type] || '作品' }}</span>
                <span class="h-2 w-px bg-line" />
                <span>{{ workStatusLabels[work.status] || '资料' }}</span>
              </div>
              <h3 class="mt-1.5 truncate font-display text-lg tracking-tight transition group-hover:text-accent">{{ work.title }}</h3>
              <p class="mt-1 truncate text-[10px] text-muted">{{ work.platform || '播出平台待补充' }}</p>
              <p v-if="work.characters?.length" class="mt-2 line-clamp-1 text-[10px] text-muted">
                饰 {{ work.characters.map((c) => c.name).join(' / ') }}
              </p>
            </div>
          </RouterLink>
        </div>
      </section>
    </div>

    <div class="mt-14 flex items-center justify-between border-t border-line pt-5">
      <p class="text-[10px] leading-5 text-muted">作品信息来自公开资料，具体以官方发布为准。</p>
      <button class="inline-flex items-center gap-1.5 text-[11px] text-muted transition hover:text-ink" :disabled="loading" @click="load">
        <RefreshCw class="h-3 w-3" :class="loading ? 'animate-spin' : ''" />
        刷新
      </button>
    </div>
  </div>
</template>
