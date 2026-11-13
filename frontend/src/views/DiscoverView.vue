<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowDown, ArrowLeft, Search, SlidersHorizontal, X } from 'lucide-vue-next'
import { api } from '../api/client'
import type {
  Character,
  CharacterDetail,
  Content,
  ContentFacets,
  Event,
  EventDetail,
  Paged,
  Scene,
  Work,
  WorkDetail,
} from '../types'
import {
  sourceTypeLabels,
  timelineCategories,
  type TimelineCategoriesItem,
} from '../lib/format'
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
const category = ref(queryString('category') || 'all')
const selectedScenes = ref<string[]>(queryString('scene') ? queryString('scene').split(',').filter(Boolean) : [])
const sourceType = ref(queryString('source_type'))
const platform = ref(queryString('platform'))
const range = ref(queryString('range'))
const workId = ref(queryString('work_id'))
const characterId = ref(queryString('character_id'))
const eventId = ref(queryString('event_id'))
const contextWork = ref<Work | null>(null)
const contextCharacter = ref<Character | null>(null)
const contextEvent = ref<Event | null>(null)
const workOptions = ref<Work[]>([])
const eventOptions = ref<Event[]>([])

const scenes = ref<Scene[]>([])
const facets = ref<ContentFacets>({ platforms: [], content_types: [], source_types: [] })
const items = ref<Content[]>([])
const total = ref(0)
const nextCursor = ref('')
const pageSize = 12
const loading = ref(true)
const loadingMore = ref(false)
const error = ref('')
const filterOpen = ref(false)

const draftSourceType = ref('')
const draftPlatform = ref('')
const draftRange = ref('')
const draftScenes = ref<string[]>([])
const draftWorkId = ref('')
const draftEventId = ref('')

const activeCategory = computed<TimelineCategoriesItem>(
  () => timelineCategories.find((c) => c.key === category.value) ?? timelineCategories[0],
)
const hasMore = computed(() => Boolean(nextCursor.value))
const activeFilterCount = computed(
  () =>
    [category.value !== 'all' ? category.value : '', sourceType.value, platform.value, range.value].filter(Boolean).length +
    (selectedScenes.value.length ? 1 : 0) +
    (workId.value || characterId.value ? 1 : 0) +
    (eventId.value ? 1 : 0),
)
const contexts = computed(() => {
  const items: { kind: string; label: string; to: { name: string; params: { id: string } } }[] = []
  if (characterId.value) {
    items.push({
      kind: '角色',
      label: contextCharacter.value
        ? contextCharacter.value.work?.title
          ? `${contextCharacter.value.name} · ${contextCharacter.value.work.title}`
          : contextCharacter.value.name
        : '相关内容',
      to: { name: 'character', params: { id: characterId.value } },
    })
  } else if (workId.value) {
    items.push({
      kind: '作品',
      label: contextWork.value?.title ?? '相关内容',
      to: { name: 'work', params: { id: workId.value } },
    })
  }
  if (eventId.value) {
    items.push({
      kind: '活动',
      label: contextEvent.value?.title ?? '相关内容',
      to: { name: 'event', params: { id: eventId.value } },
    })
  }
  return items
})
const hasContext = computed(() => contexts.value.length > 0)

const rangeOptions = [
  { value: '', label: '不限时间' },
  { value: 'today', label: '今天' },
  { value: '7d', label: '近 7 天' },
  { value: '30d', label: '近 30 天' },
  { value: '90d', label: '近 90 天' },
]

function buildParams(cursor: string): string {
  const params = new URLSearchParams()
  params.set('sort', 'latest')
  params.set('page_size', String(pageSize))
  if (cursor) params.set('cursor', cursor)
  if (keyword.value.trim()) params.set('keyword', keyword.value.trim())
  const types = activeCategory.value.types
  if (types?.length) params.set('content_type', types.join(','))
  const sceneCodes = [...(activeCategory.value.scene ? [activeCategory.value.scene] : []), ...selectedScenes.value]
  if (sceneCodes.length) params.set('scene', [...new Set(sceneCodes)].join(','))
  if (sourceType.value) params.set('source_type', sourceType.value)
  if (platform.value) params.set('source', platform.value)
  if (range.value) params.set('range', range.value)
  if (workId.value) params.set('work_id', workId.value)
  if (characterId.value) params.set('character_id', characterId.value)
  if (eventId.value) params.set('event_id', eventId.value)
  return params.toString()
}

