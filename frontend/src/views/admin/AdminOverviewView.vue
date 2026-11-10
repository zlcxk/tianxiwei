<script setup lang="ts">
import { onMounted, ref } from 'vue'
import {
  AlertCircle,
  ArrowUpRight,
  ClipboardCheck,
  Database,
  FilePlus2,
  Files,
  RefreshCw,
} from 'lucide-vue-next'
import { api } from '../../api/client'
import type { Overview } from '../../types'
import { formatDateTime, jobStatusLabels, relativeTime } from '../../lib/format'
import AdminPageHeader from './AdminPageHeader.vue'
import EmptyState from '../../components/EmptyState.vue'
import LoadingBlock from '../../components/LoadingBlock.vue'

const data = ref<Overview | null>(null)
const loading = ref(true)
const error = ref('')

const statusTone: Record<string, string> = {
  success: 'text-moss border-moss/30 bg-moss/10',
  failed: 'text-accent border-accent/30 bg-blush/40',
  running: 'text-[#8a6a32] border-[#b99a5e]/30 bg-[#f3ead7]',
  queued: 'text-muted border-line bg-white',
  skipped: 'text-muted border-line bg-white',
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    data.value = await api.get<Overview>('/admin/overview')
  } catch (err) {
    error.value = err instanceof Error ? err.message : '概览加载失败'
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<template>
  <div>
    <AdminPageHeader
      eyebrow="Console / Overview"
      title="管理概览"
      description="采集、审核与来源健康的即时状态。所有数字来自真实数据表，不做模拟。"
    >
      <template #actions>
        <button class="btn-ghost !min-h-9 !px-3 text-[11px]" :disabled="loading" @click="load">
          <RefreshCw class="h-3.5 w-3.5" :class="loading ? 'animate-spin' : ''" />
          刷新
        </button>
        <RouterLink to="/admin/contents" class="btn-primary !min-h-9 !px-3 text-[11px]">
          <FilePlus2 class="h-3.5 w-3.5" />
          手动录入
        </RouterLink>
      </template>
    </AdminPageHeader>

    <LoadingBlock v-if="loading" type="text" :rows="8" />
    <EmptyState v-else-if="error" title="概览加载失败" :description="error" action-label="重新加载" @action="load" />

    <template v-else-if="data">
      <section class="grid gap-px overflow-hidden border border-line bg-line sm:grid-cols-2 xl:grid-cols-4">
        <div class="bg-canvas p-5 md:p-6">
          <div class="flex items-center justify-between text-[10px] uppercase tracking-widest2 text-muted">
            今日采集
            <Database class="h-3.5 w-3.5" />
          </div>
          <div class="mt-4 font-display text-[38px] leading-none">{{ data.today_collected }}</div>
          <p class="mt-2 text-[10px] text-muted">今日任务抓取条目总数</p>
        </div>
        <div class="bg-canvas p-5 md:p-6">
          <div class="flex items-center justify-between text-[10px] uppercase tracking-widest2 text-muted">
            今日入库
            <Files class="h-3.5 w-3.5" />
          </div>
          <div class="mt-4 font-display text-[38px] leading-none">{{ data.today_inserted }}</div>
          <p class="mt-2 text-[10px] text-muted">经去重后新增的内容</p>
        </div>
        <RouterLink
          to="/admin/reviews"
          class="group p-5 md:p-6"
          :class="data.pending_reviews > 0 ? 'bg-blush/45' : 'bg-canvas'"
        >
          <div class="flex items-center justify-between text-[10px] uppercase tracking-widest2 text-muted">
            待审核
            <ClipboardCheck class="h-3.5 w-3.5" />
          </div>
          <div class="mt-4 flex items-end gap-2">
            <span class="font-display text-[38px] leading-none">{{ data.pending_reviews }}</span>
            <ArrowUpRight v-if="data.pending_reviews" class="mb-1 h-4 w-4 text-muted transition group-hover:text-ink" />
          </div>
          <p class="mt-2 text-[10px] text-muted">需人工确认的重要或社区内容</p>
        </RouterLink>
        <RouterLink
          to="/admin/sources"
          class="group p-5 md:p-6"
          :class="data.source_errors > 0 ? 'bg-[#f6eadf]' : 'bg-canvas'"
        >
          <div class="flex items-center justify-between text-[10px] uppercase tracking-widest2 text-muted">
            来源异常
            <AlertCircle class="h-3.5 w-3.5" />
          </div>
          <div class="mt-4 flex items-end gap-2">
            <span class="font-display text-[38px] leading-none">{{ data.source_errors }}</span>
            <ArrowUpRight v-if="data.source_errors" class="mb-1 h-4 w-4 text-muted transition group-hover:text-ink" />
          </div>
          <p class="mt-2 text-[10px] text-muted">启用中且最近一次采集报错</p>
        </RouterLink>
      </section>

      <div class="mt-6 grid gap-6 lg:grid-cols-[1.5fr_1fr]">
        <section class="border border-line bg-canvas">
          <div class="flex items-center justify-between border-b border-line px-5 py-4">
            <div>
              <div class="eyebrow">Recent Jobs</div>
              <h2 class="mt-1 font-display text-lg">最近采集任务</h2>
            </div>
            <RouterLink to="/admin/jobs" class="inline-flex items-center gap-1 text-[10px] text-muted hover:text-ink">
              全部任务 <ArrowUpRight class="h-3 w-3" />
            </RouterLink>
          </div>
          <div v-if="data.recent_jobs?.length" class="divide-y divide-line">
            <div v-for="job in data.recent_jobs" :key="job.id" class="flex flex-wrap items-center gap-x-4 gap-y-2 px-5 py-3.5">
              <span class="w-14 shrink-0 border px-1.5 py-1 text-center text-[9px]" :class="statusTone[job.status]">
                {{ jobStatusLabels[job.status] || job.status }}
              </span>
              <div class="min-w-0 flex-1">
                <div class="truncate text-[12px]">{{ job.source_name }}</div>
                <div class="mt-0.5 text-[9px] text-muted">
                  {{ formatDateTime(job.created_at) }} · 触发方式 {{ job.trigger_type }}
                </div>
              </div>
              <div class="flex items-center gap-3 text-[10px] text-muted">
                <span>抓取 {{ job.fetched_count }}</span>
                <span class="text-moss">新增 {{ job.inserted_count }}</span>
                <span v-if="job.failed_count" class="text-accent">失败 {{ job.failed_count }}</span>
              </div>
            </div>
          </div>
          <div v-else class="px-5 py-10 text-center text-[11px] text-muted">还没有采集任务</div>
        </section>

        <div class="space-y-6">
          <section class="border border-line bg-canvas p-5">
            <div class="eyebrow">Health</div>
            <div class="mt-2 flex items-end justify-between">
              <h2 class="font-display text-lg">近 7 天采集成功率</h2>
              <span class="font-display text-2xl">{{ Math.round(data.success_rate * 100) }}%</span>
            </div>
            <div class="mt-4 h-1.5 w-full bg-line">
              <div
                class="h-full bg-moss transition-all"
                :style="{ width: `${Math.round(data.success_rate * 100)}%` }"
              />
            </div>
            <p class="mt-3 text-[10px] leading-5 text-muted">
              仅统计已结束的成功 / 失败任务；排队与采集中不计入。来源报错会同步显示在数据源页。
            </p>
          </section>

          <section class="border border-line bg-canvas">
            <div class="border-b border-line px-5 py-4">
              <div class="eyebrow">Shortcuts</div>
              <h2 class="mt-1 font-display text-lg">快捷操作</h2>
            </div>
            <div class="divide-y divide-line">
              <RouterLink to="/admin/contents" class="flex items-center justify-between px-5 py-3.5 text-[12px] transition hover:bg-white/60">
                手动录入公开内容
                <ArrowUpRight class="h-3.5 w-3.5 text-muted" />
              </RouterLink>
              <RouterLink to="/admin/reviews" class="flex items-center justify-between px-5 py-3.5 text-[12px] transition hover:bg-white/60">
                处理待审核内容
                <span v-if="data.pending_reviews" class="tag-accent !py-0.5">{{ data.pending_reviews }}</span>
                <ArrowUpRight v-else class="h-3.5 w-3.5 text-muted" />
              </RouterLink>
              <RouterLink to="/admin/sources" class="flex items-center justify-between px-5 py-3.5 text-[12px] transition hover:bg-white/60">
                配置与启停数据源
                <ArrowUpRight class="h-3.5 w-3.5 text-muted" />
              </RouterLink>
              <RouterLink to="/admin/base" class="flex items-center justify-between px-5 py-3.5 text-[12px] transition hover:bg-white/60">
                更新作品与角色资料
                <ArrowUpRight class="h-3.5 w-3.5 text-muted" />
              </RouterLink>
            </div>
          </section>
        </div>
      </div>

      <p class="mt-6 text-[10px] text-muted">
        概览于 {{ formatDateTime(new Date().toISOString()) }} 读取 · 最近任务更新于
        {{ data.recent_jobs?.[0] ? relativeTime(data.recent_jobs[0].created_at) : '—' }}
      </p>
    </template>
  </div>
</template>
