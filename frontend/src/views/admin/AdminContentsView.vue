<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import {
  ArrowUpRight,
  ChevronLeft,
  ChevronRight,
  ExternalLink,
  FilePlus2,
  Pencil,
  RefreshCw,
  Search,
  X,
} from 'lucide-vue-next'
import { api } from '../../api/client'
import type { Character, Content, Event, Paged, Scene, Source, Work } from '../../types'
import {
  contentStatusLabels,
  contentTypeLabels,
  filterReasonLabels,
  formatDate,
  formatDateTime,
  precisionLabels,
  relevanceLabels,
  reviewStatusLabels,
  sourceTypeLabels,
  trustLabels,
} from '../../lib/format'
import { toast } from '../../stores/toast'
import AdminPageHeader from './AdminPageHeader.vue'
import EmptyState from '../../components/EmptyState.vue'
import LoadingBlock from '../../components/LoadingBlock.vue'
import Modal from '../../components/Modal.vue'

const items = ref<Content[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 20
const loading = ref(true)
const error = ref('')
const saving = ref(false)
const modalOpen = ref(false)
const editingId = ref('')

const keyword = ref('')
const contentStatus = ref('')
const reviewStatus = ref('')
const contentType = ref('')
const candidateOnly = ref(false)
const metadataStatus = ref('')
const relevanceLabel = ref('')
const filterReason = ref('')
const editRaw = ref<unknown>(null)

const sources = ref<Source[]>([])
const scenes = ref<Scene[]>([])
const works = ref<Work[]>([])
const characters = ref<Character[]>([])
const events = ref<Event[]>([])

const statusOptions = Object.entries(contentStatusLabels)
const reviewOptions = Object.entries(reviewStatusLabels)
const typeOptions = Object.entries(contentTypeLabels)
const trustOptions = Object.entries(trustLabels)
const precisionOptions = Object.entries(precisionLabels)
const metadataOptions = [
  { value: 'not_attempted', label: '未尝试解析' },
  { value: 'ok', label: '解析成功' },
  { value: 'failed', label: '解析失败' },
  { value: 'blocked', label: '被来源拦截' },
]
const relevanceOptions = Object.entries(relevanceLabels)
const filterReasonOptions = Object.entries(filterReasonLabels)
const contentStatusKeys = ['pending', 'reviewing', 'published', 'rejected', 'expired', 'archived']
const reviewStatusKeys = ['none', 'pending', 'approved', 'rejected']

const totalPages = computed(() => Math.max(1, Math.ceil(total.value / pageSize)))
const workTitleMap = computed(() => Object.fromEntries(works.value.map((w) => [w.id, w.title])))
const rawPreview = computed(() => {
  if (!editRaw.value) return ''
  try {
    return JSON.stringify(editRaw.value, null, 2)
  } catch {
    return String(editRaw.value)
  }
})

type ManualForm = {
  title: string
  summary: string
  content_type: string
  source_id: string
  source_author: string
  source_url: string
  cover_url: string
  published_at: string
  published_precision: string
  clear_published_at: boolean
  trust_level: string
  is_official: boolean
  scene_ids: string[]
  work_ids: string[]
  character_ids: string[]
  event_ids: string[]
  content_status: string
  review_status: string
}

function toLocalInput(value?: string | null): string {
  if (!value) return ''
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return ''
  const offset = d.getTimezoneOffset() * 60000
  return new Date(d.getTime() - offset).toISOString().slice(0, 16)
}

function nowLocalInput(): string {
  const now = new Date()
  const offset = now.getTimezoneOffset() * 60000
  return new Date(now.getTime() - offset).toISOString().slice(0, 16)
}

function emptyForm(): ManualForm {
  return {
    title: '',
    summary: '',
    content_type: 'internal',
    source_id: '',
    source_author: '',
    source_url: '',
    cover_url: '',
    published_at: nowLocalInput(),
    published_precision: 'verified',
    clear_published_at: false,
    trust_level: 'internal',
    is_official: false,
    scene_ids: [],
    work_ids: [],
    character_ids: [],
    event_ids: [],
    content_status: 'published',
    review_status: 'approved',
  }
}

const form = reactive<ManualForm>(emptyForm())

function toggleId(list: string[], id: string): string[] {
  return list.includes(id) ? list.filter((x) => x !== id) : [...list, id]
}

function openCreate() {
  editingId.value = ''
  Object.assign(form, emptyForm())
  modalOpen.value = true
}

async function openEdit(item: Content) {
  editingId.value = item.id
  editRaw.value = null
  Object.assign(form, emptyForm(), {
    title: item.title,
    summary: item.summary,
    content_type: item.content_type,
    source_url: item.source_url,
    cover_url: item.cover_url,
    published_at: toLocalInput(item.published_at),
    published_precision: item.published_precision || 'unknown',
    clear_published_at: false,
    trust_level: item.trust_level,
    is_official: item.is_official,
    scene_ids: item.scenes?.map((s) => s.id) || [],
    work_ids: item.works?.map((w) => w.id) || [],
    character_ids: item.characters?.map((c) => c.id) || [],
    event_ids: item.events?.map((e) => e.id) || [],
    content_status: item.content_status,
    review_status: item.review_status,
  })
  modalOpen.value = true
  try {
    const full = await api.get<Content>(`/admin/contents/${item.id}`)
    form.scene_ids = full.scenes?.map((s) => s.id) || []
    form.work_ids = full.works?.map((w) => w.id) || []
    form.character_ids = full.characters?.map((c) => c.id) || []
    form.event_ids = full.events?.map((e) => e.id) || []
    form.published_at = toLocalInput(full.published_at)
    form.published_precision = full.published_precision || 'unknown'
    editRaw.value = full.raw_data ?? null
  } catch {
    /* keep the list-provided relations */
  }
}

function buildParams(nextPage: number): string {
  const params = new URLSearchParams()
  params.set('page', String(nextPage))
  params.set('page_size', String(pageSize))
  if (keyword.value.trim()) params.set('keyword', keyword.value.trim())
  if (contentStatus.value) params.set('content_status', contentStatus.value)
  if (reviewStatus.value) params.set('review_status', reviewStatus.value)
  if (contentType.value) params.set('content_type', contentType.value)
  if (candidateOnly.value) params.set('candidate', 'true')
  if (metadataStatus.value) params.set('metadata_status', metadataStatus.value)
  if (relevanceLabel.value) params.set('relevance_label', relevanceLabel.value)
  if (filterReason.value) params.set('filter_reason', filterReason.value)
  return params.toString()
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await api.get<Paged<Content>>(`/admin/contents?${buildParams(page.value)}`)
    items.value = data.items
    total.value = data.total
  } catch (err) {
    error.value = err instanceof Error ? err.message : '内容列表加载失败'
  } finally {
    loading.value = false
  }
}

