<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowDown, ArrowLeft, Search, SlidersHorizontal, X } from 'lucide-vue-next'
import { api } from '../api/client'
import type { Character, CharacterDetail, Content, Paged, Scene, Work, WorkDetail } from '../types'
import { contentTypeLabels, sourceTypeLabels } from '../lib/format'
import ContentCard from '../components/ContentCard.vue'
import EmptyState from '../components/EmptyState.vue'
import LoadingBlock from '../components/LoadingBlock.vue'
import Modal from '../components/Modal.vue'
import SceneIcon from '../components/SceneIcon.vue'

const route = useRoute()
const router = useRouter()

function queryString(key: string): string {
  const value = route.query[key]
  return typeof value === 'string' ? value : ''
}

const keyword = ref(queryString('keyword'))
const sort = ref<'latest' | 'recommended'>(queryString('sort') === 'recommended' ? 'recommended' : 'latest')
const selectedScenes = ref<string[]>(queryString('scene') ? queryString('scene').split(',').filter(Boolean) : [])
const contentType = ref(queryString('content_type'))
const sourceType = ref(queryString('source_type'))
const range = ref(queryString('range'))
const workId = ref(queryString('work_id'))
const characterId = ref(queryString('character_id'))
const contextWork = ref<Work | null>(null)
const contextCharacter = ref<Character | null>(null)