function syncURL() {
  const query: Record<string, string> = {}
  if (keyword.value.trim()) query.keyword = keyword.value.trim()
  if (category.value !== 'all') query.category = category.value
  if (selectedScenes.value.length) query.scene = selectedScenes.value.join(',')
  if (sourceType.value) query.source_type = sourceType.value
  if (platform.value) query.platform = platform.value
  if (range.value) query.range = range.value
  if (workId.value) query.work_id = workId.value
  if (characterId.value) query.character_id = characterId.value
  if (eventId.value) query.event_id = eventId.value
  router.replace({ name: 'discover', query })
}

async function load(reset = true) {
  if (reset) {
    nextCursor.value = ''
    loading.value = true
    error.value = ''
  } else {
    loadingMore.value = true
  }
  try {
    const cursor = reset ? '' : nextCursor.value
    const data = await api.get<Paged<Content>>(`/contents?${buildParams(cursor)}`)
    items.value = reset ? data.items : [...items.value, ...data.items]
    total.value = data.total
    nextCursor.value = data.next_cursor || ''
  } catch (err) {
    error.value = err instanceof Error ? err.message : '内容加载失败'
  } finally {
    loading.value = false
    loadingMore.value = false
  }
}

async function loadMore() {
  if (loadingMore.value || !hasMore.value) return
  await load(false)
}

async function selectCategory(key: string) {
  if (category.value === key) return
  category.value = key
  syncURL()
  await load()
}

function toggleDraftScene(code: string) {
  draftScenes.value = draftScenes.value.includes(code)
    ? draftScenes.value.filter((c) => c !== code)
    : [...draftScenes.value, code]
}

function openFilters() {
  draftSourceType.value = sourceType.value
  draftPlatform.value = platform.value
  draftRange.value = range.value
  draftScenes.value = [...selectedScenes.value]
  draftWorkId.value = workId.value
  draftEventId.value = eventId.value
  filterOpen.value = true
}

async function applyFilters() {
  sourceType.value = draftSourceType.value
  platform.value = draftPlatform.value
  range.value = draftRange.value
  selectedScenes.value = [...draftScenes.value]
  if (draftWorkId.value !== workId.value) {
    workId.value = draftWorkId.value
    characterId.value = ''
    contextWork.value = null
    contextCharacter.value = null
  }
  eventId.value = draftEventId.value
  filterOpen.value = false
  syncURL()
  await load()
  await loadContext()
}

async function clearFilters() {
  category.value = 'all'
  sourceType.value = ''
  platform.value = ''
  range.value = ''
  selectedScenes.value = []
  workId.value = ''
  characterId.value = ''
  eventId.value = ''
  contextWork.value = null
  contextCharacter.value = null
  contextEvent.value = null
  filterOpen.value = false
  syncURL()
  await load()
}

async function clearContext() {
  workId.value = ''
  characterId.value = ''
  eventId.value = ''
  contextWork.value = null
  contextCharacter.value = null
  contextEvent.value = null
  syncURL()
  await load()
}

