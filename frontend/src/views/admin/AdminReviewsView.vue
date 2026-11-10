<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { Check, ExternalLink, RefreshCw, ShieldAlert, X } from 'lucide-vue-next'
import { api } from '../../api/client'
import type { Content } from '../../types'
import { contentTypeLabels, formatDateTime, trustLabels } from '../../lib/format'
import { toast } from '../../stores/toast'
import AdminPageHeader from './AdminPageHeader.vue'
import EmptyState from '../../components/EmptyState.vue'
import LoadingBlock from '../../components/LoadingBlock.vue'
import Modal from '../../components/Modal.vue'

const items = ref<Content[]>([])
const loading = ref(true)
const error = ref('')
const acting = ref('')
const rejectTarget = ref<Content | null>(null)
const rejectReason = ref('')
const rejecting = ref(false)

async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await api.get<{ items: Content[] }>('/admin/reviews')
    items.value = data.items
  } catch (err) {
    error.value = err instanceof Error ? err.message : '待审核列表加载失败'
  } finally {
    loading.value = false
  }
}

async function approve(item: Content) {
  if (acting.value) return
  acting.value = item.id
  try {
    await api.post(`/admin/reviews/${item.id}/approve`)
    items.value = items.value.filter((i) => i.id !== item.id)
    toast.success('已通过审核并公开')
  } catch (err) {
    toast.error(err instanceof Error ? err.message : '审核操作失败')
  } finally {
    acting.value = ''
  }
}

function openReject(item: Content) {
  rejectTarget.value = item
  rejectReason.value = ''
}

async function reject() {
  if (!rejectTarget.value || rejecting.value) return
  rejecting.value = true
  try {
    await api.post(`/admin/reviews/${rejectTarget.value.id}/reject`, {
      reason: rejectReason.value.trim() || '未通过审核',
    })
    items.value = items.value.filter((i) => i.id !== rejectTarget.value?.id)
    toast.success('已拒绝该内容')
    rejectTarget.value = null
  } catch (err) {
    toast.error(err instanceof Error ? err.message : '审核操作失败')
  } finally {
    rejecting.value = false
  }
}

onMounted(load)
</script>

<template>
  <div>
    <AdminPageHeader
      eyebrow="Console / Review Queue"
      title="待审核"
      description="重要信息、社区来源与未分类内容会进入人工审核。通过后才会在公开页面展示。"
    >
      <template #actions>
        <span class="text-[11px] text-muted">{{ items.length }} 条待处理</span>
        <button class="btn-ghost !min-h-9 !px-3 text-[11px]" :disabled="loading" @click="load">
          <RefreshCw class="h-3.5 w-3.5" :class="loading ? 'animate-spin' : ''" />
          刷新
        </button>
      </template>
    </AdminPageHeader>

    <LoadingBlock v-if="loading" :rows="5" />

    <EmptyState v-else-if="error" title="审核列表加载失败" :description="error" action-label="重新加载" @action="load" />

    <EmptyState
      v-else-if="!items.length"
      :icon="Check"
      title="审核队列已清空"
      description="当前没有需要人工确认的内容。新的重要内容进入后会出现在这里。"
    />

    <div v-else class="space-y-4">
      <article v-for="item in items" :key="item.id" class="border border-line bg-canvas">
        <div class="flex flex-col gap-3 border-b border-line px-5 py-4 sm:flex-row sm:items-start sm:justify-between">
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2 text-[9px] uppercase tracking-[0.14em] text-muted">
              <span class="tag-accent !py-0.5">{{ contentTypeLabels[item.content_type] || item.content_type }}</span>
              <span>{{ item.source_name }}</span>
              <span class="h-2.5 w-px bg-line" />
              <span>{{ trustLabels[item.trust_level] || item.trust_level }}</span>
              <span class="h-2.5 w-px bg-line" />
              <span>{{ formatDateTime(item.published_at) }}</span>
            </div>
            <h2 class="mt-2 font-display text-xl leading-snug tracking-tight">{{ item.title }}</h2>
          </div>
          <a
            v-if="item.source_url"
            :href="item.source_url"
            target="_blank"
            rel="noopener noreferrer nofollow"
            class="inline-flex shrink-0 items-center gap-1 text-[10px] text-muted underline underline-offset-4 hover:text-ink"
          >
            核对原文
            <ExternalLink class="h-3 w-3" />
          </a>
        </div>

        <div class="px-5 py-4">
          <p v-if="item.summary" class="line-clamp-3 text-[12px] leading-6 text-ink/70">{{ item.summary }}</p>
          <div class="mt-3 flex flex-wrap gap-1.5">
            <span v-for="scene in item.scenes" :key="scene.id" class="tag !py-0.5">{{ scene.name }}</span>
            <span v-for="work in item.works" :key="work.id" class="tag !py-0.5">《{{ work.title }}》</span>
            <span v-for="character in item.characters" :key="character.id" class="tag !py-0.5">{{ character.name }}</span>
          </div>
          <div v-if="item.review_reason" class="mt-3 flex items-start gap-2 border border-line bg-white/50 px-3 py-2.5">
            <ShieldAlert class="mt-0.5 h-3.5 w-3.5 shrink-0 text-[#a67c46]" />
            <p class="text-[10px] leading-5 text-muted">进入审核原因：{{ item.review_reason }}</p>
          </div>
        </div>

        <div class="flex flex-wrap items-center justify-end gap-2 border-t border-line bg-white/30 px-5 py-3">
          <button
            class="btn-ghost !min-h-8 !px-3 text-[11px] text-accent"
            :disabled="!!acting"
            @click="openReject(item)"
          >
            <X class="h-3 w-3" />
            拒绝
          </button>
          <button
            class="btn-primary !min-h-8 !px-3.5 text-[11px]"
            :disabled="!!acting"
            @click="approve(item)"
          >
            <span v-if="acting === item.id" class="h-3 w-3 animate-spin rounded-full border border-white/40 border-t-white" />
            <Check v-else class="h-3 w-3" />
            通过并公开
          </button>
        </div>
      </article>
    </div>

    <Modal :open="!!rejectTarget" title="拒绝内容" description="拒绝后内容不会公开；拒绝原因会记录在审核日志中。" @close="rejectTarget = null">
      <div v-if="rejectTarget">
        <p class="mb-4 border border-line bg-white/60 px-3 py-2 text-[11px] leading-5 text-muted">
          {{ rejectTarget.title }}
        </p>
        <label for="reject-reason" class="field-label">拒绝原因</label>
        <textarea
          id="reject-reason"
          v-model="rejectReason"
          rows="4"
          class="field resize-y"
          placeholder="例如：来源无法核实 / 内容与艺人无关 / 重复内容"
        />
      </div>
      <template #footer>
        <button class="btn-ghost !min-h-9 text-xs" @click="rejectTarget = null">取消</button>
        <button class="btn-primary !min-h-9 bg-accent text-xs hover:bg-[#b97782]" :disabled="rejecting" @click="reject">
          {{ rejecting ? '提交中' : '确认拒绝' }}
        </button>
      </template>
    </Modal>
  </div>
</template>
