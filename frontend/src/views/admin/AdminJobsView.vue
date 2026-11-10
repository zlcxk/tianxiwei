<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { ArrowUpRight, RefreshCw, RotateCcw } from 'lucide-vue-next'
import { api } from '../../api/client'
import type { Job } from '../../types'
import { formatDateTime, jobStatusLabels, relativeTime } from '../../lib/format'
import { toast } from '../../stores/toast'
import AdminPageHeader from './AdminPageHeader.vue'
import EmptyState from '../../components/EmptyState.vue'
import LoadingBlock from '../../components/LoadingBlock.vue'

const jobs = ref<Job[]>([])
const loading = ref(true)
const refreshing = ref(false)
const error = ref('')
const filter = ref<'all' | 'queued' | 'running' | 'success' | 'partial' | 'failed'>('all')
const retrying = ref('')
const autoRefresh = ref(true)
let timer: ReturnType<typeof setInterval> | undefined

const tabs: { value: typeof filter.value; label: string }[] = [
  { value: 'all', label: '全部' },
  { value: 'queued', label: '排队中' },
  { value: 'running', label: '采集中' },
  { value: 'success', label: '成功' },
  { value: 'partial', label: '部分成功' },
  { value: 'failed', label: '失败' },
]

const filtered = computed(() => (filter.value === 'all' ? jobs.value : jobs.value.filter((j) => j.status === filter.value)))
const runningCount = computed(() => jobs.value.filter((j) => j.status === 'running').length)
const queuedCount = computed(() => jobs.value.filter((j) => j.status === 'queued').length)
const failedCount = computed(() => jobs.value.filter((j) => j.status === 'failed').length)

function countFor(status: typeof filter.value) {
  return status === 'all' ? jobs.value.length : jobs.value.filter((j) => j.status === status).length
}

function duration(job: Job): string {
  if (!job.started_at) return '—'
  const end = job.finished_at ? new Date(job.finished_at).getTime() : Date.now()
  const start = new Date(job.started_at).getTime()
  if (Number.isNaN(start) || Number.isNaN(end)) return '—'
  const seconds = Math.max(0, Math.round((end - start) / 1000))
  return seconds < 60 ? `${seconds}s` : `${Math.floor(seconds / 60)}m ${seconds % 60}s`
}

const statusTone: Record<string, string> = {
  success: 'text-moss border-moss/30 bg-moss/10',
  partial: 'text-[#8a6a32] border-[#b99a5e]/30 bg-[#f3ead7]',
  failed: 'text-accent border-accent/30 bg-blush/40',
  running: 'text-[#8a6a32] border-[#b99a5e]/30 bg-[#f3ead7]',
  queued: 'text-muted border-line bg-white',
  skipped: 'text-muted border-line bg-white',
}

async function load(silent = false) {
  if (silent) refreshing.value = true
  else loading.value = true
  error.value = ''
  try {
    const data = await api.get<{ items: Job[] }>('/admin/jobs')
    jobs.value = data.items
  } catch (err) {
    if (!silent) error.value = err instanceof Error ? err.message : '任务加载失败'
  } finally {
    loading.value = false
    refreshing.value = false
  }
}

async function retry(job: Job) {
  if (retrying.value) return
  retrying.value = job.id
  try {
    await api.post(`/admin/jobs/${job.id}/retry`)
    toast.success('已重新排队，稍后刷新查看结果')
    await load(true)
  } catch (err) {
    toast.error(err instanceof Error ? err.message : '重试失败')
  } finally {
    retrying.value = ''
  }
}

onMounted(() => {
  load()
  timer = setInterval(() => {
    if (autoRefresh.value) load(true)
  }, 15000)
})

onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
})
</script>