const scenes = ref<Scene[]>([])
const items = ref<Content[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 12
const loading = ref(true)
const loadingMore = ref(false)
const error = ref('')
const filterOpen = ref(false)

const draftContentType = ref('')
const draftSourceType = ref('')
const draftRange = ref('')

const hasMore = computed(() => items.value.length < total.value)
const activeFilterCount = computed(
  () => [contentType.value, sourceType.value, range.value].filter(Boolean).length + (selectedScenes.value.length ? 1 : 0),
)
const activeSceneNames = computed(() => {
  if (!selectedScenes.value.length) return '全部场景'
  return (
    scenes.value
      .filter((s) => selectedScenes.value.includes(s.code))
      .map((s) => s.name)
      .join(' · ') || '已选场景'
  )
})
const hasContext = computed(() => Boolean(workId.value || characterId.value))
const contextLabel = computed(() => {
  if (contextCharacter.value) {
    return contextCharacter.value.work?.title
      ? `${contextCharacter.value.name} · ${contextCharacter.value.work.title}`
      : contextCharacter.value.name
  }
  return contextWork.value?.title ?? '相关内容'
})
const contextKind = computed(() => (characterId.value ? '角色' : '作品'))
const contextTo = computed(() => {
  if (characterId.value) return { name: 'character', params: { id: characterId.value } }
  return { name: 'work', params: { id: workId.value } }
})

const rangeOptions = [
  { value: '', label: '不限时间' },
  { value: 'today', label: '今天' },
  { value: '7d', label: '近 7 天' },
  { value: '30d', label: '近 30 天' },
  { value: '90d', label: '近 90 天' },
]

function buildParams(nextPage: number): string {
  const params = new URLSearchParams()
  params.set('sort', sort.value)
  params.set('page', String(nextPage))
  params.set('page_size', String(pageSize))
  if (keyword.value.trim()) params.set('keyword', keyword.value.trim())
  if (selectedScenes.value.length) params.set('scene', selectedScenes.value.join(','))
  if (contentType.value) params.set('content_type', contentType.value)
  if (sourceType.value) params.set('source_type', sourceType.value)
  if (range.value) params.set('range', range.value)
  if (workId.value) params.set('work_id', workId.value)
  if (characterId.value) params.set('character_id', characterId.value)
  return params.toString()
}

function syncURL() {
  const query: Record<string, string> = {}
  if (keyword.value.trim()) query.keyword = keyword.value.trim()
  if (sort.value !== 'latest') query.sort = sort.value
  if (selectedScenes.value.length) query.scene = selectedScenes.value.join(',')
  if (contentType.value) query.content_type = contentType.value
  if (sourceType.value) query.source_type = sourceType.value
  if (range.value) query.range = range.value
  if (workId.value) query.work_id = workId.value
  if (characterId.value) query.character_id = characterId.value
  router.replace({ name: 'discover', query })
}

async function load(reset = true) {
  if (reset) {
    page.value = 1
    loading.value = true
    error.value = ''
  } else {
    loadingMore.value = true
  }
  try {
    const data = await api.get<Paged<Content>>(`/contents?${buildParams(page.value)}`)
    items.value = reset ? data.items : [...items.value, ...data.items]
    total.value = data.total
  } catch (err) {
    error.value = err instanceof Error ? err.message : '内容加载失败'
  } finally {
    loading.value = false
    loadingMore.value = false
  }
}

async function loadMore() {
  if (loadingMore.value || !hasMore.value) return
  page.value += 1
  await load(false)
}

async function toggleScene(code: string) {
  selectedScenes.value = selectedScenes.value.includes(code)
    ? selectedScenes.value.filter((c) => c !== code)
    : [...selectedScenes.value, code]
  syncURL()
  await load()
}

async function clearScenes() {
  selectedScenes.value = []
  syncURL()
  await load()
}

function setSort(value: 'latest' | 'recommended') {
  if (sort.value === value) return
  sort.value = value
  syncURL()
  load()
}

function openFilters() {
  draftContentType.value = contentType.value
  draftSourceType.value = sourceType.value
  draftRange.value = range.value
  filterOpen.value = true
}

async function applyFilters() {
  contentType.value = draftContentType.value
  sourceType.value = draftSourceType.value
  range.value = draftRange.value
  filterOpen.value = false
  syncURL()
  await load()
}

async function clearFilters() {
  contentType.value = ''
  sourceType.value = ''
  range.value = ''
  selectedScenes.value = []
  workId.value = ''
  characterId.value = ''
  contextWork.value = null
  contextCharacter.value = null
  filterOpen.value = false
  syncURL()
  await load()
}

async function clearContext() {
  workId.value = ''
  characterId.value = ''
  contextWork.value = null
  contextCharacter.value = null
  syncURL()
  await load()
}

async function loadContext() {
  if (characterId.value && !contextCharacter.value) {
    try {
      const data = await api.get<CharacterDetail>(`/characters/${characterId.value}`)
      contextCharacter.value = data.character
    } catch {
      contextCharacter.value = null
    }
  }
  if (workId.value && !characterId.value && !contextWork.value) {
    try {
      const data = await api.get<WorkDetail>(`/works/${workId.value}`)
      contextWork.value = data.work
    } catch {
      contextWork.value = null
    }
  }
}

let searchTimer: ReturnType<typeof setTimeout> | undefined
watch(keyword, () => {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(() => {
    syncURL()
    load()
  }, 380)
})

onMounted(async () => {
  load()
  loadContext()
  try {
    const res = await api.get<{ items: Scene[] }>('/scenes')
    scenes.value = res.items
  } catch {
    scenes.value = []
  }
})
</script>

<template>
  <div class="shell pb-16 pt-8 md:pb-24 md:pt-12">
    <header class="mb-7 md:mb-10">
      <div class="eyebrow mb-3">Discover</div>
      <div class="flex flex-col gap-4 md:flex-row md:items-end md:justify-between">
        <div>
          <h1 class="display-title text-[36px] md:text-[48px]">发现</h1>
          <p class="mt-2 text-sm text-muted">按场景筛选公开动态，找到此刻最想看的内容。</p>
        </div>
        <div class="flex items-center gap-2 text-[11px] text-muted">
          <span class="h-1 w-1 rounded-full bg-moss" />
          内容经审核后公开 · 共 {{ total }} 条
        </div>
      </div>
    </header>

    <!-- Search -->
    <div class="relative">
      <Search class="pointer-events-none absolute left-4 top-1/2 h-4 w-4 -translate-y-1/2 text-muted" />
      <input
        v-model="keyword"
        type="search"
        class="field !py-3.5 pl-11 pr-11 text-[13px]"
        placeholder="搜索作品、角色、活动或关键词"
        aria-label="搜索内容"
      />
      <button
        v-if="keyword"
        class="absolute right-3 top-1/2 flex h-7 w-7 -translate-y-1/2 items-center justify-center text-muted hover:text-ink"
        aria-label="清空搜索"
        @click="keyword = ''"
      >
        <X class="h-4 w-4" />
      </button>
    </div>

    <!-- Context: work or character -->
    <div
      v-if="hasContext"
      class="mt-4 flex items-center justify-between gap-3 border border-line bg-blush/25 px-4 py-3"
    >
      <div class="flex min-w-0 items-center gap-3">
        <span class="shrink-0 border border-accent/30 px-2 py-1 text-[9px] tracking-widest text-accent">{{ contextKind }}</span>
        <div class="min-w-0">
          <p class="text-[9px] uppercase tracking-widest text-muted">正在查看相关内容</p>
          <RouterLink :to="contextTo" class="group inline-flex max-w-full items-center gap-1.5">
            <span class="truncate font-display text-base leading-6 transition group-hover:text-accent">{{ contextLabel }}</span>
            <ArrowLeft class="h-3 w-3 shrink-0 text-muted transition group-hover:text-accent" />
          </RouterLink>
        </div>
      </div>
      <button class="shrink-0 text-[10px] text-muted underline underline-offset-4 hover:text-ink" @click="clearContext">
        查看全部
      </button>
    </div>

    <!-- Scene chips -->
    <div class="no-scrollbar -mx-5 mt-5 flex gap-2 overflow-x-auto px-5 pb-1 md:mx-0 md:flex-wrap md:px-0">
      <button
        class="shrink-0 border px-3.5 py-2 text-[11px] transition"
        :class="!selectedScenes.length ? 'border-ink bg-ink text-white' : 'border-line bg-white/50 text-muted hover:border-ink/25 hover:text-ink'"
        @click="clearScenes"
      >
        全部
      </button>
      <button
        v-for="scene in scenes.filter((s) => s.code !== 'all')"
        :key="scene.id"
        class="inline-flex shrink-0 items-center gap-1.5 border px-3.5 py-2 text-[11px] transition"
        :class="selectedScenes.includes(scene.code)
          ? 'border-accent/50 bg-blush/50 text-ink'
          : 'border-line bg-white/50 text-muted hover:border-ink/25 hover:text-ink'"
        @click="toggleScene(scene.code)"
      >
        <SceneIcon :code="scene.code" class="h-3.5 w-3.5" />
        {{ scene.name }}
      </button>
    </div>

    <!-- Toolbar -->
    <div class="mt-6 flex flex-wrap items-center justify-between gap-3 border-y border-line py-3">
      <div class="flex items-center gap-2">
        <button class="btn-ghost !min-h-9 !border-line !px-3 !py-1.5 text-[11px]" @click="openFilters">
          <SlidersHorizontal class="h-3.5 w-3.5" />
          筛选
          <span v-if="activeFilterCount" class="flex h-4 min-w-4 items-center justify-center rounded-full bg-accent px-1 text-[9px] text-white">
            {{ activeFilterCount }}
          </span>
        </button>
        <span class="hidden text-[11px] text-muted sm:inline">{{ activeSceneNames }}</span>
      </div>
      <div class="flex items-center gap-1 border border-line bg-white/40 p-0.5">
        <button
          class="px-3 py-1.5 text-[11px] transition"
          :class="sort === 'latest' ? 'bg-ink text-white' : 'text-muted hover:text-ink'"
          @click="setSort('latest')"
        >
          最新
        </button>
        <button
          class="px-3 py-1.5 text-[11px] transition"
          :class="sort === 'recommended' ? 'bg-ink text-white' : 'text-muted hover:text-ink'"
          @click="setSort('recommended')"
        >
          为你推荐
        </button>
      </div>
    </div>

    <!-- Active filter summary -->
    <div v-if="contentType || sourceType || range" class="mt-3 flex flex-wrap items-center gap-2">
      <span v-if="contentType" class="tag">{{ contentTypeLabels[contentType] }}</span>
      <span v-if="sourceType" class="tag">{{ sourceTypeLabels[sourceType] }}</span>
      <span v-if="range" class="tag">{{ rangeOptions.find((r) => r.value === range)?.label }}</span>
      <button class="text-[10px] text-muted underline underline-offset-4 hover:text-ink" @click="clearFilters">清除筛选</button>
    </div>

    <!-- List -->
    <div class="mt-4 min-h-[320px]">
      <LoadingBlock v-if="loading" :rows="5" />
      <EmptyState
        v-else-if="error"
        title="内容加载失败"
        :description="error"
        action-label="重新加载"
        @action="load()"
      />
      <EmptyState
        v-else-if="!items.length"
        title="没有找到相关内容"
        description="试试更换场景、缩短关键词，或清除筛选条件。"
        action-label="清除筛选"
        @action="clearFilters"
      />
      <div v-else class="grid border-t border-line md:grid-cols-2 md:gap-x-10">
        <ContentCard v-for="item in items" :key="item.id" :content="item" />
      </div>

      <div v-if="items.length && hasMore" class="mt-8 flex justify-center">
        <button class="btn-ghost !min-h-10 text-xs" :disabled="loadingMore" @click="loadMore">
          <span v-if="loadingMore" class="h-3.5 w-3.5 animate-spin rounded-full border border-muted border-t-transparent" />
          {{ loadingMore ? '加载中' : '加载更多' }}
          <ArrowDown v-if="!loadingMore" class="h-3.5 w-3.5" />
        </button>
      </div>
      <p v-else-if="items.length" class="mt-8 text-center text-[11px] text-muted">已经到底了 · 共 {{ total }} 条</p>
    </div>

    <!-- Filter sheet -->
    <Modal :open="filterOpen" title="筛选内容" description="组合条件，只看此刻相关的部分。" @close="filterOpen = false">
      <div class="space-y-7">
        <div>
          <div class="field-label">内容类型</div>
          <div class="flex flex-wrap gap-2">
            <button
              v-for="(label, key) in contentTypeLabels"
              :key="key"
              class="border px-3 py-1.5 text-[11px] transition"
              :class="draftContentType === key ? 'border-ink bg-ink text-white' : 'border-line bg-white text-muted hover:border-ink/25'"
              @click="draftContentType = draftContentType === key ? '' : key"
            >
              {{ label }}
            </button>
          </div>
        </div>
        <div>
          <div class="field-label">来源类型</div>
          <div class="flex flex-wrap gap-2">
            <button
              v-for="(label, key) in sourceTypeLabels"
              :key="key"
              class="border px-3 py-1.5 text-[11px] transition"
              :class="draftSourceType === key ? 'border-ink bg-ink text-white' : 'border-line bg-white text-muted hover:border-ink/25'"
              @click="draftSourceType = draftSourceType === key ? '' : key"
            >
              {{ label }}
            </button>
          </div>
        </div>
        <div>
          <div class="field-label">发布时间</div>
          <div class="grid grid-cols-2 gap-2 sm:grid-cols-3">
            <button
              v-for="option in rangeOptions"
              :key="option.value"
              class="border px-3 py-2 text-[11px] transition"
              :class="draftRange === option.value ? 'border-ink bg-ink text-white' : 'border-line bg-white text-muted hover:border-ink/25'"
              @click="draftRange = option.value"
            >
              {{ option.label }}
            </button>
          </div>
        </div>
      </div>
      <template #footer>
        <button class="btn-ghost !min-h-9 text-xs" @click="clearFilters">全部清除</button>
        <button class="btn-primary !min-h-9 text-xs" @click="applyFilters">应用筛选</button>
      </template>
    </Modal>
  </div>
</template>
