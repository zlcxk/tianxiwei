<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { CalendarDays, MapPin, Pencil, Plus, RefreshCw, Trash2 } from 'lucide-vue-next'
import { api } from '../../api/client'
import type { Event } from '../../types'
import { formatDate } from '../../lib/format'
import { toast } from '../../stores/toast'
import AdminPageHeader from './AdminPageHeader.vue'
import EmptyState from '../../components/EmptyState.vue'
import ImagePicker from '../../components/ImagePicker.vue'
import LoadingBlock from '../../components/LoadingBlock.vue'
import Modal from '../../components/Modal.vue'

const route = useRoute()
const events = ref<Event[]>([])
const loading = ref(true)
const error = ref('')
const saving = ref(false)
const modalOpen = ref(false)
const editingId = ref('')

const statusOptions = [
  { value: 'draft', label: '草稿' },
  { value: 'published', label: '已公开' },
  { value: 'archived', label: '已归档' },
]

const form = reactive({
  title: '',
  summary: '',
  event_date: '',
  location: '',
  cover_url: '',
  status: 'draft',
})

const publishedCount = computed(() => events.value.filter((e) => e.status === 'published').length)

function resetForm() {
  Object.assign(form, {
    title: '',
    summary: '',
    event_date: '',
    location: '',
    cover_url: '',
    status: 'draft',
  })
}

function openCreate() {
  editingId.value = ''
  resetForm()
  modalOpen.value = true
}

function openEdit(event: Event) {
  editingId.value = event.id
  Object.assign(form, {
    title: event.title,
    summary: event.summary,
    event_date: event.event_date,
    location: event.location,
    cover_url: event.cover_url,
    status: event.status,
  })
  modalOpen.value = true
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await api.get<{ items: Event[] }>('/admin/events')
    events.value = data.items
  } catch (err) {
    error.value = err instanceof Error ? err.message : '事件列表加载失败'
  } finally {
    loading.value = false
  }
}

async function save() {
  if (saving.value) return
  if (!form.title.trim()) {
    toast.error('事件标题不能为空')
    return
  }
  saving.value = true
  try {
    const payload = {
      title: form.title.trim(),
      summary: form.summary,
      event_date: form.event_date,
      location: form.location,
      cover_url: form.cover_url,
      status: form.status,
    }
    if (editingId.value) {
      await api.put(`/admin/events/${editingId.value}`, payload)
      toast.success('事件已更新')
    } else {
      await api.post('/admin/events', payload)
      toast.success('事件已创建')
    }
    modalOpen.value = false
    await load()
  } catch (err) {
    toast.error(err instanceof Error ? err.message : '保存事件失败')
  } finally {
    saving.value = false
  }
}

async function remove(event: Event) {
  if (!window.confirm(`删除事件「${event.title}」？关联内容不会被删除，只会解除事件关联。`)) return
  try {
    await api.del(`/admin/events/${event.id}`)
    toast.success('事件已删除')
    await load()
  } catch (err) {
    toast.error(err instanceof Error ? err.message : '删除事件失败')
  }
}

function statusLabel(status: string): string {
  return statusOptions.find((s) => s.value === status)?.label || status
}

onMounted(async () => {
  await load()
  const edit = typeof route.query.edit === 'string' ? route.query.edit : ''
  if (!edit) return
  const event = events.value.find((item) => item.id === edit)
  if (event) openEdit(event)
  else toast.error('事件不存在，可能已被删除')
})
</script>