<template>
  <div>
    <AdminPageHeader
      eyebrow="Console / Collection Jobs"
      title="采集任务"
      description="任务由后台 Worker 执行，单个来源同时只允许一个活动任务。页面每 15 秒自动刷新。"
    >
      <template #actions>
        <label class="flex cursor-pointer items-center gap-2 text-[10px] text-muted">
          <input v-model="autoRefresh" type="checkbox" class="h-3.5 w-3.5 accent-[#c98791]" />
          自动刷新
        </label>
        <button class="btn-ghost !min-h-9 !px-3 text-[11px]" :disabled="refreshing" @click="load(true)">
          <RefreshCw class="h-3.5 w-3.5" :class="refreshing ? 'animate-spin' : ''" />
          刷新
        </button>
        <RouterLink to="/admin/sources" class="btn-primary !min-h-9 !px-3 text-[11px]">
          数据源
          <ArrowUpRight class="h-3.5 w-3.5" />
        </RouterLink>
      </template>
    </AdminPageHeader>

    <div class="mb-5 flex flex-col gap-3 border-b border-line pb-4 sm:flex-row sm:items-center sm:justify-between">
      <div class="no-scrollbar flex gap-1 overflow-x-auto">
        <button
          v-for="tab in tabs"
          :key="tab.value"
          class="shrink-0 border px-3 py-1.5 text-[11px] transition"
          :class="filter === tab.value ? 'border-ink bg-ink text-white' : 'border-line bg-white/40 text-muted hover:text-ink'"
          @click="filter = tab.value"
        >
          {{ tab.label }}
          <span class="ml-1 opacity-60">{{ countFor(tab.value) }}</span>
        </button>
      </div>
      <div class="flex items-center gap-4 text-[10px] text-muted">
        <span v-if="runningCount" class="inline-flex items-center gap-1 text-[#8a6a32]">
          <span class="h-1.5 w-1.5 animate-pulse rounded-full bg-[#b99a5e]" />
          采集中 {{ runningCount }}
        </span>
        <span>排队 {{ queuedCount }}</span>
        <span :class="failedCount ? 'text-accent' : ''">失败 {{ failedCount }}</span>
      </div>
    </div>

    <LoadingBlock v-if="loading" :rows="5" />

    <EmptyState v-else-if="error" title="任务加载失败" :description="error" action-label="重新加载" @action="load()" />

    <EmptyState
      v-else-if="!filtered.length"
      :icon="RefreshCw"
      :title="filter === 'all' ? '还没有采集任务' : '这个状态下没有任务'"
      description="在数据源页面点击「运行」，或等待 Worker 按间隔自动调度。"
      action-label="前往数据源"
      to="/admin/sources"
    />

    <div v-else class="space-y-2.5">
      <article v-for="job in filtered" :key="job.id" class="border border-line bg-canvas px-4 py-4 md:px-5">
        <div class="flex flex-wrap items-start gap-x-4 gap-y-3">
          <span class="w-14 shrink-0 border py-1 text-center text-[9px]" :class="statusTone[job.status]">
            {{ jobStatusLabels[job.status] || job.status }}
          </span>
          <div class="min-w-0 flex-1">
            <div class="flex flex-wrap items-baseline gap-x-2">
              <h2 class="font-display text-lg tracking-tight">{{ job.source_name }}</h2>
              <span class="text-[9px] uppercase tracking-widest text-muted">{{ job.trigger_type }}</span>
            </div>
            <div class="mt-1.5 flex flex-wrap gap-x-4 gap-y-1 text-[10px] text-muted">
              <span>创建 {{ formatDateTime(job.created_at) }}</span>
              <span v-if="job.started_at">开始 {{ formatDateTime(job.started_at) }}</span>
              <span v-if="job.finished_at">结束 {{ formatDateTime(job.finished_at) }}</span>
              <span>耗时 {{ duration(job) }}</span>
              <span class="text-muted/70">{{ relativeTime(job.created_at) }}</span>
            </div>
            <div class="mt-2.5 flex flex-wrap gap-x-5 gap-y-1 text-[10px]">
              <span>抓取 <span class="text-ink">{{ job.fetched_count }}</span></span>
              <span class="text-moss">新增 <span class="font-medium">{{ job.inserted_count }}</span></span>
              <span>去重 <span class="text-ink">{{ job.duplicate_count }}</span></span>
              <span :class="job.failed_count ? 'text-accent' : ''">失败条目 {{ job.failed_count }}</span>
            </div>
            <p v-if="job.error_message" class="mt-2 border-l-2 border-accent/40 bg-blush/20 px-3 py-2 text-[10px] leading-5 text-accent">
              {{ job.error_message }}
            </p>
          </div>
          <button
            v-if="job.status === 'failed'"
            class="btn-ghost !min-h-8 !shrink-0 !px-2.5 text-[10px]"
            :disabled="!!retrying"
            @click="retry(job)"
          >
            <RotateCcw class="h-3 w-3" :class="retrying === job.id ? 'animate-spin' : ''" />
            重试
          </button>
        </div>
      </article>
    </div>

    <p class="mt-6 text-[10px] leading-5 text-muted">
      仅展示最近 100 条任务。失败任务重试会新建一条关联任务；原始抓取数据保存在后台，不在普通用户接口暴露。
    </p>
  </div>
</template>
