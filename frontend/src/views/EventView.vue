<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { ArrowLeft, CalendarDays, MapPin } from 'lucide-vue-next'
import { api } from '../api/client'
import type { EventDetail } from '../types'
import { formatDate } from '../lib/format'
import ContentCard from '../components/ContentCard.vue'
import CoverArt from '../components/CoverArt.vue'
import EmptyState from '../components/EmptyState.vue'
import LoadingBlock from '../components/LoadingBlock.vue'

const route = useRoute()
const data = ref<EventDetail | null>(null)
const loading = ref(true)
const error = ref('')
const eventId = computed(() => String(route.params.id || ''))

async function load() {
  loading.value = true
  error.value = ''
  try {
    data.value = await api.get<EventDetail>(`/events/${eventId.value}`)
  } catch (err) {
    error.value = err instanceof Error ? err.message : '事件加载失败'
  } finally {
    loading.value = false
  }
}

onMounted(load)
watch(eventId, (next, prev) => {
  if (next && next !== prev) load()
})
</script>

<template>
  <div class="shell pb-16 pt-6 md:pb-24 md:pt-10">
    <RouterLink to="/discover" class="mb-6 inline-flex items-center gap-2 text-xs text-muted transition hover:text-ink">
      <ArrowLeft class="h-3.5 w-3.5" />
      返回发现
    </RouterLink>

    <LoadingBlock v-if="loading" :rows="4" />
    <EmptyState v-else-if="error" title="事件不可用" :description="error" action-label="回到发现" to="/discover" />

    <template v-else-if="data">
      <header class="grid gap-8 border-b border-line pb-8 md:grid-cols-[1fr_320px] md:items-end md:pb-10">
        <div>
          <div class="eyebrow mb-3">Event Archive</div>
          <h1 class="display-title text-[34px] leading-[1.12] md:text-[48px]">{{ data.event.title }}</h1>
          <p v-if="data.event.summary" class="mt-4 max-w-2xl text-sm leading-7 text-muted">{{ data.event.summary }}</p>
          <div class="mt-5 flex flex-wrap items-center gap-x-5 gap-y-2 text-[11px] text-muted">
            <span v-if="data.event.event_date" class="inline-flex items-center gap-1.5">
              <CalendarDays class="h-3.5 w-3.5" />
              {{ formatDate(data.event.event_date) }}
            </span>
            <span v-if="data.event.location" class="inline-flex items-center gap-1.5">
              <MapPin class="h-3.5 w-3.5" />
              {{ data.event.location }}
            </span>
            <span>{{ data.event.content_count }} 条关联内容</span>
            <span class="border border-line px-1.5 py-0.5 text-[9px]">
              {{ data.event.status === 'published' ? '已公开' : data.event.status === 'archived' ? '已归档' : '草稿' }}
            </span>
          </div>
        </div>
        <CoverArt
          :title="data.event.title"
          subtitle="Xiwei Space Event"
          :src="data.event.cover_url"
          aspect="wide"
        />
      </header>

      <section class="mt-8 md:mt-10">
        <div class="mb-4 flex items-end justify-between gap-4">
          <div>
            <div class="eyebrow mb-1.5">Related Contents</div>
            <h2 class="font-display text-2xl tracking-tight">相关内容</h2>
          </div>
          <span class="text-[10px] text-muted">按时间倒序</span>
        </div>
        <div v-if="data.contents.length" class="border-t border-line md:grid md:grid-cols-2 md:gap-x-10">
          <ContentCard v-for="item in data.contents" :key="item.id" :content="item" />
        </div>
        <EmptyState
          v-else
          compact
          title="暂无关联内容"
          description="该事件还没有已公开的关联内容。"
        />
      </section>
    </template>
  </div>
</template>
