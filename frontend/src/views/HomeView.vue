<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { ArrowRight, ArrowUpRight, RefreshCw, Sparkles } from 'lucide-vue-next'
import { api } from '../api/client'
import type { Content, EditorialPick, HomeData } from '../types'
import { auth } from '../stores/auth'
import { contentTimeLabel, formatYear, relativeTime, todayLabel, workStatusLabels, workTypeLabels } from '../lib/format'
import ContentCard from '../components/ContentCard.vue'
import CoverArt from '../components/CoverArt.vue'
import EmptyState from '../components/EmptyState.vue'
import LoadingBlock from '../components/LoadingBlock.vue'
import SceneIcon from '../components/SceneIcon.vue'

const data = ref<HomeData | null>(null)
const loading = ref(true)
const error = ref('')

const latest = computed(() => data.value?.latest || [])
const forYou = computed(() => data.value?.for_you || [])
const archive = computed(() => data.value?.archive || [])
const works = computed(() => data.value?.works?.slice(0, 8) || [])
const activeScenes = computed(() => data.value?.scenes?.filter((s) => s.code !== 'all') || [])
const profile = computed(() => (data.value?.artist?.profile_data || {}) as Record<string, string>)

const picks = computed(() =>
  (data.value?.picks || []).filter(
    (pick): pick is EditorialPick & { content: Content } => Boolean(pick.content),
  ),
)
const spotlight = computed(() => {
  const pick = picks.value[0]
  if (!pick) return null
  return {
    contentId: pick.content.id,
    title: pick.title || pick.content.title,
    description: pick.description || pick.content.summary,
    cover: pick.cover_url || pick.content.cover_url,
    source: pick.content.source_name,
    time: contentTimeLabel(pick.content),
    scene: pick.content.scenes?.[0]?.name || '',
  }
})
const morePicks = computed(() =>
  picks.value.slice(1, 8).map((pick) => ({
    id: pick.id,
    contentId: pick.content.id,
    title: pick.title || pick.content.title,
    description: pick.description || pick.content.summary,
    cover: pick.cover_url || pick.content.cover_url,
    source: pick.content.source_name,
    time: contentTimeLabel(pick.content),
  })),
)

async function load() {
  loading.value = true
  error.value = ''
  try {
    data.value = await api.get<HomeData>('/home')
  } catch (err) {
    error.value = err instanceof Error ? err.message : '首页加载失败'
  } finally {
    loading.value = false
  }
}

onMounted(load)
watch(() => auth.user?.id, load)
</script>

