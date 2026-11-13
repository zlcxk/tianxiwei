<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ArrowUpRight, Pencil, Plus, RefreshCw, Search, Star, Trash2 } from 'lucide-vue-next'
import { api } from '../../api/client'
import type { Content, EditorialPick, Paged } from '../../types'
import { contentStatusLabels, formatDateTime } from '../../lib/format'
import { toast } from '../../stores/toast'
import AdminPageHeader from './AdminPageHeader.vue'
import CoverArt from '../../components/CoverArt.vue'
import EmptyState from '../../components/EmptyState.vue'
import ImagePicker from '../../components/ImagePicker.vue'
import LoadingBlock from '../../components/LoadingBlock.vue'
import Modal from '../../components/Modal.vue'

const picks = ref<EditorialPick[]>([])
const loading = ref(true)
const error = ref('')
const saving = ref(false)
const togglingId = ref('')
const modalOpen = ref(false)
const editingId = ref('')

const contentKeyword = ref('')
const searching = ref(false)
const searchResults = ref<Content[]>([])
const selectedContent = ref<Content | null>(null)

const form = reactive({
  title: '',
  description: '',
  cover_url: '',
  sort_order: 0,
  starts_at: '',
  ends_at: '',
  enabled: true,
})

const activeCount = computed(() => picks.value.filter((p) => p.enabled).length)

function resetForm() {
  Object.assign(form, {
    title: '',
    description: '',
    cover_url: '',
    sort_order: 0,
    starts_at: '',
    ends_at: '',
    enabled: true,
  })
}

