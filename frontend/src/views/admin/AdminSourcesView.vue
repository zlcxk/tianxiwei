<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import {
  AlertTriangle,
  CheckCircle2,
  Clock,
  Pencil,
  Play,
  Plus,
  RefreshCw,
  Rss,
  Settings2,
} from 'lucide-vue-next'
import { api } from '../../api/client'
import type { Source } from '../../types'
import { formatDateTime, relativeTime, sourceTypeLabels, trustLabels } from '../../lib/format'
import { toast } from '../../stores/toast'
import AdminPageHeader from './AdminPageHeader.vue'
import EmptyState from '../../components/EmptyState.vue'
import LoadingBlock from '../../components/LoadingBlock.vue'
import Modal from '../../components/Modal.vue'

type SourceForm = {
  id: string
  name: string
  platform: string
  source_type: string
  trust_level: string
  adapter_type: 'rss' | 'manual'
  interval_minutes: number
  enabled: boolean
  feed_url: string
  keywords: string
  use_feed_images: boolean
  note: string
  url: string
  extra: Record<string, unknown>
}

const items = ref<Source[]>([])
const loading = ref(true)
const error = ref('')
const modalOpen = ref(false)
const saving = ref(false)
const acting = ref('')
const editingId = ref('')

const sourceTypes = ['official', 'media', 'community', 'reference', 'internal']
const trustLevels = ['official', 'media', 'community', 'reference', 'internal']

const form = reactive<SourceForm>({
  id: '',
  name: '',
  platform: '',
  source_type: 'media',
  trust_level: 'media',
  adapter_type: 'rss',
  interval_minutes: 120,
  enabled: true,
  feed_url: '',
  keywords: '',
  use_feed_images: false,
  note: '',
  url: '',
  extra: {},
})

const enabledCount = computed(() => items.value.filter((s) => s.enabled).length)
const errorCount = computed(() => items.value.filter((s) => s.enabled && s.last_error).length)

// A source that still succeeds but has produced no valid content for a week is
// effectively dead weight; surface it so editors can fix keywords or disable it.
function isLowQuality(source: Source): boolean {
  if (!source.enabled || source.recent_valid_count > 0 || !source.last_success_at) return false
  return Date.now() - new Date(source.last_success_at).getTime() > 7 * 24 * 3600 * 1000
}
const lowQualityCount = computed(() => items.value.filter(isLowQuality).length)

function configString(config: Record<string, unknown>, key: string): string {
  const value = config?.[key]
  return typeof value === 'string' ? value : ''
}

function configKeywords(config: Record<string, unknown>): string {
  const value = config?.keywords
  if (Array.isArray(value)) return value.join(', ')
  return typeof value === 'string' ? value : ''
}

function resetForm() {
  Object.assign(form, {
    id: '',
    name: '',
    platform: '',
    source_type: 'media',
    trust_level: 'media',
    adapter_type: 'rss',
    interval_minutes: 120,
    enabled: true,
    feed_url: '',
    keywords: '',
    use_feed_images: false,
    note: '',
    url: '',
    extra: {},
  })
}

function openCreate() {
  editingId.value = ''
  resetForm()
  modalOpen.value = true
}

function openEdit(source: Source) {
  editingId.value = source.id
  const config = { ...(source.config || {}) }
  const { feed_url, keywords, use_feed_images, note, url, ...extra } = config
  Object.assign(form, {
    id: source.id,
    name: source.name,
    platform: source.platform,
    source_type: source.source_type,
    trust_level: source.trust_level,
    adapter_type: source.adapter_type === 'manual' ? 'manual' : 'rss',
    interval_minutes: source.interval_minutes,
    enabled: source.enabled,
    feed_url: typeof feed_url === 'string' ? feed_url : '',
    keywords: configKeywords(config),
    use_feed_images: use_feed_images === true,
    note: typeof note === 'string' ? note : '',
    url: typeof url === 'string' ? url : '',
    extra,
  })
  modalOpen.value = true
}

function buildConfig(): Record<string, unknown> {
  const config: Record<string, unknown> = { ...form.extra }
  if (form.adapter_type === 'rss') {
    config.feed_url = form.feed_url.trim()
    config.keywords = form.keywords
      .split(/[,，\n]/)
      .map((k) => k.trim())
      .filter(Boolean)
    config.use_feed_images = form.use_feed_images
  }
  if (form.note.trim()) config.note = form.note.trim()
  else delete config.note
  if (form.url.trim()) config.url = form.url.trim()
  else delete config.url
  return config
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await api.get<{ items: Source[] }>('/admin/sources')
    items.value = data.items
  } catch (err) {
    error.value = err instanceof Error ? err.message : '数据源加载失败'
  } finally {
    loading.value = false
  }
}

