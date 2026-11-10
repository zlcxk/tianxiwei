<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { ArrowLeft, ArrowUpRight, CalendarDays, ExternalLink, Film, Play } from 'lucide-vue-next'
import { api } from '../api/client'
import type { WorkDetail } from '../types'
import { formatDate, workStatusLabels, workTypeLabels } from '../lib/format'
import ContentCard from '../components/ContentCard.vue'
import CoverArt from '../components/CoverArt.vue'
import EmptyState from '../components/EmptyState.vue'
import FavoriteButton from '../components/FavoriteButton.vue'
import LoadingBlock from '../components/LoadingBlock.vue'

const route = useRoute()
const detail = ref<WorkDetail | null>(null)
const loading = ref(true)
const error = ref('')

const workId = computed(() => String(route.params.id || ''))
const work = computed(() => detail.value?.work || null)
const characters = computed(() => work.value?.characters || [])

async function load() {
  loading.value = true
  error.value = ''
  try {
    detail.value = await api.get<WorkDetail>(`/works/${workId.value}`)
  } catch (err) {
    error.value = err instanceof Error ? err.message : '作品加载失败'
  } finally {
    loading.value = false
  }
}

onMounted(load)
watch(workId, (next, prev) => {
  if (next && next !== prev) load()
})
</script>

<template>
  <div class="shell pb-16 pt-6 md:pb-24 md:pt-10">
    <RouterLink to="/works" class="mb-6 inline-flex items-center gap-2 text-xs text-muted transition hover:text-ink">
      <ArrowLeft class="h-3.5 w-3.5" />
      返回作品档案
    </RouterLink>

    <div v-if="loading" class="grid gap-8 md:grid-cols-[300px_1fr]">
      <div class="skeleton aspect-[3/4] w-full max-w-[300px]" />
      <div class="space-y-4">
        <div class="skeleton h-3 w-24" />
        <div class="skeleton h-12 w-3/4" />
        <div class="skeleton h-24 w-full" />
      </div>
    </div>

    <EmptyState v-else-if="error" title="作品不可用" :description="error" action-label="回到作品档案" to="/works" />

    <template v-else-if="work">
      <header class="grid gap-8 border-b border-line pb-10 md:grid-cols-[280px_1fr] md:gap-12 md:pb-14">
        <div class="max-w-[280px]">
          <CoverArt
            :title="work.title"
            :subtitle="workTypeLabels[work.type] || '作品'"
            :meta="formatDate(work.release_date)"
            :src="work.cover"
            aspect="portrait"
          />
        </div>
        <div class="flex flex-col justify-between">
          <div>
            <div class="flex flex-wrap items-center gap-2 text-[10px] uppercase tracking-[0.14em] text-muted">
              <span class="tag">{{ workTypeLabels[work.type] || '作品' }}</span>
              <span>{{ workStatusLabels[work.status] || '资料' }}</span>
              <span class="h-2.5 w-px bg-line" />
              <span>{{ formatDate(work.release_date) }}</span>
            </div>
            <h1 class="display-title mt-5 text-[38px] md:text-[58px]">{{ work.title }}</h1>
            <p v-if="work.description" class="mt-5 max-w-2xl text-[14px] leading-8 text-ink/70">{{ work.description }}</p>
            <div class="mt-6 flex flex-wrap gap-x-8 gap-y-3 text-xs">
              <div v-if="work.platform">
                <span class="text-[10px] uppercase tracking-widest text-muted">播出平台</span>
                <p class="mt-1">{{ work.platform }}</p>
              </div>
              <div v-if="work.release_date">
                <span class="text-[10px] uppercase tracking-widest text-muted">首播 / 上线</span>
                <p class="mt-1">{{ formatDate(work.release_date) }}</p>
              </div>
              <div v-if="characters.length">
                <span class="text-[10px] uppercase tracking-widest text-muted">饰演角色</span>
                <p class="mt-1">{{ characters.map((c) => c.name).join(' / ') }}</p>
              </div>
            </div>
          </div>
          <div class="mt-8 flex flex-wrap items-center gap-3">
            <a
              v-if="work.official_url"
              :href="work.official_url"
              target="_blank"
              rel="noopener noreferrer nofollow"
              class="btn-primary !min-h-10 text-xs"
            >
              查看公开资料
              <ExternalLink class="h-3.5 w-3.5" />
            </a>
            <FavoriteButton
              v-model="work.favorited"
              target-type="work"
              :target-id="work.id"
              class="btn-ghost !min-h-10 !px-4 text-xs"
            />
          </div>
        </div>
      </header>

      <section v-if="characters.length" class="pt-10 md:pt-14">
        <div class="mb-5 flex items-end justify-between">
          <div>
            <div class="eyebrow mb-2">Characters</div>
            <h2 class="font-display text-2xl tracking-tight md:text-[28px]">角色档案</h2>
          </div>
          <span class="text-[11px] text-muted">共 {{ characters.length }} 个角色</span>
        </div>
        <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <RouterLink
            v-for="character in characters"
            :key="character.id"
            :to="{ name: 'character', params: { id: character.id } }"
            class="group flex items-start gap-4 border border-line bg-white/40 p-4 transition hover:border-ink/25 hover:bg-white/70"
          >
            <span class="flex h-14 w-14 shrink-0 items-center justify-center border border-line bg-blush/40 font-display text-2xl text-[#8f5e66]">
              {{ character.name.slice(0, 1) }}
            </span>
            <div class="min-w-0 flex-1">
              <div class="flex items-center justify-between gap-2">
                <h3 class="font-display text-lg tracking-tight transition group-hover:text-accent">{{ character.name }}</h3>
                <ArrowUpRight class="h-3.5 w-3.5 shrink-0 text-muted opacity-0 transition group-hover:opacity-100" />
              </div>
              <p class="mt-1 line-clamp-2 text-[11px] leading-5 text-muted">{{ character.description || '角色资料整理中' }}</p>
            </div>
          </RouterLink>
        </div>
      </section>

      <section class="pt-12 md:pt-16">
        <div class="mb-5 flex items-end justify-between">
          <div>
            <div class="eyebrow mb-2">Related Stories</div>
            <h2 class="font-display text-2xl tracking-tight md:text-[28px]">相关动态</h2>
          </div>
          <RouterLink
            :to="{ name: 'discover', query: { work_id: work.id } }"
            class="inline-flex items-center gap-1 text-xs text-muted hover:text-ink"
          >
            筛选全部 <ArrowUpRight class="h-3 w-3" />
          </RouterLink>
        </div>
        <div v-if="detail?.contents?.length" class="grid border-t border-line md:grid-cols-2 md:gap-x-10">
          <ContentCard v-for="item in detail.contents" :key="item.id" :content="item" />
        </div>
        <EmptyState
          v-else
          compact
          :icon="Film"
          title="暂无相关动态"
          description="关于这部作品的公开内容会在审核通过后出现在这里。"
        />
      </section>
    </template>
  </div>
</template>
