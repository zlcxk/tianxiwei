<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { ArrowLeft, ArrowUpRight, BadgeCheck, ExternalLink, ShieldCheck } from 'lucide-vue-next'
import { api } from '../api/client'
import type { Content, Paged } from '../types'
import {
  contentDateLabel,
  contentTypeLabels,
  formatDate,
  formatDateTime,
  precisionLabels,
  sourceTypeLabels,
  trustLabels,
} from '../lib/format'
import ContentCard from '../components/ContentCard.vue'
import CoverArt from '../components/CoverArt.vue'
import EmptyState from '../components/EmptyState.vue'
import FavoriteButton from '../components/FavoriteButton.vue'
import LoadingBlock from '../components/LoadingBlock.vue'
import SceneIcon from '../components/SceneIcon.vue'

const route = useRoute()
const content = ref<Content | null>(null)
const related = ref<Content[]>([])
const loading = ref(true)
const error = ref('')

const contentId = computed(() => String(route.params.id || ''))
const isInternal = computed(() => content.value?.source_type === 'internal' || content.value?.content_type === 'internal')
const publicationLabel = computed(() => (content.value ? contentDateLabel(content.value) : '—'))
const coverMeta = computed(() => {
  if (!content.value?.published_at) return '时间未知'
  return formatDate(content.value.published_at)
})
const safeSourceURL = computed(() => {
  const raw = content.value?.source_url
  if (!raw) return ''
  try {
    const parsed = new URL(raw)
    return parsed.protocol === 'http:' || parsed.protocol === 'https:' ? parsed.toString() : ''
  } catch {
    return ''
  }
})
const sourceHost = computed(() => {
  if (!safeSourceURL.value) return ''
  try {
    return new URL(safeSourceURL.value).hostname.replace('www.', '')
  } catch {
    return ''
  }
})

async function loadRelated(item: Content) {
  const params = new URLSearchParams({ sort: 'latest', page_size: '4', page: '1' })
  if (item.works?.[0]) params.set('work_id', item.works[0].id)
  else if (item.characters?.[0]) params.set('character_id', item.characters[0].id)
  else return
  try {
    const data = await api.get<Paged<Content>>(`/contents?${params.toString()}`)
    related.value = data.items.filter((c) => c.id !== item.id).slice(0, 3)
  } catch {
    related.value = []
  }
}

async function load() {
  loading.value = true
  error.value = ''
  related.value = []
  try {
    const item = await api.get<Content>(`/contents/${contentId.value}`)
    content.value = item
    loadRelated(item)
  } catch (err) {
    error.value = err instanceof Error ? err.message : '内容加载失败'
  } finally {
    loading.value = false
  }
}

onMounted(load)
watch(contentId, (next, prev) => {
  if (next && next !== prev) load()
})
</script>