async function save() {
  if (saving.value) return
  if (!form.name.trim()) {
    toast.error('请填写来源名称')
    return
  }
  if (form.adapter_type === 'rss' && !form.feed_url.trim()) {
    toast.error('RSS 来源需要填写 Feed 地址')
    return
  }
  saving.value = true
  const body = {
    name: form.name.trim(),
    platform: form.platform.trim(),
    source_type: form.source_type,
    trust_level: form.trust_level,
    adapter_type: form.adapter_type,
    interval_minutes: Number(form.interval_minutes) || 120,
    enabled: form.enabled,
    config: buildConfig(),
  }
  try {
    if (editingId.value) {
      await api.patch(`/admin/sources/${editingId.value}`, body)
      toast.success('数据源已更新')
    } else {
      await api.post('/admin/sources', body)
      toast.success('数据源已创建')
    }
    modalOpen.value = false
    await load()
  } catch (err) {
    toast.error(err instanceof Error ? err.message : '保存失败')
  } finally {
    saving.value = false
  }
}

async function toggle(source: Source) {
  if (acting.value) return
  acting.value = source.id
  try {
    const updated = await api.patch<Source>(`/admin/sources/${source.id}`, { enabled: !source.enabled })
    const index = items.value.findIndex((s) => s.id === source.id)
    if (index >= 0) items.value[index] = updated
    toast.success(updated.enabled ? '已启用数据源' : '已停用数据源')
  } catch (err) {
    toast.error(err instanceof Error ? err.message : '操作失败')
  } finally {
    acting.value = ''
  }
}

async function run(source: Source) {
  if (source.adapter_type === 'manual') {
    toast.info('手动来源不支持自动采集，请使用内容手动录入')
    return
  }
  if (acting.value) return
  acting.value = source.id
  try {
    await api.post(`/admin/sources/${source.id}/run`)
    toast.success('采集任务已排队，稍后可在「采集任务」查看')
  } catch (err) {
    toast.error(err instanceof Error ? err.message : '任务创建失败')
  } finally {
    acting.value = ''
  }
}

onMounted(load)
</script>