<template>
  <div>
    <!-- Editorial hero, drawn entirely in CSS. No third-party imagery. -->
    <section class="relative overflow-hidden border-b border-line">
      <div class="hero-canvas">
        <div class="shell relative z-10 grid gap-8 py-12 md:grid-cols-[1.15fr_.85fr] md:items-end md:py-20 lg:py-24">
          <div>
            <div class="flex items-center gap-2.5 text-[10px] uppercase tracking-widest2 text-ink/55">
              <span class="h-1.5 w-1.5 rounded-full bg-accent" />
              {{ todayLabel() }}
            </div>
            <h1 class="display-title mt-6 text-[52px] leading-[0.94] md:text-[82px] lg:text-[96px]">
              田曦薇
            </h1>
            <p class="mt-4 font-display text-xl tracking-tight text-ink/75 md:text-2xl">
              把关于她的每一天，收进一个安静的空间。
            </p>
            <p class="mt-4 max-w-md text-[13px] leading-7 text-ink/55">
              {{ data?.artist?.bio || '演员田曦薇的专属信息档案：作品、角色、公开动态与经核验的资料。' }}
            </p>
            <div class="mt-7 flex flex-wrap items-center gap-3">
              <RouterLink to="/discover" class="btn-primary">
                开始发现
                <ArrowRight class="h-4 w-4" />
              </RouterLink>
              <RouterLink to="/works" class="btn-ghost">作品档案</RouterLink>
            </div>
            <RouterLink
              v-if="spotlight"
              :to="{ name: 'content', params: { id: spotlight.contentId } }"
              class="group mt-6 inline-flex max-w-full items-center gap-2 text-[11px] text-ink/60 transition hover:text-ink"
            >
              <span class="shrink-0 text-[9px] uppercase tracking-widest text-ink/40">本期精选</span>
              <span class="truncate">{{ spotlight.title }}</span>
              <ArrowUpRight class="h-3 w-3 shrink-0 transition group-hover:translate-x-0.5 group-hover:-translate-y-0.5" />
            </RouterLink>
          </div>

          <div class="relative hidden md:block">
            <div class="relative ml-auto w-full max-w-[330px] border border-ink/10 bg-white/35 p-5 backdrop-blur-sm">
              <div class="flex items-start justify-between">
                <span class="text-[9px] uppercase tracking-[0.2em] text-ink/45">Volume 01 / 2026</span>
                <span class="vertical-text font-display text-[11px] tracking-[0.3em] text-ink/40">TIAN XIWEI</span>
              </div>
              <div class="mt-10 border-t border-ink/10 pt-4">
                <div class="grid grid-cols-2 gap-x-4 gap-y-4 text-[11px]">
                  <div>
                    <div class="text-[9px] uppercase tracking-widest text-ink/40">出生</div>
                    <div class="mt-1 text-ink/75">{{ profile.birth_date || '1997-10-14' }}</div>
                  </div>
                  <div>
                    <div class="text-[9px] uppercase tracking-widest text-ink/40">出生地</div>
                    <div class="mt-1 text-ink/75">{{ profile.birth_place || '重庆' }}</div>
                  </div>
                  <div>
                    <div class="text-[9px] uppercase tracking-widest text-ink/40">毕业院校</div>
                    <div class="mt-1 text-ink/75">{{ profile.education || '上海戏剧学院' }}</div>
                  </div>
                  <div>
                    <div class="text-[9px] uppercase tracking-widest text-ink/40">职业</div>
                    <div class="mt-1 text-ink/75">{{ profile.profession || '演员' }}</div>
                  </div>
                </div>
              </div>
              <div class="mt-8 flex items-end justify-between">
                <span class="font-display text-5xl leading-none text-ink/20">曦</span>
                <span class="text-[9px] leading-5 tracking-wide text-ink/40">资料经人工核验<br />来源可追溯</span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div class="border-t border-line bg-canvas/80">
        <div class="shell flex flex-wrap items-center gap-x-8 gap-y-3 py-3.5 text-[11px] text-muted">
          <span class="inline-flex items-center gap-1.5">
            <span class="h-1 w-1 rounded-full bg-moss" />
            今日新增 {{ data?.brief?.new_count ?? 0 }} 条
          </span>
          <span class="hidden h-3 w-px bg-line sm:block" />
          <span>最近更新 {{ data?.brief?.last_updated_at ? relativeTime(data.brief.last_updated_at) : '等待首轮采集' }}</span>
          <span class="hidden h-3 w-px bg-line sm:block" />
          <span>公开来源 · 摘要呈现 · 原文可溯</span>
          <RouterLink to="/discover" class="ml-auto inline-flex items-center gap-1 hover:text-ink">
            全部动态 <ArrowUpRight class="h-3 w-3" />
          </RouterLink>
        </div>
      </div>
    </section>

    <div v-if="loading" class="shell py-10 md:py-14">
      <LoadingBlock :rows="4" />
    </div>

    <div v-else-if="error" class="shell py-16">
      <EmptyState title="暂时没能打开首页" :description="error" action-label="重新加载" @action="load" />
    </div>

    <template v-else-if="data">
      <!-- Scene selector -->
      <section v-if="activeScenes.length" class="shell pt-10 md:pt-14">
        <div class="flex items-end justify-between gap-4">
          <div>
            <div class="eyebrow mb-2">Today's Mood</div>
            <h2 class="font-display text-2xl tracking-tight md:text-[28px]">今天想看点什么？</h2>
          </div>
          <RouterLink to="/profile" class="hidden text-xs text-muted transition hover:text-ink sm:inline-flex sm:items-center sm:gap-1">
            调整兴趣 <ArrowUpRight class="h-3 w-3" />
          </RouterLink>
        </div>
        <div class="no-scrollbar -mx-5 mt-5 flex gap-2.5 overflow-x-auto px-5 pb-1 md:mx-0 md:flex-wrap md:px-0">
          <RouterLink
            v-for="scene in activeScenes"
            :key="scene.id"
            :to="{ name: 'discover', query: { scene: scene.code } }"
            class="group inline-flex shrink-0 items-center gap-2 border px-4 py-2.5 text-xs transition"
            :class="scene.is_default
              ? 'border-accent/40 bg-blush/40 text-ink'
              : 'border-line bg-white/50 text-ink/70 hover:border-ink/25 hover:text-ink'"
          >
            <SceneIcon :code="scene.code" class="h-3.5 w-3.5" />
            {{ scene.name }}
            <span v-if="scene.is_default" class="text-[9px] uppercase tracking-widest text-[#9a6c72]">默认</span>
          </RouterLink>
          <RouterLink
            to="/discover"
            class="inline-flex shrink-0 items-center gap-2 border border-dashed border-line px-4 py-2.5 text-xs text-muted transition hover:border-ink/25 hover:text-ink"
          >
            全部内容
            <ArrowRight class="h-3.5 w-3.5" />
          </RouterLink>
        </div>
      </section>

      <!-- Editorial picks -->
      <section v-if="spotlight" class="shell pt-10 md:pt-16">
        <div class="mb-4 flex flex-wrap items-end justify-between gap-3">
          <div>
            <div class="eyebrow mb-2">Editor's Picks</div>
            <h2 class="font-display text-2xl tracking-tight md:text-[28px]">本周精选</h2>
          </div>
          <span class="text-[11px] text-muted">由编辑从已审核内容中挑选</span>
        </div>
        <RouterLink
          :to="{ name: 'content', params: { id: spotlight.contentId } }"
          class="group grid overflow-hidden border border-line bg-white/40 md:grid-cols-[.9fr_1.1fr]"
        >
          <div class="flex flex-col justify-between p-6 md:p-10">
            <div>
              <div class="flex flex-wrap items-center gap-2 text-[10px] uppercase tracking-widest text-muted">
                <span>本期精选</span>
                <span class="h-2.5 w-px bg-line" />
                <span>{{ spotlight.source }}</span>
              </div>
              <h3 class="mt-5 font-display text-[28px] leading-[1.2] tracking-tight transition group-hover:text-accent md:text-[38px]">
                {{ spotlight.title }}
              </h3>
              <p v-if="spotlight.description" class="mt-4 line-clamp-3 max-w-lg text-[13px] leading-7 text-muted">
                {{ spotlight.description }}
              </p>
            </div>
            <div class="mt-8 flex items-center justify-between">
              <span class="text-[11px] text-muted">{{ spotlight.time }}</span>
              <span class="inline-flex items-center gap-1.5 text-xs text-ink">
                阅读全文
                <ArrowUpRight class="h-3.5 w-3.5 transition group-hover:translate-x-0.5 group-hover:-translate-y-0.5" />
              </span>
            </div>
          </div>
          <div class="order-first min-h-[220px] md:order-last md:min-h-[360px]">
            <CoverArt
              :title="spotlight.title"
              subtitle="Xiwei Space Editorial"
              :meta="spotlight.scene"
              :src="spotlight.cover"
              aspect="auto"
              class="h-full"
            />
          </div>
        </RouterLink>

        <div v-if="morePicks.length" class="mt-1 grid gap-x-10 md:grid-cols-2">
          <RouterLink
            v-for="pick in morePicks"
            :key="pick.id"
            :to="{ name: 'content', params: { id: pick.contentId } }"
            class="group flex gap-4 border-b border-line py-5"
          >
            <div class="w-24 shrink-0 overflow-hidden sm:w-28">
              <CoverArt
                :title="pick.title"
                :src="pick.cover"
                aspect="square"
                class="transition duration-500 group-hover:scale-[1.03]"
              />
            </div>
            <div class="min-w-0 flex-1">
              <div class="text-[9px] uppercase tracking-widest text-muted">编辑精选 · {{ pick.source }}</div>
              <h3 class="mt-1.5 line-clamp-2 font-display text-[17px] leading-[1.35] tracking-tight transition group-hover:text-accent">
                {{ pick.title }}
              </h3>
              <p v-if="pick.description" class="mt-1 line-clamp-2 text-[11px] leading-5 text-muted">{{ pick.description }}</p>
              <p class="mt-2 text-[10px] text-muted">{{ pick.time }}</p>
            </div>
          </RouterLink>
        </div>
      </section>

      <!-- Recommended -->
      <section v-if="forYou.length" class="shell pt-10 md:pt-16">
        <div class="mb-5 flex items-end justify-between gap-4">
          <div>
            <div class="eyebrow mb-2 flex items-center gap-1.5"><Sparkles class="h-3 w-3" /> For You</div>
            <h2 class="font-display text-2xl tracking-tight md:text-[28px]">为你挑选</h2>
          </div>
          <RouterLink to="/discover" class="inline-flex items-center gap-1 text-xs text-muted transition hover:text-ink">
            全部动态 <ArrowUpRight class="h-3 w-3" />
          </RouterLink>
        </div>
        <div class="grid gap-x-10 md:grid-cols-2">
          <ContentCard v-for="item in forYou.slice(0, 6)" :key="item.id" :content="item" :show-summary="false" />
        </div>
      </section>

      <!-- Latest -->
      <section v-if="latest.length" class="shell pt-12 md:pt-20">
        <div class="mb-5 flex items-end justify-between gap-4">
          <div>
            <div class="eyebrow mb-2">Latest</div>
            <h2 class="font-display text-2xl tracking-tight md:text-[28px]">最新动态</h2>
          </div>
          <RouterLink to="/discover" class="inline-flex items-center gap-1 text-xs text-muted transition hover:text-ink">
            查看全部 <ArrowUpRight class="h-3 w-3" />
          </RouterLink>
        </div>
        <div class="border-t border-line">
          <ContentCard v-for="item in latest.slice(0, 6)" :key="item.id" :content="item" />
        </div>
      </section>

      <!-- Archive -->
      <section v-if="archive.length" class="shell pt-12 md:pt-20">
        <div class="mb-5 flex items-end justify-between gap-4">
          <div>
            <div class="eyebrow mb-2">Archive</div>
            <h2 class="font-display text-2xl tracking-tight md:text-[28px]">历史回顾</h2>
          </div>
          <RouterLink to="/discover" class="inline-flex items-center gap-1 text-xs text-muted transition hover:text-ink">
            继续浏览 <ArrowUpRight class="h-3 w-3" />
          </RouterLink>
        </div>
        <p class="mb-3 text-[11px] text-muted">六个月前的公开内容，按时间回溯。</p>
        <div class="grid gap-x-10 md:grid-cols-2">
          <ContentCard v-for="item in archive" :key="item.id" :content="item" :show-summary="false" />
        </div>
      </section>

      <!-- Works -->
      <section v-if="works.length" class="shell pt-12 md:pt-20">
        <div class="mb-5 flex items-end justify-between gap-4">
          <div>
            <div class="eyebrow mb-2">Filmography</div>
            <h2 class="font-display text-2xl tracking-tight md:text-[28px]">作品档案</h2>
          </div>
          <RouterLink to="/works" class="inline-flex items-center gap-1 text-xs text-muted transition hover:text-ink">
            全部作品 <ArrowUpRight class="h-3 w-3" />
          </RouterLink>
        </div>
        <div class="no-scrollbar -mx-5 flex gap-4 overflow-x-auto px-5 pb-2 md:mx-0 md:grid md:grid-cols-4 md:gap-5 md:px-0 lg:grid-cols-5">
          <RouterLink
            v-for="work in works"
            :key="work.id"
            :to="{ name: 'work', params: { id: work.id } }"
            class="group w-[136px] shrink-0 md:w-auto"
          >
            <div class="overflow-hidden">
              <CoverArt
                :title="work.title"
                :subtitle="workTypeLabels[work.type] || '作品'"
                :meta="formatYear(work.release_date)"
                :src="work.cover"
                aspect="portrait"
                class="transition duration-500 group-hover:scale-[1.02]"
              />
            </div>
            <div class="mt-3">
              <h3 class="truncate font-display text-[15px] tracking-tight transition group-hover:text-accent">{{ work.title }}</h3>
              <p class="mt-1 text-[10px] text-muted">
                {{ formatYear(work.release_date) }} · {{ workStatusLabels[work.status] || '' }}
              </p>
            </div>
          </RouterLink>
        </div>
      </section>

      <!-- Closing note -->
      <section class="shell py-14 md:py-20">
        <div class="border-y border-line py-8 md:flex md:items-center md:justify-between">
          <div>
            <div class="eyebrow mb-2">A Quiet Archive</div>
            <p class="font-display text-xl tracking-tight md:text-2xl">不追逐喧闹，只保留值得被记住的部分。</p>
            <p class="mt-2 max-w-xl text-[11px] leading-6 text-muted">
              本站内容来自公开渠道，按来源展示摘要与原文链接。站内不使用未获授权的艺人图片。
            </p>
          </div>
          <button class="btn-ghost mt-5 !min-h-9 text-xs md:mt-0" :disabled="loading" @click="load">
            <RefreshCw class="h-3.5 w-3.5" :class="loading ? 'animate-spin' : ''" />
            刷新
          </button>
        </div>
      </section>
    </template>
  </div>
</template>