<template>
  <div class="shell pb-16 pt-6 md:pb-24 md:pt-10">
    <RouterLink to="/discover" class="mb-6 inline-flex items-center gap-2 text-xs text-muted transition hover:text-ink">
      <ArrowLeft class="h-3.5 w-3.5" />
      返回发现
    </RouterLink>

    <div v-if="loading" class="grid gap-10 md:grid-cols-[1.5fr_1fr]">
      <div>
        <div class="skeleton h-3 w-24" />
        <div class="skeleton mt-6 h-10 w-full" />
        <div class="skeleton mt-3 h-10 w-3/4" />
        <div class="skeleton mt-8 h-64 w-full" />
      </div>
      <div class="skeleton h-72 w-full" />
    </div>

    <EmptyState
      v-else-if="error"
      title="内容不可用"
      :description="error"
      action-label="回到发现"
      to="/discover"
    />

    <article v-else-if="content">
      <header class="grid gap-8 border-b border-line pb-8 md:grid-cols-[1.5fr_1fr] md:gap-12 md:pb-12">
        <div>
          <div class="flex flex-wrap items-center gap-x-2.5 gap-y-2 text-[10px] uppercase tracking-[0.14em] text-muted">
            <span class="tag-accent !border-accent/20">{{ contentTypeLabels[content.content_type] || '资讯' }}</span>
            <span>{{ content.source_name }}</span>
            <span v-if="content.origin_platform" class="text-muted/80">{{ content.origin_platform }}</span>
            <span class="h-2.5 w-px bg-line" />
            <time>{{ publicationLabel }}</time>
            <span v-if="content.is_official" class="inline-flex items-center gap-1 text-[#9a6c72]">
              <BadgeCheck class="h-3 w-3" />官方来源
            </span>
            <span
              v-if="content.published_precision !== 'verified'"
              class="border border-accent/25 bg-blush/30 px-1.5 py-0.5 text-[9px] tracking-normal text-[#9a6c72]"
            >
              {{ precisionLabels[content.published_precision] }}
            </span>
          </div>
          <h1 class="display-title mt-5 text-[32px] leading-[1.14] md:text-[48px]">
            {{ content.title }}
          </h1>
          <p v-if="content.summary" class="mt-5 max-w-2xl text-[15px] leading-8 text-ink/70">
            {{ content.summary }}
          </p>
          <div class="mt-6 flex flex-wrap items-center gap-3">
            <a
              v-if="safeSourceURL"
              :href="safeSourceURL"
              target="_blank"
              rel="noopener noreferrer nofollow"
              class="btn-primary !min-h-10 text-xs"
            >
              {{ isInternal ? '查看参考资料' : '阅读原文' }}
              <ExternalLink class="h-3.5 w-3.5" />
            </a>
            <FavoriteButton
              v-model="content.favorited"
              target-type="content"
              :target-id="content.id"
              class="btn-ghost !min-h-10 !px-4 text-xs"
            />
          </div>
        </div>
        <div class="relative">
          <CoverArt
            :title="content.title"
            :subtitle="content.source_name"
            :meta="coverMeta"
            :src="content.cover_url"
            aspect="wide"
            class="md:h-full"
          />
        </div>
      </header>

      <div class="grid gap-10 pt-8 md:grid-cols-[1.5fr_1fr] md:gap-12 md:pt-12">
        <div>
          <div class="section-label mb-5">内容摘要</div>
          <div class="prose-editorial">
            <p v-if="content.summary">{{ content.summary }}</p>
            <p v-else class="text-muted">该条目暂无摘要，请前往来源页面查看完整内容。</p>
          </div>

          <div
            v-if="content.search_task_id"
            class="mt-8 flex items-start gap-3 border border-accent/25 bg-blush/25 p-4"
          >
            <ShieldCheck class="mt-0.5 h-4 w-4 shrink-0 text-[#9a6c72]" />
            <div>
              <p class="text-xs font-medium">该条目由搜索发现</p>
              <p class="mt-1 text-[11px] leading-5 text-muted">
                <template v-if="content.metadata_status === 'ok'">
                  站内展示来源页面摘要，完整内容请前往原文核对。
                </template>
                <template v-else>
                  来源页面未能完整读取，站内展示的是搜索摘要而非原文全文，请务必前往原始链接核对。
                </template>
              </p>
            </div>
          </div>

          <div v-if="content.review_status === 'approved'" class="mt-8 flex items-start gap-3 border border-line bg-white/50 p-4">
            <ShieldCheck class="mt-0.5 h-4 w-4 shrink-0 text-moss" />
            <div>
              <p class="text-xs font-medium">该内容已通过人工审核</p>
              <p class="mt-1 text-[11px] leading-5 text-muted">
                审核仅针对来源可信度与分类准确性；站内保留摘要，完整内容请以原文为准。
              </p>
            </div>
          </div>

          <div v-if="content.scenes.length || content.works.length || content.characters.length || content.events?.length" class="mt-8">
            <div class="section-label mb-4">关联档案</div>
            <div class="flex flex-wrap gap-2">
              <RouterLink
                v-for="event in content.events || []"
                :key="event.id"
                :to="{ name: 'event', params: { id: event.id } }"
                class="inline-flex items-center gap-1.5 border border-accent/30 bg-blush/30 px-3 py-1.5 text-[11px] text-ink transition hover:border-accent/60"
              >
                {{ event.title }}
                <ArrowUpRight class="h-3 w-3 text-muted" />
              </RouterLink>
              <RouterLink
                v-for="scene in content.scenes"
                :key="scene.id"
                :to="{ name: 'discover', query: { scene: scene.code } }"
                class="inline-flex items-center gap-1.5 border border-line bg-white/50 px-3 py-1.5 text-[11px] text-muted transition hover:border-ink/25 hover:text-ink"
              >
                <SceneIcon :code="scene.code" class="h-3 w-3" />
                {{ scene.name }}
              </RouterLink>
              <RouterLink
                v-for="work in content.works"
                :key="work.id"
                :to="{ name: 'work', params: { id: work.id } }"
                class="tag hover:border-ink/25 hover:text-ink"
              >
                {{ work.title }}
              </RouterLink>
              <RouterLink
                v-for="character in content.characters"
                :key="character.id"
                :to="{ name: 'character', params: { id: character.id } }"
                class="tag hover:border-ink/25 hover:text-ink"
              >
                {{ character.name }}
              </RouterLink>
            </div>
          </div>
        </div>

        <aside class="h-fit border border-line bg-white/40">
          <div class="border-b border-line px-5 py-4">
            <div class="eyebrow">Source & Traceability</div>
            <h2 class="mt-1.5 font-display text-lg">来源信息</h2>
          </div>
          <dl class="divide-y divide-line text-xs">
            <div class="flex items-start justify-between gap-4 px-5 py-3.5">
              <dt class="shrink-0 text-muted">来源媒体</dt>
              <dd class="text-right">{{ content.source_name || '—' }}</dd>
            </div>
            <div v-if="content.origin_platform" class="flex items-start justify-between gap-4 px-5 py-3.5">
              <dt class="shrink-0 text-muted">来源平台</dt>
              <dd class="text-right">{{ content.origin_platform }}</dd>
            </div>
            <div class="flex items-start justify-between gap-4 px-5 py-3.5">
              <dt class="shrink-0 text-muted">作者 / 账号</dt>
              <dd class="text-right">{{ content.source_author || '—' }}</dd>
            </div>
            <div class="flex items-start justify-between gap-4 px-5 py-3.5">
              <dt class="shrink-0 text-muted">来源类型</dt>
              <dd class="text-right">
                {{ sourceTypeLabels[content.origin_type || content.source_type] || trustLabels[content.trust_level] || content.trust_level }}
              </dd>
            </div>
            <div class="flex items-start justify-between gap-4 px-5 py-3.5">
              <dt class="shrink-0 text-muted">发布时间</dt>
              <dd class="text-right">
                <div>{{ publicationLabel }}</div>
                <div class="mt-0.5 text-[10px] text-muted">{{ precisionLabels[content.published_precision] }}</div>
              </dd>
            </div>
            <div class="flex items-start justify-between gap-4 px-5 py-3.5">
              <dt class="shrink-0 text-muted">发现时间</dt>
              <dd class="text-right">{{ formatDateTime(content.discovered_at) }}</dd>
            </div>
            <div class="flex items-start justify-between gap-4 px-5 py-3.5">
              <dt class="shrink-0 text-muted">收录时间</dt>
              <dd class="text-right">{{ formatDateTime(content.collected_at) }}</dd>
            </div>
            <div v-if="sourceHost" class="flex items-start justify-between gap-4 px-5 py-3.5">
              <dt class="shrink-0 text-muted">原文域名</dt>
              <dd class="text-right">{{ sourceHost }}</dd>
            </div>
          </dl>
          <a
            v-if="safeSourceURL"
            :href="safeSourceURL"
            target="_blank"
            rel="noopener noreferrer nofollow"
            class="flex items-center justify-between gap-2 border-t border-line px-5 py-3.5 text-[11px] text-muted transition hover:text-ink"
          >
            前往来源核对
            <ArrowUpRight class="h-3.5 w-3.5" />
          </a>
        </aside>
      </div>

      <section v-if="related.length" class="mt-14 border-t border-line pt-8 md:mt-20">
        <div class="mb-4 flex items-end justify-between">
          <div>
            <div class="eyebrow mb-1.5">More Like This</div>
            <h2 class="font-display text-2xl tracking-tight">相关内容</h2>
          </div>
          <RouterLink
            v-if="content.works?.[0]"
            :to="{ name: 'work', params: { id: content.works[0].id } }"
            class="inline-flex items-center gap-1 text-xs text-muted hover:text-ink"
          >
            作品主页 <ArrowUpRight class="h-3 w-3" />
          </RouterLink>
        </div>
        <div class="grid md:grid-cols-2 md:gap-x-10">
          <ContentCard v-for="item in related" :key="item.id" :content="item" :show-summary="false" />
        </div>
      </section>
    </article>
  </div>
</template>