<template>
  <div>
    <AdminPageHeader
      eyebrow="Console / Events"
      title="事件管理"
      description="把同一公开活动、作品节点或采访的分散内容归入一个事件。事件只做聚合，不改变内容原始发布时间，也不影响主时间线排序。"
    >
      <template #actions>
        <button class="btn-ghost !min-h-9 !px-3 text-[11px]" :disabled="loading" @click="load">
          <RefreshCw class="h-3.5 w-3.5" :class="loading ? 'animate-spin' : ''" />
          刷新
        </button>
        <button class="btn-primary !min-h-9 !px-3 text-[11px]" @click="openCreate">
          <Plus class="h-3.5 w-3.5" />
          新建事件
        </button>
      </template>
    </AdminPageHeader>

    <div class="mb-5 flex flex-wrap gap-x-6 gap-y-2 text-[10px] text-muted">
      <span>共 {{ events.length }} 个事件</span>
      <span>已公开 {{ publishedCount }} 个</span>
      <span>仅已公开事件对普通用户可见</span>
    </div>

    <LoadingBlock v-if="loading" :rows="5" />
    <EmptyState v-else-if="error" title="事件列表加载失败" :description="error" action-label="重新加载" @action="load" />
    <EmptyState
      v-else-if="!events.length"
      title="还没有事件"
      description="创建事件后，可在内容管理里把多条内容关联到同一事件。"
      action-label="新建事件"
      @action="openCreate"
    />

    <div v-else class="grid gap-3 md:grid-cols-2">
      <article v-for="event in events" :key="event.id" class="border border-line bg-white/40 p-4">
        <div class="flex items-start justify-between gap-3">
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <h2 class="font-display text-[17px] tracking-tight">{{ event.title }}</h2>
              <span
                class="border px-1.5 py-0.5 text-[9px]"
                :class="event.status === 'published'
                  ? 'border-moss/30 bg-moss/10 text-moss'
                  : event.status === 'archived'
                    ? 'border-line bg-white text-muted'
                    : 'border-[#b99a5e]/30 bg-[#f3ead7] text-[#8a6a32]'"
              >
                {{ statusLabel(event.status) }}
              </span>
            </div>
            <p v-if="event.summary" class="mt-1.5 line-clamp-2 text-[10px] leading-5 text-muted">{{ event.summary }}</p>
            <div class="mt-2 flex flex-wrap gap-x-3 gap-y-1 text-[10px] text-muted">
              <span v-if="event.event_date" class="inline-flex items-center gap-1">
                <CalendarDays class="h-3 w-3" />{{ formatDate(event.event_date) }}
              </span>
              <span v-if="event.location" class="inline-flex items-center gap-1">
                <MapPin class="h-3 w-3" />{{ event.location }}
              </span>
              <span>{{ event.content_count }} 条已公开关联内容</span>
            </div>
          </div>
        </div>
        <div class="mt-3 flex items-center gap-1.5 border-t border-line pt-3">
          <RouterLink
            v-if="event.status === 'published'"
            :to="{ name: 'event', params: { id: event.id } }"
            target="_blank"
            class="btn-ghost !min-h-7 !px-2.5 text-[10px]"
          >
            预览
          </RouterLink>
          <button class="btn-ghost !min-h-7 !px-2.5 text-[10px]" @click="openEdit(event)">
            <Pencil class="h-3 w-3" />编辑
          </button>
          <button class="btn-icon !h-7 !w-7 hover:!text-accent" title="删除事件" @click="remove(event)">
            <Trash2 class="h-3 w-3" />
          </button>
        </div>
      </article>
    </div>

    <Modal
      :open="modalOpen"
      :title="editingId ? '编辑事件' : '新建事件'"
      description="事件用于聚合相关内容，不会修改任何内容的发布时间。"
      @close="modalOpen = false"
    >
      <div class="grid gap-4">
        <div>
          <label for="event-title" class="field-label">事件标题 *</label>
          <input id="event-title" v-model="form.title" class="field" placeholder="例如：2026 秋冬品牌活动" />
        </div>
        <div>
          <label for="event-summary" class="field-label">事件说明</label>
          <textarea id="event-summary" v-model="form.summary" rows="3" class="field resize-y" placeholder="简要说明事件背景" />
        </div>
        <div class="grid gap-4 sm:grid-cols-2">
          <div>
            <label for="event-date" class="field-label">事件日期</label>
            <input id="event-date" v-model="form.event_date" type="date" class="field" />
          </div>
          <div>
            <label for="event-status" class="field-label">状态</label>
            <select id="event-status" v-model="form.status" class="select-field">
              <option v-for="option in statusOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
            </select>
          </div>
        </div>
        <div>
          <label for="event-location" class="field-label">地点</label>
          <input id="event-location" v-model="form.location" class="field" placeholder="例如：上海" />
        </div>
        <div>
          <ImagePicker
            id="event-cover"
            v-model="form.cover_url"
            label="封面（可选，需已获授权）"
            aspect="wide"
            hint="16:9 活动主视觉；留空则使用站内抽象封面"
          />
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