function applyFilters() {
  page.value = 1
  load()
}

function resetFilters() {
  keyword.value = ''
  contentStatus.value = ''
  reviewStatus.value = ''
  contentType.value = ''
  candidateOnly.value = false
  metadataStatus.value = ''
  relevanceLabel.value = ''
  filterReason.value = ''
  applyFilters()
}

function goPage(next: number) {
  if (next < 1 || next > totalPages.value || loading.value) return
  page.value = next
  load()
}

async function save() {
  if (saving.value) return
  if (!form.title.trim()) {
    toast.error('标题不能为空')
    return
  }
  saving.value = true
  try {
    if (editingId.value) {
      await api.patch(`/admin/contents/${editingId.value}`, {
        title: form.title.trim(),
        summary: form.summary,
        content_type: form.content_type,
        source_url: form.source_url,
        cover_url: form.cover_url,
        is_official: form.is_official,
        trust_level: form.trust_level,
        content_status: form.content_status,
        review_status: form.review_status,
        scene_ids: form.scene_ids,
        work_ids: form.work_ids,
        character_ids: form.character_ids,
        event_ids: form.event_ids,
        ...(form.clear_published_at
          ? { clear_published_at: true }
          : form.published_at
            ? {
                published_at: new Date(form.published_at).toISOString(),
                published_precision: form.published_precision,
              }
            : {}),
      })
      toast.success('内容已更新')
    } else {
      await api.post('/admin/contents', {
        title: form.title.trim(),
        summary: form.summary,
        content_type: form.content_type,
        source_id: form.source_id,
        source_author: form.source_author,
        source_url: form.source_url,
        cover_url: form.cover_url,
        published_at: new Date(form.published_at || Date.now()).toISOString(),
        trust_level: form.trust_level,
        is_official: form.is_official,
        scene_ids: form.scene_ids,
        work_ids: form.work_ids,
        character_ids: form.character_ids,
        event_ids: form.event_ids,
      })
      toast.success('内容已创建并公开')
    }
    modalOpen.value = false
    await load()
  } catch (err) {
    toast.error(err instanceof Error ? err.message : '保存失败')
  } finally {
    saving.value = false
  }
}