async function loadContext() {
  if (characterId.value && contextCharacter.value?.id !== characterId.value) {
    try {
      const data = await api.get<CharacterDetail>(`/characters/${characterId.value}`)
      contextCharacter.value = data.character
    } catch {
      contextCharacter.value = null
    }
  }
  if (workId.value && !characterId.value && contextWork.value?.id !== workId.value) {
    try {
      const data = await api.get<WorkDetail>(`/works/${workId.value}`)
      contextWork.value = data.work
    } catch {
      contextWork.value = null
    }
  }
  if (eventId.value && contextEvent.value?.id !== eventId.value) {
    try {
      const data = await api.get<EventDetail>(`/events/${eventId.value}`)
      contextEvent.value = data.event
    } catch {
      contextEvent.value = null
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

// The global search dialog can navigate to /discover?keyword=... while this
// view is already mounted. Keep the input in sync with the route in that case.
watch(
  () => route.query.keyword,
  (value) => {
    const next = typeof value === 'string' ? value : ''
    if (next !== keyword.value) {
      clearTimeout(searchTimer)
      keyword.value = next
    }
  },
)

onMounted(async () => {
  load()
  loadContext()
  try {
    const res = await api.get<{ items: Scene[] }>('/scenes')
    scenes.value = res.items
  } catch {
    scenes.value = []
  }
  try {
    facets.value = await api.get<ContentFacets>('/contents/facets')
  } catch {
    facets.value = { platforms: [], content_types: [], source_types: [] }
  }
  try {
    const res = await api.get<{ items: Work[] }>('/works')
    workOptions.value = res.items
  } catch {
    workOptions.value = []
  }
  try {
    const res = await api.get<{ items: Event[] }>('/events')
    eventOptions.value = res.items
  } catch {
    eventOptions.value = []
  }
})
</script>

<template>
  <div class="shell pb-16 pt-8 md:pb-24 md:pt-12">
    <header class="mb-7 md:mb-10">
      <div class="eyebrow mb-3">Discover / Timeline</div>
      <div class="flex flex-col gap-4 md:flex-row md:items-end md:justify-between">
        <div>
          <h1 class="display-title text-[36px] md:text-[48px]">发现</h1>
          <p class="mt-2 max-w-xl text-sm leading-6 text-muted">
            按原始发布时间排列的全部公开动态。时间不明的内容会标注发现时间，不伪造发布时间。
          </p>
        </div>
        <div class="flex items-center gap-2 text-[11px] text-muted">
          <span class="h-1 w-1 rounded-full bg-moss" />
          经核验后公开 · 共 {{ total }} 条
        </div>
      </div>
    </header>

    <div class="relative">
      <Search class="pointer-events-none absolute left-4 top-1/2 h-4 w-4 -translate-y-1/2 text-muted" />
      <input
        v-model="keyword"
        type="search"
        class="field !py-3.5 pl-11 pr-11 text-[13px]"
        placeholder="搜索标题、来源或关键词"
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

    <div
      v-if="hasContext"
      class="mt-4 flex flex-wrap items-center justify-between gap-3 border border-line bg-blush/25 px-4 py-3"
    >
      <div class="min-w-0 space-y-2">
        <p class="text-[9px] uppercase tracking-widest text-muted">正在查看相关内容</p>
        <div class="flex flex-wrap items-center gap-x-4 gap-y-2">
          <RouterLink
            v-for="ctx in contexts"
            :key="`${ctx.kind}-${ctx.to.params.id}`"
            :to="ctx.to"
            class="group inline-flex min-w-0 items-center gap-2"
          >
            <span class="shrink-0 border border-accent/30 px-2 py-0.5 text-[9px] tracking-widest text-accent">{{ ctx.kind }}</span>
            <span class="truncate font-display text-base leading-6 transition group-hover:text-accent">{{ ctx.label }}</span>
            <ArrowLeft class="h-3 w-3 shrink-0 text-muted transition group-hover:text-accent" />
          </RouterLink>
        </div>
      </div>
      <button class="shrink-0 text-[10px] text-muted underline underline-offset-4 hover:text-ink" @click="clearContext">
        查看全部
      </button>
    </div>

    <div class="no-scrollbar -mx-5 mt-5 flex gap-2 overflow-x-auto px-5 pb-1 md:mx-0 md:flex-wrap md:px-0">
      <button
        v-for="cat in timelineCategories"
        :key="cat.key"
        class="shrink-0 border px-3.5 py-2 text-[11px] transition"
        :class="category === cat.key
          ? 'border-ink bg-ink text-white'
          : 'border-line bg-white/50 text-muted hover:border-ink/25 hover:text-ink'"
        @click="selectCategory(cat.key)"
      >
        {{ cat.label }}
      </button>
    </div>

    <div class="mt-6 flex flex-wrap items-center justify-between gap-3 border-y border-line py-3">
      <div class="flex items-center gap-2">
        <button class="btn-ghost !min-h-9 !border-line !px-3 !py-1.5 text-[11px]" @click="openFilters">
          <SlidersHorizontal class="h-3.5 w-3.5" />
          筛选
          <span v-if="activeFilterCount" class="flex h-4 min-w-4 items-center justify-center rounded-full bg-accent px-1 text-[9px] text-white">
            {{ activeFilterCount }}
          </span>
        </button>
        <span class="text-[11px] text-muted">{{ activeCategory.label }} · 按时间倒序</span>
      </div>
      <span class="text-[10px] text-muted">不按热度排序</span>
    </div>

    <div v-if="category !== 'all' || sourceType || platform || range || selectedScenes.length" class="mt-3 flex flex-wrap items-center gap-2">
      <span v-if="category !== 'all'" class="tag">{{ activeCategory.label }}</span>
      <span v-if="platform" class="tag">{{ platform }}</span>
      <span v-if="sourceType" class="tag">{{ sourceTypeLabels[sourceType] || sourceType }}</span>
      <span v-if="range" class="tag">{{ rangeOptions.find((r) => r.value === range)?.label }}</span>
      <span v-for="code in selectedScenes" :key="code" class="tag">
        {{ scenes.find((s) => s.code === code)?.name || code }}
      </span>
      <button class="text-[10px] text-muted underline underline-offset-4 hover:text-ink" @click="clearFilters">清除筛选</button>
    </div>

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
        description="试试更换分类、缩短关键词，或清除筛选条件。"
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

    <Modal :open="filterOpen" title="筛选内容" description="按来源平台、时间、作品与活动组合查看。" @close="filterOpen = false">
      <div class="space-y-7">
        <div>
          <div class="field-label">来源平台</div>
          <div class="flex flex-wrap gap-2">
            <button
              class="border px-3 py-1.5 text-[11px] transition"
              :class="!draftPlatform ? 'border-ink bg-ink text-white' : 'border-line bg-white text-muted hover:border-ink/25'"
              @click="draftPlatform = ''"
            >
              全部平台
            </button>
            <button
              v-for="p in facets.platforms"
              :key="p.value"
              class="border px-3 py-1.5 text-[11px] transition"
              :class="draftPlatform === p.value ? 'border-ink bg-ink text-white' : 'border-line bg-white text-muted hover:border-ink/25'"
              @click="draftPlatform = draftPlatform === p.value ? '' : p.value"
            >
              {{ p.value }} · {{ p.count }}
            </button>
          </div>
          <p v-if="!facets.platforms.length" class="mt-2 text-[10px] text-muted">暂无平台数据</p>
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
          <div class="field-label">时间范围</div>
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
        <div class="grid gap-4 sm:grid-cols-2">
          <div>
            <label for="filter-work" class="field-label">关联作品</label>
            <select id="filter-work" v-model="draftWorkId" class="select-field">
              <option value="">全部作品</option>
              <option v-for="work in workOptions" :key="work.id" :value="work.id">{{ work.title }}</option>
            </select>
          </div>
          <div>
            <label for="filter-event" class="field-label">关联活动</label>
            <select id="filter-event" v-model="draftEventId" class="select-field">
              <option value="">全部活动</option>
              <option v-for="event in eventOptions" :key="event.id" :value="event.id">{{ event.title }}</option>
            </select>
          </div>
        </div>
        <p class="text-[10px] leading-5 text-muted">选择作品会替换当前角色上下文；活动可以与作品同时筛选。</p>
        <div>
          <div class="field-label">饭圈场景</div>
          <div class="flex flex-wrap gap-2">
            <button
              v-for="scene in scenes.filter((s) => s.code !== 'all')"
              :key="scene.id"
              class="inline-flex items-center gap-1.5 border px-3 py-1.5 text-[11px] transition"
              :class="draftScenes.includes(scene.code)
                ? 'border-accent/50 bg-blush/50 text-ink'
                : 'border-line bg-white text-muted hover:border-ink/25'"
              @click="toggleDraftScene(scene.code)"
            >
              <SceneIcon :code="scene.code" class="h-3.5 w-3.5" />
              {{ scene.name }}
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