function toLocalInput(value: string | null): string {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`
}

function toISO(value: string): string | null {
  if (!value) return null
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? null : date.toISOString()
}

function effectiveTitle(pick: EditorialPick): string {
  return pick.title || pick.content?.title || '内容已不可用'
}

function effectiveDescription(pick: EditorialPick): string {
  return pick.description || pick.content?.summary || ''
}

function effectiveCover(pick: EditorialPick): string {
  return pick.cover_url || pick.content?.cover_url || ''
}

function windowLabel(pick: EditorialPick): string {
  if (pick.starts_at && pick.ends_at) {
    return `${formatDateTime(pick.starts_at)} 至 ${formatDateTime(pick.ends_at)}`
  }
  if (pick.starts_at) return `${formatDateTime(pick.starts_at)} 起`
  if (pick.ends_at) return `展示至 ${formatDateTime(pick.ends_at)}`
  return '长期展示'
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await api.get<{ items: EditorialPick[] }>('/admin/editorial-picks')
    picks.value = data.items
  } catch (err) {
    error.value = err instanceof Error ? err.message : '编辑精选加载失败'
  } finally {
    loading.value = false
  }
}

async function searchContent() {
  searching.value = true
  try {
    const params = new URLSearchParams({ content_status: 'published', page_size: '8' })
    if (contentKeyword.value.trim()) params.set('keyword', contentKeyword.value.trim())
    const data = await api.get<Paged<Content>>(`/admin/contents?${params.toString()}`)
    searchResults.value = data.items.filter((item) => item.content_status === 'published')
  } catch (err) {
    toast.error(err instanceof Error ? err.message : '搜索内容失败')
    searchResults.value = []
  } finally {
    searching.value = false
  }
}

function openCreate() {
  editingId.value = ''
  selectedContent.value = null
  contentKeyword.value = ''
  searchResults.value = []
  resetForm()
  modalOpen.value = true
  searchContent()
}

function openEdit(pick: EditorialPick) {
  editingId.value = pick.id
  selectedContent.value = pick.content ?? null
  contentKeyword.value = ''
  searchResults.value = []
  Object.assign(form, {
    title: pick.title,
    description: pick.description,
    cover_url: pick.cover_url,
    sort_order: pick.sort_order,
    starts_at: toLocalInput(pick.starts_at),
    ends_at: toLocalInput(pick.ends_at),
    enabled: pick.enabled,
  })
  modalOpen.value = true
}

function chooseContent(content: Content) {
  selectedContent.value = content
  searchResults.value = []
}

function clearSelectedContent() {
  selectedContent.value = null
  searchContent()
}

async function save() {
  if (saving.value) return
  if (!selectedContent.value) {
    toast.error('请先选择一条已发布内容')
    return
  }
  if (form.starts_at && form.ends_at && new Date(form.ends_at) <= new Date(form.starts_at)) {
    toast.error('结束时间必须晚于开始时间')
    return
  }
  saving.value = true
  try {
    const payload = {
      content_id: selectedContent.value.id,
      title: form.title.trim(),
      description: form.description.trim(),
      cover_url: form.cover_url.trim(),
      sort_order: Number(form.sort_order) || 0,
      starts_at: toISO(form.starts_at),
      ends_at: toISO(form.ends_at),
      enabled: form.enabled,
    }
    if (editingId.value) {
      await api.put(`/admin/editorial-picks/${editingId.value}`, payload)
      toast.success('编辑精选已更新')
    } else {
      await api.post('/admin/editorial-picks', payload)
      toast.success('编辑精选已创建')
    }
    modalOpen.value = false
    await load()
  } catch (err) {
    toast.error(err instanceof Error ? err.message : '保存编辑精选失败')
  } finally {
    saving.value = false
  }
}

async function toggle(pick: EditorialPick) {
  if (togglingId.value) return
  togglingId.value = pick.id
  try {
    await api.put(`/admin/editorial-picks/${pick.id}`, {
      content_id: pick.content_id,
      title: pick.title,
      description: pick.description,
      cover_url: pick.cover_url,
      sort_order: pick.sort_order,
      starts_at: pick.starts_at,
      ends_at: pick.ends_at,
      enabled: !pick.enabled,
    })
    toast.success(pick.enabled ? '精选已停用' : '精选已启用')
    await load()
  } catch (err) {
    toast.error(err instanceof Error ? err.message : '更新状态失败')
  } finally {
    togglingId.value = ''
  }
}

async function remove(pick: EditorialPick) {
  if (!window.confirm(`删除精选「${effectiveTitle(pick)}」？关联内容不会被删除。`)) return
  try {
    await api.del(`/admin/editorial-picks/${pick.id}`)
    toast.success('编辑精选已删除')
    await load()
  } catch (err) {
    toast.error(err instanceof Error ? err.message : '删除编辑精选失败')
  }
}

onMounted(load)
</script>

<template>
  <div>
    <AdminPageHeader
      eyebrow="Console / Editorial"
      title="编辑精选"
      description="从已发布内容中挑选首页展示条目。精选只保存展示信息与关联关系，不复制内容，也不改变原始发布时间。"
    >
      <template #actions>
        <button class="btn-ghost !min-h-9 !px-3 text-[11px]" :disabled="loading" @click="load">
          <RefreshCw class="h-3.5 w-3.5" :class="loading ? 'animate-spin' : ''" />
          刷新
        </button>
        <button class="btn-primary !min-h-9 !px-3 text-[11px]" @click="openCreate">
          <Plus class="h-3.5 w-3.5" />
          新建精选
        </button>
      </template>
    </AdminPageHeader>

    <div class="mb-5 flex flex-wrap gap-x-6 gap-y-2 text-[10px] text-muted">
      <span>共 {{ picks.length }} 条精选</span>
      <span>已启用 {{ activeCount }} 条</span>
      <span>停用、未到展示时间或关联内容已下线的精选不会出现在首页</span>
    </div>

    <LoadingBlock v-if="loading" :rows="4" />
    <EmptyState v-else-if="error" title="编辑精选加载失败" :description="error" action-label="重新加载" @action="load" />
    <EmptyState
      v-else-if="!picks.length"
      title="还没有编辑精选"
      description="从已审核内容中挑选值得推荐的照片、采访或活动回顾。"
      action-label="新建精选"
      @action="openCreate"
    />

    <div v-else class="space-y-3">
      <article
        v-for="pick in picks"
        :key="pick.id"
        class="flex flex-col gap-4 border border-line bg-white/40 p-4 sm:flex-row"
      >
        <div class="w-full shrink-0 overflow-hidden sm:w-24">
          <CoverArt :title="effectiveTitle(pick)" :src="effectiveCover(pick)" aspect="square" />
        </div>
        <div class="min-w-0 flex-1">
          <div class="flex flex-wrap items-center gap-2">
            <h2 class="font-display text-[17px] tracking-tight">{{ effectiveTitle(pick) }}</h2>
            <span
              class="border px-1.5 py-0.5 text-[9px]"
              :class="pick.enabled ? 'border-moss/30 bg-moss/10 text-moss' : 'border-line bg-white text-muted'"
            >
              {{ pick.enabled ? '已启用' : '已停用' }}
            </span>
            <span class="text-[9px] text-muted">顺序 {{ pick.sort_order }}</span>
          </div>
          <p v-if="effectiveDescription(pick)" class="mt-1.5 line-clamp-2 text-[10px] leading-5 text-muted">
            {{ effectiveDescription(pick) }}
          </p>
          <div class="mt-2 flex flex-wrap gap-x-3 gap-y-1 text-[10px] text-muted">
            <span>{{ windowLabel(pick) }}</span>
            <span>创建于 {{ formatDateTime(pick.created_at) }}</span>
            <span v-if="pick.content" class="inline-flex items-center gap-1">
              <Star class="h-3 w-3" />
              {{ contentStatusLabels[pick.content.content_status] || pick.content.content_status }}
            </span>
          </div>
        </div>
        <div class="flex shrink-0 flex-wrap items-center gap-1.5 self-start border-t border-line pt-3 sm:border-0 sm:pt-0">
          <RouterLink
            v-if="pick.content"
            :to="{ name: 'content', params: { id: pick.content.id } }"
            target="_blank"
            class="btn-ghost !min-h-7 !px-2.5 text-[10px]"
          >
            预览
            <ArrowUpRight class="h-3 w-3" />
          </RouterLink>
          <button
            class="btn-ghost !min-h-7 !px-2.5 text-[10px]"
            :disabled="togglingId === pick.id"
            @click="toggle(pick)"
          >
            {{ pick.enabled ? '停用' : '启用' }}
          </button>
          <button class="btn-ghost !min-h-7 !px-2.5 text-[10px]" @click="openEdit(pick)">
            <Pencil class="h-3 w-3" />编辑
          </button>
          <button class="btn-icon !h-7 !w-7 hover:!text-accent" title="删除精选" @click="remove(pick)">
            <Trash2 class="h-3 w-3" />
          </button>
        </div>
      </article>
    </div>

    <Modal
      :open="modalOpen"
      :title="editingId ? '编辑精选' : '新建精选'"
      description="精选只引用已发布内容；标题、描述与封面留空时使用内容自身信息。"
      @close="modalOpen = false"
    >
      <div class="space-y-6">
        <div>
          <div class="field-label">关联内容 *</div>
          <div v-if="selectedContent" class="flex items-start gap-3 border border-line bg-white/60 p-3">
            <div class="w-14 shrink-0 overflow-hidden">
              <CoverArt :title="selectedContent.title" :src="selectedContent.cover_url" aspect="square" />
            </div>
            <div class="min-w-0 flex-1">
              <p class="line-clamp-2 text-xs leading-5">{{ selectedContent.title }}</p>
              <p class="mt-1 text-[10px] text-muted">
                {{ selectedContent.source_name }} ·
                {{ contentStatusLabels[selectedContent.content_status] || selectedContent.content_status }}
              </p>
            </div>
            <button class="text-[10px] text-muted underline underline-offset-4 hover:text-ink" @click="clearSelectedContent">
              更换
            </button>
          </div>
          <div v-else class="mt-1 space-y-2">
            <div class="flex gap-2">
              <input
                v-model="contentKeyword"
                class="field !py-2.5 text-xs"
                placeholder="搜索已发布内容的标题或关键词"
                @keyup.enter="searchContent"
              />
              <button class="btn-ghost !min-h-9 shrink-0 !px-3 text-[11px]" :disabled="searching" @click="searchContent">
                <Search class="h-3.5 w-3.5" />
                {{ searching ? '搜索中' : '搜索' }}
              </button>
            </div>
            <div v-if="searchResults.length" class="max-h-56 divide-y divide-line overflow-y-auto border border-line bg-white/50">
              <button
                v-for="item in searchResults"
                :key="item.id"
                class="flex w-full items-start gap-3 px-3 py-2.5 text-left transition hover:bg-blush/30"
                @click="chooseContent(item)"
              >
                <span class="mt-0.5 shrink-0 border border-line px-1.5 py-0.5 text-[8px] text-muted">
                  {{ contentStatusLabels[item.content_status] || item.content_status }}
                </span>
                <span class="min-w-0 flex-1">
                  <span class="line-clamp-2 block text-xs leading-5">{{ item.title }}</span>
                  <span class="mt-1 block text-[10px] text-muted">{{ item.source_name }} · {{ formatDateTime(item.effective_at) }}</span>
                </span>
              </button>
            </div>
            <p v-else-if="!searching" class="text-[10px] text-muted">没有找到可用的已发布内容，请调整关键词。</p>
          </div>
        </div>

        <div class="grid gap-4">
          <div>
            <label for="pick-title" class="field-label">精选标题（可选）</label>
            <input id="pick-title" v-model="form.title" class="field" placeholder="留空则使用内容标题" />
          </div>
          <div>
            <label for="pick-description" class="field-label">简短描述（可选）</label>
            <textarea
              id="pick-description"
              v-model="form.description"
              rows="3"
              class="field resize-y"
              placeholder="留空则使用内容摘要"
            />
          </div>
          <div>
            <ImagePicker
              id="pick-cover"
              v-model="form.cover_url"
              label="封面（可选）"
              aspect="wide"
              hint="16:9 精选配图；留空则使用内容封面或站内抽象封面"
            />
          </div>
        </div>

        <div class="grid gap-4 sm:grid-cols-2">
          <div>
            <label for="pick-start" class="field-label">展示开始时间</label>
            <input id="pick-start" v-model="form.starts_at" type="datetime-local" class="field" />
          </div>
          <div>
            <label for="pick-end" class="field-label">展示结束时间</label>
            <input id="pick-end" v-model="form.ends_at" type="datetime-local" class="field" />
          </div>
        </div>

        <div class="grid gap-4 sm:grid-cols-2">
          <div>
            <label for="pick-order" class="field-label">展示顺序</label>
            <input id="pick-order" v-model.number="form.sort_order" type="number" class="field" />
            <p class="mt-1 text-[10px] text-muted">数字越小越靠前。</p>
          </div>
          <label class="flex items-center gap-2.5 pt-6 text-xs sm:pt-7">
            <input v-model="form.enabled" type="checkbox" class="h-4 w-4 accent-[#9a6c72]" />
            启用并在首页展示
          </label>
        </div>
      </div>
      <template #footer>
        <button class="btn-ghost !min-h-9 text-xs" @click="modalOpen = false">取消</button>
        <button class="btn-primary !min-h-9 text-xs" :disabled="saving" @click="save">
          {{ saving ? '保存中' : '保存' }}
        </button>
      </template>
    </Modal>
  </div>
</template>