function statusTone(status: string): string {
  switch (status) {
    case 'published':
      return 'border-moss/30 bg-moss/10 text-moss'
    case 'pending':
    case 'reviewing':
      return 'border-[#b99a5e]/30 bg-[#f3ead7] text-[#8a6a32]'
    case 'rejected':
      return 'border-accent/30 bg-blush/40 text-accent'
    default:
      return 'border-line bg-white text-muted'
  }
}

function relevanceTone(label: string): string {
  switch (label) {
    case 'high':
      return 'border-moss/30 bg-moss/10 text-moss'
    case 'candidate':
      return 'border-[#b99a5e]/30 bg-[#f3ead7] text-[#8a6a32]'
    case 'filtered':
      return 'border-accent/30 bg-blush/40 text-accent'
    default:
      return 'border-line bg-white text-muted'
  }
}

let keywordTimer: ReturnType<typeof setTimeout> | undefined
watch(keyword, () => {
  clearTimeout(keywordTimer)
  keywordTimer = setTimeout(applyFilters, 380)
})

onMounted(async () => {
  load()
  try {
    const [sourceRes, sceneRes, workRes, charRes, eventRes] = await Promise.all([
      api.get<{ items: Source[] }>('/admin/sources'),
      api.get<{ items: Scene[] }>('/admin/scenes'),
      api.get<{ items: Work[] }>('/admin/works'),
      api.get<{ items: Character[] }>('/admin/characters'),
      api.get<{ items: Event[] }>('/admin/events'),
    ])
    sources.value = sourceRes.items
    scenes.value = sceneRes.items
    works.value = workRes.items
    characters.value = charRes.items
    events.value = eventRes.items
  } catch {
    /* metadata is only needed when opening the form */
  }
})
</script>