<template>
  <div>
    <AdminPageHeader
      eyebrow="Console / Sources"
      title="数据源"
      description="每个来源可独立启停。仅接入公开 RSS 或人工核验录入，不使用模拟登录或绕过反爬。"
    >
      <template #actions>
        <button class="btn-ghost !min-h-9 !px-3 text-[11px]" :disabled="loading" @click="load">
          <RefreshCw class="h-3.5 w-3.5" :class="loading ? 'animate-spin' : ''" />
          刷新
        </button>
        <button class="btn-primary !min-h-9 !px-3 text-[11px]" @click="openCreate">
          <Plus class="h-3.5 w-3.5" />
          新建来源
        </button>
      </template>
    </AdminPageHeader>

    <div class="mb-5 flex flex-wrap items-center gap-x-6 gap-y-2 border-b border-line pb-4 text-[11px] text-muted">
      <span>共 {{ items.length }} 个来源</span>
      <span class="text-moss">启用 {{ enabledCount }}</span>
      <span v-if="errorCount" class="text-accent">异常 {{ errorCount }}</span>
      <span v-if="lowQualityCount" class="text-[#8a6a32]">近期无有效内容 {{ lowQualityCount }}</span>
      <span class="ml-auto text-[10px]">密钥字段在接口返回中脱敏，编辑时不会回传明文。</span>
    </div>

    <LoadingBlock v-if="loading" :rows="4" />

    <EmptyState v-else-if="error" title="数据源加载失败" :description="error" action-label="重新加载" @action="load" />

    <EmptyState
      v-else-if="!items.length"
      title="还没有数据源"
      description="添加公开 RSS 源，或建立人工核验来源。"
      action-label="新建来源"
      @action="openCreate"
    />

    <div v-else class="space-y-3">
      <article v-for="source in items" :key="source.id" class="border border-line bg-canvas">
        <div class="flex flex-col gap-4 p-5 md:flex-row md:items-start md:justify-between">
          <div class="min-w-0 flex-1">
            <div class="flex flex-wrap items-center gap-2">
              <span
                class="flex h-6 w-6 items-center justify-center border"
                :class="source.adapter_type === 'rss' ? 'border-moss/30 bg-moss/10 text-moss' : 'border-line bg-white text-muted'"
              >
                <Rss v-if="source.adapter_type === 'rss'" class="h-3 w-3" />
                <Settings2 v-else class="h-3 w-3" />
              </span>
              <h2 class="font-display text-lg tracking-tight">{{ source.name }}</h2>
              <span class="tag !py-0.5">{{ sourceTypeLabels[source.source_type] || source.source_type }}</span>
              <span class="tag !py-0.5">{{ source.adapter_type === 'rss' ? 'RSS 自动' : '人工录入' }}</span>
              <span
                class="text-[9px] uppercase tracking-widest"
                :class="source.enabled ? 'text-moss' : 'text-muted'"
              >{{ source.enabled ? '已启用' : '已停用' }}</span>
            </div>
            <div class="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1 text-[10px] text-muted">
              <span>{{ source.platform || '平台未填写' }}</span>
              <span>可信等级 · {{ trustLabels[source.trust_level] || source.trust_level }}</span>
              <span class="inline-flex items-center gap-1"><Clock class="h-3 w-3" />每 {{ source.interval_minutes }} 分钟</span>
            </div>
            <div v-if="source.config?.feed_url" class="mt-2 truncate text-[10px] text-muted">
              <span class="text-ink/50">Feed：</span>{{ source.config.feed_url }}
            </div>
            <div v-else-if="source.config?.note" class="mt-2 line-clamp-1 text-[10px] text-muted">
              {{ source.config.note }}
            </div>
            <div class="mt-3 flex flex-wrap items-center gap-x-5 gap-y-1 text-[10px]">
              <span v-if="source.last_success_at" class="inline-flex items-center gap-1 text-moss">
                <CheckCircle2 class="h-3 w-3" />最近成功 {{ relativeTime(source.last_success_at) }}
              </span>
              <span v-if="source.last_failure_at" class="inline-flex items-center gap-1 text-accent">
                <AlertTriangle class="h-3 w-3" />最近失败 {{ relativeTime(source.last_failure_at) }}
              </span>
              <span v-if="!source.last_success_at && !source.last_failure_at" class="text-muted">尚未运行</span>
              <span v-if="source.consecutive_failures > 0" class="inline-flex items-center gap-1 text-accent">
                <AlertTriangle class="h-3 w-3" />连续失败 {{ source.consecutive_failures }} 次
              </span>
              <span class="text-muted">近 30 天有效内容 {{ source.recent_valid_count }} 条</span>
            </div>
            <div
              v-if="isLowQuality(source)"
              class="mt-2 border-l-2 border-[#b99a5e]/40 bg-[#f3ead7]/40 px-3 py-2 text-[10px] leading-5 text-[#8a6a32]"
            >
              最近 7 天没有新的有效内容，建议检查关键词或停用该来源。
            </div>
            <div v-if="source.last_error" class="mt-2 border-l-2 border-accent/40 bg-blush/20 px-3 py-2 text-[10px] leading-5 text-accent">
              {{ source.last_error }}
            </div>
          </div>

          <div class="flex shrink-0 flex-wrap items-center gap-2 md:flex-col md:items-end">
            <button
              class="relative h-5 w-9 rounded-full transition"
              :class="source.enabled ? 'bg-accent' : 'bg-line'"
              :aria-pressed="source.enabled"
              :aria-label="source.enabled ? '停用来源' : '启用来源'"
              :disabled="acting === source.id"
              @click="toggle(source)"
            >
              <span
                class="absolute top-0.5 h-4 w-4 rounded-full bg-white shadow transition"
                :class="source.enabled ? 'left-[18px]' : 'left-0.5'"
              />
            </button>
            <div class="flex items-center gap-1.5">
              <button
                class="btn-ghost !min-h-8 !px-2.5 text-[10px]"
                :disabled="source.adapter_type === 'manual' || acting === source.id"
                :title="source.adapter_type === 'manual' ? '手动来源不支持自动采集' : '立即运行一次'"
                @click="run(source)"
              >
                <Play class="h-3 w-3" />
                运行
              </button>
              <button class="btn-ghost !min-h-8 !px-2.5 text-[10px]" @click="openEdit(source)">
                <Pencil class="h-3 w-3" />
                编辑
              </button>
            </div>
            <div class="text-[9px] text-muted md:text-right">
              创建 {{ formatDateTime(source.created_at) }}
            </div>
          </div>
        </div>
      </article>
    </div>

    <Modal
      :open="modalOpen"
      :title="editingId ? '编辑数据源' : '新建数据源'"
      description="仅填写公开可访问的 RSS 地址；人工来源用于审核后录入。"
      size="lg"
      @close="modalOpen = false"
    >
      <div class="grid gap-5 sm:grid-cols-2">
        <div class="sm:col-span-2">
          <label for="source-name" class="field-label">来源名称</label>
          <input id="source-name" v-model="form.name" class="field" placeholder="例如：某媒体 · 田曦薇新闻" />
        </div>
        <div>
          <label for="source-platform" class="field-label">平台</label>
          <input id="source-platform" v-model="form.platform" class="field" placeholder="例如：新闻聚合 / 官方微博" />
        </div>
        <div>
          <label for="source-interval" class="field-label">采集间隔（分钟，最小 5）</label>
          <input id="source-interval" v-model.number="form.interval_minutes" type="number" min="5" class="field" />
        </div>
        <div>
          <label for="source-type" class="field-label">来源类型</label>
          <select id="source-type" v-model="form.source_type" class="select-field">
            <option v-for="type in sourceTypes" :key="type" :value="type">{{ sourceTypeLabels[type] }}</option>
          </select>
        </div>
        <div>
          <label for="source-trust" class="field-label">可信等级</label>
          <select id="source-trust" v-model="form.trust_level" class="select-field">
            <option v-for="level in trustLevels" :key="level" :value="level">{{ trustLabels[level] }}</option>
          </select>
        </div>
        <div>
          <label for="source-adapter" class="field-label">采集方式</label>
          <select id="source-adapter" v-model="form.adapter_type" class="select-field">
            <option value="rss">RSS 自动采集</option>
            <option value="manual">人工核验录入</option>
          </select>
        </div>
        <div class="flex items-end pb-2">
          <label class="flex cursor-pointer items-center gap-2.5 text-xs">
            <input v-model="form.enabled" type="checkbox" class="h-4 w-4 accent-[#c98791]" />
            启用该来源
          </label>
        </div>

        <template v-if="form.adapter_type === 'rss'">
          <div class="sm:col-span-2">
            <label for="source-feed" class="field-label">RSS / Atom 地址</label>
            <input id="source-feed" v-model="form.feed_url" class="field" placeholder="https://example.com/feed.xml" />
            <p class="mt-1.5 text-[10px] leading-5 text-muted">必须为公开可访问的 http(s) 地址，服务端会以明确的 Bot User-Agent 读取。</p>
          </div>
          <div class="sm:col-span-2">
            <label for="source-keywords" class="field-label">关键词过滤（逗号分隔，留空表示不过滤）</label>
            <input id="source-keywords" v-model="form.keywords" class="field" placeholder="田曦薇, 曦薇" />
          </div>
          <label class="flex cursor-pointer items-start gap-2.5 text-xs sm:col-span-2">
            <input v-model="form.use_feed_images" type="checkbox" class="mt-0.5 h-4 w-4 accent-[#c98791]" />
            <span>
              引用来源图片
              <span class="block text-[10px] leading-5 text-muted">
                默认关闭。仅在你已确认该来源图片拥有展示授权时开启；否则条目将使用站内抽象封面。
              </span>
            </span>
          </label>
        </template>

        <div class="sm:col-span-2">
          <label for="source-note" class="field-label">备注 / 使用说明</label>
          <textarea id="source-note" v-model="form.note" rows="2" class="field resize-y" placeholder="说明该来源的用途与核验方式" />
        </div>
        <div v-if="form.adapter_type === 'manual'" class="sm:col-span-2">
          <label for="source-url" class="field-label">参考链接（可选）</label>
          <input id="source-url" v-model="form.url" class="field" placeholder="https://..." />
        </div>
      </div>
      <template #footer>
        <button class="btn-ghost !min-h-9 text-xs" @click="modalOpen = false">取消</button>
        <button class="btn-primary !min-h-9 text-xs" :disabled="saving" @click="save">
          {{ saving ? '保存中' : editingId ? '保存修改' : '创建来源' }}
        </button>
      </template>
    </Modal>
  </div>
</template>