<template>
  <div>
    <AdminPageHeader
      eyebrow="Console / Contents"
      title="内容管理"
      description="内容入库、编辑与发布状态管理。手动录入仅用于已核验的公开信息，需保留来源链接。"
    >
      <template #actions>
        <button class="btn-ghost !min-h-9 !px-3 text-[11px]" :disabled="loading" @click="load">
          <RefreshCw class="h-3.5 w-3.5" :class="loading ? 'animate-spin' : ''" />
          刷新
        </button>
        <button class="btn-primary !min-h-9 !px-3 text-[11px]" @click="openCreate">
          <FilePlus2 class="h-3.5 w-3.5" />
          手动录入
        </button>
      </template>
    </AdminPageHeader>

    <!-- Filters -->
    <div class="mb-5 border border-line bg-canvas p-3 md:p-4">
      <div class="grid gap-3 md:grid-cols-2 xl:grid-cols-[minmax(180px,1fr)_repeat(6,auto)]">
        <div class="relative">
          <Search class="pointer-events-none absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted" />
          <input v-model="keyword" class="field !py-2.5 pl-9 text-xs" placeholder="搜索标题、摘要或来源" />
        </div>
        <select v-model="contentStatus" class="select-field !py-2.5 text-xs" @change="applyFilters">
          <option value="">全部发布状态</option>
          <option v-for="[key, label] in statusOptions" :key="key" :value="key">{{ label }}</option>
        </select>
        <select v-model="reviewStatus" class="select-field !py-2.5 text-xs" @change="applyFilters">
          <option value="">全部审核状态</option>
          <option v-for="[key, label] in reviewOptions" :key="key" :value="key">{{ label }}</option>
        </select>
        <select v-model="contentType" class="select-field !py-2.5 text-xs" @change="applyFilters">
          <option value="">全部内容类型</option>
          <option v-for="[key, label] in typeOptions" :key="key" :value="key">{{ label }}</option>
        </select>
        <select v-model="metadataStatus" class="select-field !py-2.5 text-xs" @change="applyFilters">
          <option value="">全部解析状态</option>
          <option v-for="option in metadataOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
        </select>
        <select v-model="relevanceLabel" class="select-field !py-2.5 text-xs" @change="applyFilters">
          <option value="">全部相关度</option>
          <option v-for="[key, label] in relevanceOptions" :key="key" :value="key">{{ label }}</option>
        </select>
        <select v-model="filterReason" class="select-field !py-2.5 text-xs" @change="applyFilters">
          <option value="">全部过滤原因</option>
          <option v-for="[key, label] in filterReasonOptions" :key="key" :value="key">{{ label }}</option>
        </select>
      </div>
      <div class="mt-2.5 flex flex-wrap items-center justify-between gap-2 text-[10px] text-muted">
        <div class="flex items-center gap-3">
          <span>共 {{ total }} 条 · 第 {{ page }} / {{ totalPages }} 页</span>
          <label class="inline-flex cursor-pointer items-center gap-1.5">
            <input v-model="candidateOnly" type="checkbox" class="h-3.5 w-3.5 accent-[#c98791]" @change="applyFilters" />
            仅搜索候选
          </label>
        </div>
        <button
          v-if="keyword || contentStatus || reviewStatus || contentType || candidateOnly || metadataStatus || relevanceLabel || filterReason"
          class="inline-flex items-center gap-1 hover:text-ink"
          @click="resetFilters"
        >
          <X class="h-3 w-3" />清除筛选
        </button>
      </div>
    </div>

    <LoadingBlock v-if="loading" :rows="6" />

    <EmptyState v-else-if="error" title="内容列表加载失败" :description="error" action-label="重新加载" @action="load" />

    <EmptyState
      v-else-if="!items.length"
      title="没有匹配的内容"
      description="调整筛选条件，或手动录入一条已核验的公开内容。"
      action-label="清除筛选"
      @action="resetFilters"
    />

    <div v-else>
      <!-- Desktop header -->
      <div class="hidden grid-cols-[1fr_120px_110px_90px_80px] gap-4 border-b border-line px-3 py-2 text-[9px] uppercase tracking-widest text-muted lg:grid">
        <span>内容</span>
        <span>发布 / 审核</span>
        <span>来源</span>
        <span>时间</span>
        <span class="text-right">操作</span>
      </div>
      <div class="divide-y divide-line border-b border-line">
        <article
          v-for="item in items"
          :key="item.id"
          class="group grid gap-3 px-1 py-4 transition hover:bg-white/40 md:px-3 lg:grid-cols-[1fr_120px_110px_90px_80px] lg:items-center lg:gap-4"
        >
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-1.5 text-[9px] text-muted">
              <span class="uppercase tracking-widest">{{ contentTypeLabels[item.content_type] || item.content_type }}</span>
              <span class="h-2.5 w-px bg-line" />
              <span>{{ trustLabels[item.trust_level] || item.trust_level }}</span>
              <span v-if="item.is_official" class="text-[#9a6c72]">官方</span>
              <span v-if="item.search_task_id" class="border border-accent/25 bg-blush/30 px-1 py-0.5 text-[8px] text-[#9a6c72]">
                搜索候选
              </span>
              <span
                v-if="item.relevance_label"
                class="border px-1 py-0.5 text-[8px]"
                :class="relevanceTone(item.relevance_label)"
              >
                {{ relevanceLabels[item.relevance_label] }} · {{ item.relevance_score }}
              </span>
              <span v-if="item.filter_reason" class="text-[8px] text-[#9a6c72]">
                {{ filterReasonLabels[item.filter_reason] || item.filter_reason }}
              </span>
              <span
                v-if="item.metadata_status !== 'not_attempted'"
                class="text-[8px]"
                :class="item.metadata_status === 'ok' ? 'text-moss' : 'text-[#9a6c72]'"
              >
                解析{{ item.metadata_status === 'ok' ? '成功' : item.metadata_status === 'blocked' ? '被拦截' : '失败' }}
              </span>
            </div>
            <h2 class="mt-1 line-clamp-1 font-display text-[16px] tracking-tight">{{ item.title }}</h2>
            <p v-if="item.summary" class="mt-0.5 line-clamp-1 text-[10px] text-muted">{{ item.summary }}</p>
            <div class="mt-1.5 flex flex-wrap gap-1">
              <span v-for="scene in item.scenes?.slice(0, 2)" :key="scene.id" class="text-[9px] text-muted">{{ scene.name }}</span>
              <span v-if="item.works?.length" class="text-[9px] text-muted">· {{ item.works.map((w) => w.title).join(' / ') }}</span>
              <span v-if="item.events?.length" class="text-[9px] text-[#9a6c72]">· {{ item.events.map((e) => e.title).join(' / ') }}</span>
            </div>
            <p v-if="item.metadata_error" class="mt-1 line-clamp-1 text-[9px] text-[#9a6c72]">{{ item.metadata_error }}</p>
            <p v-if="item.filter_reason && item.review_reason" class="mt-1 line-clamp-1 text-[9px] text-[#9a6c72]">
              {{ item.review_reason }}
            </p>
          </div>

          <div class="flex flex-wrap items-center gap-1.5">
            <span class="border px-1.5 py-0.5 text-[9px]" :class="statusTone(item.content_status)">
              {{ contentStatusLabels[item.content_status] || item.content_status }}
            </span>
            <span
              class="border px-1.5 py-0.5 text-[9px]"
              :class="item.review_status === 'pending' ? 'border-[#b99a5e]/30 bg-[#f3ead7] text-[#8a6a32]' : 'border-line bg-white text-muted'"
            >
              {{ reviewStatusLabels[item.review_status] || item.review_status }}
            </span>
          </div>

          <div class="min-w-0 text-[10px] text-muted">
            <div class="truncate">{{ item.source_name || '站内整理' }}</div>
            <div class="mt-0.5 text-[9px]">
              {{ item.origin_platform || sourceTypeLabels[item.origin_type || item.source_type] || item.source_type || '—' }}
            </div>
          </div>

          <div class="text-[10px] text-muted">
            <div>{{ item.published_at ? formatDate(item.published_at) : '时间未知' }}</div>
            <div class="mt-0.5 text-[9px]">发现 {{ formatDate(item.discovered_at) }}</div>
            <div v-if="item.published_precision !== 'verified'" class="mt-0.5 text-[9px] text-[#9a6c72]">
              {{ precisionLabels[item.published_precision] }}
            </div>
          </div>

          <div class="flex items-center gap-1 lg:justify-end">
            <a
              v-if="item.source_url"
              :href="item.source_url"
              target="_blank"
              rel="noopener noreferrer nofollow"
              class="flex h-7 w-7 items-center justify-center text-muted transition hover:text-ink"
              title="查看原文"
            >
              <ExternalLink class="h-3.5 w-3.5" />
            </a>
            <RouterLink
              :to="{ name: 'content', params: { id: item.id } }"
              target="_blank"
              class="flex h-7 w-7 items-center justify-center text-muted transition hover:text-ink"
              title="预览站内页面"
            >
              <ArrowUpRight class="h-3.5 w-3.5" />
            </RouterLink>
            <button class="flex h-7 w-7 items-center justify-center text-muted transition hover:text-ink" title="编辑" @click="openEdit(item)">
              <Pencil class="h-3.5 w-3.5" />
            </button>
          </div>
        </article>
      </div>

      <div class="mt-5 flex items-center justify-between">
        <p class="text-[10px] text-muted">每页 {{ pageSize }} 条 · 共 {{ total }} 条</p>
        <div class="flex items-center gap-1">
          <button class="btn-icon !h-8 !w-8" :disabled="page <= 1" aria-label="上一页" @click="goPage(page - 1)">
            <ChevronLeft class="h-3.5 w-3.5" />
          </button>
          <span class="px-2 text-[10px] text-muted">{{ page }} / {{ totalPages }}</span>
          <button class="btn-icon !h-8 !w-8" :disabled="page >= totalPages" aria-label="下一页" @click="goPage(page + 1)">
            <ChevronRight class="h-3.5 w-3.5" />
          </button>
        </div>
      </div>
    </div>

    <!-- Create / edit -->
    <Modal
      :open="modalOpen"
      :title="editingId ? '编辑内容' : '手动录入内容'"
      :description="editingId ? '修改会写入审核日志。' : '手动录入内容将直接发布，请确认来源真实、链接可访问。'"
      size="xl"
      @close="modalOpen = false"
    >
      <div class="grid gap-5 md:grid-cols-2">
        <div class="md:col-span-2">
          <label for="content-title" class="field-label">标题 *</label>
          <input id="content-title" v-model="form.title" class="field" placeholder="内容标题" />
        </div>
        <div class="md:col-span-2">
          <label for="content-summary" class="field-label">摘要</label>
          <textarea id="content-summary" v-model="form.summary" rows="4" class="field resize-y" placeholder="仅保留摘要，不复制全文" />
        </div>
        <div>
          <label for="content-type" class="field-label">内容类型</label>
          <select id="content-type" v-model="form.content_type" class="select-field">
            <option v-for="[key, label] in typeOptions" :key="key" :value="key">{{ label }}</option>
          </select>
        </div>
        <div>
          <label for="content-trust" class="field-label">可信等级</label>
          <select id="content-trust" v-model="form.trust_level" class="select-field">
            <option v-for="[key, label] in trustOptions" :key="key" :value="key">{{ label }}</option>
          </select>
        </div>

        <template v-if="!editingId">
          <div>
            <label for="content-source" class="field-label">关联数据源</label>
            <select id="content-source" v-model="form.source_id" class="select-field">
              <option value="">不关联来源（站内整理）</option>
              <option v-for="source in sources" :key="source.id" :value="source.id">
                {{ source.name }} · {{ sourceTypeLabels[source.source_type] }}
              </option>
            </select>
          </div>
          <div>
            <label for="content-author" class="field-label">来源作者 / 账号</label>
            <input id="content-author" v-model="form.source_author" class="field" placeholder="留空则使用数据源名称或编辑部" />
          </div>
          <div>
            <label for="content-published" class="field-label">发布时间</label>
            <input id="content-published" v-model="form.published_at" type="datetime-local" class="field" />
          </div>
          <div class="flex items-end pb-2">
            <label class="flex cursor-pointer items-center gap-2.5 text-xs">
              <input v-model="form.is_official" type="checkbox" class="h-4 w-4 accent-[#c98791]" />
              标记为官方来源
            </label>
          </div>
        </template>

        <template v-else>
          <div>
            <label for="content-status" class="field-label">发布状态</label>
            <select id="content-status" v-model="form.content_status" class="select-field">
              <option v-for="key in contentStatusKeys" :key="key" :value="key">{{ contentStatusLabels[key] }}</option>
            </select>
          </div>
          <div>
            <label for="content-review" class="field-label">审核状态</label>
            <select id="content-review" v-model="form.review_status" class="select-field">
              <option v-for="key in reviewStatusKeys" :key="key" :value="key">{{ reviewStatusLabels[key] }}</option>
            </select>
          </div>
          <div class="md:col-span-2">
            <div class="grid gap-3 border border-line bg-white/40 p-3 sm:grid-cols-2">
              <div>
                <label for="content-published-edit" class="field-label">原始发布时间</label>
                <input
                  id="content-published-edit"
                  v-model="form.published_at"
                  type="datetime-local"
                  class="field"
                  :disabled="form.clear_published_at"
                />
              </div>
              <div>
                <label for="content-precision" class="field-label">时间精确度</label>
                <select id="content-precision" v-model="form.published_precision" class="select-field" :disabled="form.clear_published_at">
                  <option v-for="[key, label] in precisionOptions" :key="key" :value="key">{{ label }}</option>
                </select>
              </div>
              <label class="flex cursor-pointer items-center gap-2 text-[11px] text-muted sm:col-span-2">
                <input v-model="form.clear_published_at" type="checkbox" class="h-3.5 w-3.5 accent-[#c98791]" />
                来源没有可靠发布时间：清空发布时间，改按发现时间排序
              </label>
              <p class="text-[10px] leading-5 text-muted sm:col-span-2">
                仅在来源明确给出时间时填写。搜索摘要中的日期只能标为「时间待核验」，不得当作已核验时间。
              </p>
            </div>
          </div>
          <div class="flex items-end pb-2">
            <label class="flex cursor-pointer items-center gap-2.5 text-xs">
              <input v-model="form.is_official" type="checkbox" class="h-4 w-4 accent-[#c98791]" />
              标记为官方来源
            </label>
          </div>
        </template>

        <div class="md:col-span-2">
          <label for="content-url" class="field-label">原文链接</label>
          <input id="content-url" v-model="form.source_url" class="field" placeholder="https://...（必须可公开访问）" />
          <p class="mt-1 text-[10px] text-muted">相同链接的内容不可重复录入，用于去重。</p>
        </div>
        <div class="md:col-span-2">
          <label for="content-cover" class="field-label">封面地址（可选，需已获授权）</label>
          <input id="content-cover" v-model="form.cover_url" class="field" placeholder="留空则使用站内抽象封面" />
        </div>

        <div class="md:col-span-2">
          <div class="field-label">关联场景</div>
          <div class="flex flex-wrap gap-1.5">
            <button
              v-for="scene in scenes"
              :key="scene.id"
              class="border px-2.5 py-1 text-[10px] transition"
              :class="form.scene_ids.includes(scene.id) ? 'border-accent/50 bg-blush/50 text-ink' : 'border-line bg-white text-muted hover:border-ink/25'"
              @click="form.scene_ids = toggleId(form.scene_ids, scene.id)"
            >
              {{ scene.name }}
            </button>
          </div>
        </div>

        <div class="md:col-span-2">
          <div class="field-label">关联作品</div>
          <div class="flex max-h-28 flex-wrap gap-1.5 overflow-y-auto">
            <button
              v-for="work in works"
              :key="work.id"
              class="border px-2.5 py-1 text-[10px] transition"
              :class="form.work_ids.includes(work.id) ? 'border-ink bg-ink text-white' : 'border-line bg-white text-muted hover:border-ink/25'"
              @click="form.work_ids = toggleId(form.work_ids, work.id)"
            >
              {{ work.title }}
            </button>
          </div>
        </div>

        <div class="md:col-span-2">
          <div class="field-label">关联事件</div>
          <div class="flex max-h-28 flex-wrap gap-1.5 overflow-y-auto">
            <button
              v-for="event in events"
              :key="event.id"
              class="border px-2.5 py-1 text-[10px] transition"
              :class="form.event_ids.includes(event.id) ? 'border-accent/60 bg-blush/50 text-ink' : 'border-line bg-white text-muted hover:border-ink/25'"
              @click="form.event_ids = toggleId(form.event_ids, event.id)"
            >
              {{ event.title }}
              <span class="opacity-50">{{ event.event_date ? `· ${event.event_date}` : '' }}</span>
            </button>
            <p v-if="!events.length" class="text-[10px] text-muted">暂无事件，可先在「内容发现」中创建。</p>
          </div>
        </div>

        <div class="md:col-span-2">
          <div class="field-label">关联角色</div>
          <div class="flex max-h-32 flex-wrap gap-1.5 overflow-y-auto">
            <button
              v-for="character in characters"
              :key="character.id"
              class="border px-2.5 py-1 text-[10px] transition"
              :class="form.character_ids.includes(character.id) ? 'border-ink bg-ink text-white' : 'border-line bg-white text-muted hover:border-ink/25'"
              @click="form.character_ids = toggleId(form.character_ids, character.id)"
            >
              {{ character.name }}
              <span class="opacity-50">{{ workTitleMap[character.work_id] ? `· ${workTitleMap[character.work_id]}` : '' }}</span>
            </button>
          </div>
        </div>
        <div v-if="editingId && rawPreview" class="md:col-span-2">
          <details class="border border-line bg-[#fbf8f3]">
            <summary class="cursor-pointer px-3 py-2 text-[10px] text-muted">采集原始数据与解析结果（仅后台可见）</summary>
            <pre class="max-h-64 overflow-auto border-t border-line px-3 py-2 text-[10px] leading-5 text-ink/70">{{ rawPreview }}</pre>
          </details>
        </div>
      </div>

      <template #footer>
        <span class="mr-auto hidden text-[10px] text-muted sm:inline">
          {{ editingId ? `发现于：${formatDateTime(items.find((i) => i.id === editingId)?.discovered_at)}` : '录入后将直接公开' }}
        </span>
        <button class="btn-ghost !min-h-9 text-xs" @click="modalOpen = false">取消</button>
        <button class="btn-primary !min-h-9 text-xs" :disabled="saving" @click="save">
          {{ saving ? '保存中' : editingId ? '保存修改' : '创建并发布' }}
        </button>
      </template>
    </Modal>
  </div>
</template>
