<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { ArrowUpRight, RefreshCw } from 'lucide-vue-next'
import { api } from '../api/client'
import type { Content, EditorialPick, HomeData } from '../types'
import { auth } from '../stores/auth'
import {
  contentTimeLabel,
  formatYear,
  precisionLabels,
  relativeTime,
  trustLabels,
  workStatusLabels,
  workTypeLabels,
} from '../lib/format'
import CoverArt from '../components/CoverArt.vue'
import EmptyState from '../components/EmptyState.vue'
import LoadingBlock from '../components/LoadingBlock.vue'

interface HomeItem {
  id: string
  contentId: string
  title: string
  description: string
  cover: string
  source: string
  time: string
  scene: string
  kind: string
}

interface TimelineRow {
  id: string
  title: string
  date: string
  week: string
  datetime: string
  month: string
  meta: string
}

const data = ref<HomeData | null>(null)
const loading = ref(true)
const error = ref('')

const artist = computed(() => data.value?.artist || null)
const artistName = computed(() => artist.value?.name || '田曦薇')
const coverGlyph = computed(() => artistName.value.trim().slice(-1) || '曦')
const latest = computed(() => data.value?.latest || [])
const forYou = computed(() => data.value?.for_you || [])
const allWorks = computed(() => data.value?.works || [])
const works = computed(() => allWorks.value.slice(0, 5))
const profile = computed(() => (data.value?.artist?.profile_data || {}) as Record<string, string>)
const coverCaption = computed(
  () => [profile.value.profession, profile.value.birth_place].filter(Boolean).join(' · ') || '公开资料 · 持续更新',
)

// The hero prefers the editorial cover, then the avatar. If neither loads, the
// CSS-drawn cover remains, so the layout never depends on a third-party image.
const heroImage = computed(() => artist.value?.cover || artist.value?.avatar || '')
const heroFailed = ref(false)
watch(heroImage, () => {
  heroFailed.value = false
})

const picks = computed(() =>
  (data.value?.picks || []).filter(
    (pick): pick is EditorialPick & { content: Content } => Boolean(pick.content),
  ),
)

const spotlight = computed<HomeItem | null>(() => {
  const pick = picks.value[0]
  if (pick) {
    return {
      id: pick.id,
      contentId: pick.content.id,
      title: pick.title || pick.content.title,
      description: pick.description || pick.content.summary,
      cover: pick.cover_url || pick.content.cover_url,
      source: pick.content.source_name,
      time: contentTimeLabel(pick.content),
      scene: pick.content.scenes?.[0]?.name || '',
      kind: '编辑精选',
    }
  }
  const item = latest.value[0]
  if (!item) return null
  return {
    id: item.id,
    contentId: item.id,
    title: item.title,
    description: item.summary,
    cover: item.cover_url,
    source: item.source_name,
    time: contentTimeLabel(item),
    scene: item.scenes?.[0]?.name || '',
    kind: '最新动态',
  }
})

const morePicks = computed<HomeItem[]>(() => {
  const fromPicks = picks.value.slice(1, 5).map<HomeItem>((pick) => ({
    id: pick.id,
    contentId: pick.content.id,
    title: pick.title || pick.content.title,
    description: pick.description || pick.content.summary,
    cover: pick.cover_url || pick.content.cover_url,
    source: pick.content.source_name,
    time: contentTimeLabel(pick.content),
    scene: pick.content.scenes?.[0]?.name || '',
    kind: '编辑精选',
  }))
  if (fromPicks.length >= 4) return fromPicks

  const used = new Set([spotlight.value?.contentId || '', ...fromPicks.map((p) => p.contentId)])
  const fromLatest = [...forYou.value, ...latest.value]
    .filter((item, index, all) => all.findIndex((entry) => entry.id === item.id) === index)
    .filter((item) => !used.has(item.id))
    .slice(0, 4 - fromPicks.length)
    .map<HomeItem>((item) => ({
      id: item.id,
      contentId: item.id,
      title: item.title,
      description: item.summary,
      cover: item.cover_url,
      source: item.source_name,
      time: contentTimeLabel(item),
      scene: item.scenes?.[0]?.name || '',
      kind: '最新动态',
    }))
  return [...fromPicks, ...fromLatest]
})

const weekNames = ['日', '一', '二', '三', '四', '五', '六']
const pad = (value: number) => String(value).padStart(2, '0')

function parseDate(value?: string | null): Date | null {
  if (!value) return null
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? null : date
}

const timeline = computed<TimelineRow[]>(() =>
  latest.value.slice(0, 4).map((item) => {
    const published = item.published_precision !== 'unknown' ? parseDate(item.published_at) : null
    const fallback = published || parseDate(item.effective_at) || parseDate(item.discovered_at)
    const verified = Boolean(published)
    const meta = [item.source_name, trustLabels[item.trust_level] || '']
    if (item.published_precision !== 'verified') {
      meta.push(precisionLabels[item.published_precision] || '时间待核验')
    }
    return {
      id: item.id,
      title: item.title,
      date: verified && fallback ? `${pad(fallback.getMonth() + 1)}.${pad(fallback.getDate())}` : '待核验',
      week: verified && fallback ? `周${weekNames[fallback.getDay()]}` : '发现时间',
      datetime: verified && fallback ? fallback.toISOString() : '',
      month: verified && fallback ? `${fallback.getFullYear()} 年 ${fallback.getMonth() + 1} 月` : '时间待核验',
      meta: meta.filter(Boolean).join(' · '),
    }
  }),
)

const timelineGroups = computed(() => {
  const groups: { month: string; rows: TimelineRow[] }[] = []
  for (const row of timeline.value) {
    const last = groups[groups.length - 1]
    if (last && last.month === row.month) last.rows.push(row)
    else groups.push({ month: row.month, rows: [row] })
  }
  return groups
})

const issueDate = computed(() => {
  const now = new Date()
  return `${now.getFullYear()}.${pad(now.getMonth() + 1)}`
})

const briefDate = computed(() => {
  const value = data.value?.brief?.date
  if (value) return value.replaceAll('-', '.')
  const now = new Date()
  return `${now.getFullYear()}.${pad(now.getMonth() + 1)}.${pad(now.getDate())}`
})

const toc = computed(() => [
  { n: '01', label: '封面故事', meta: spotlight.value?.scene || '本期人物', href: '#top' },
  { n: '02', label: '编辑精选', meta: picks.value.length ? '本期' : '待更新', href: '#picks' },
  { n: '03', label: '时间线', meta: '最近更新', href: '#timeline' },
  { n: '04', label: '作品档案', meta: '近期作品', href: '#works' },
  { n: '05', label: '关于本站', meta: '来源与核验', href: '#about' },
])

const stats = computed(() => [
  { label: '近期作品', value: `${allWorks.value.length} 部` },
  { label: '本期精选', value: `${picks.value.length} 条` },
  { label: '今日新增', value: `${data.value?.brief?.new_count ?? 0} 条` },
  {
    label: '最近更新',
    value: data.value?.brief?.last_updated_at ? relativeTime(data.value.brief.last_updated_at) : '等待采集',
  },
])

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
  <div id="top" class="issue-home">
    <div v-if="loading && !data" class="shell py-20 md:py-28">
      <LoadingBlock :rows="5" />
    </div>

    <div v-else-if="error && !data" class="shell py-20">
      <EmptyState title="暂时没能打开首页" :description="error" action-label="重新加载" @action="load" />
    </div>

    <div v-else-if="!data" class="shell py-20">
      <EmptyState title="首页还没有内容" description="采集完成后，这一期会从这里开始。" action-label="重新加载" @action="load" />
    </div>

    <template v-else>
      <!-- Cover story -->
      <section class="issue-hero">
        <div class="shell issue-hero-grid">
          <div class="issue-hero-copy">
            <p class="issue-kicker">
              <span class="issue-kicker-line" aria-hidden="true" />
              本期封面 · Cover Story
            </p>
            <h1 class="issue-title">{{ artistName }}</h1>
            <p class="issue-dek">把关于她的每一天，收进一个安静的空间。</p>
            <p class="issue-lede">
              {{ data.artist?.bio || '演员田曦薇的专属信息档案：作品、角色、公开动态与经核验的资料。' }}
            </p>
            <p class="issue-meta">
              <span class="issue-meta-dot" aria-hidden="true" />
              公开资料 · 来源可溯 · 持续更新
            </p>
            <div class="issue-actions">
              <RouterLink
                v-if="spotlight"
                :to="{ name: 'content', params: { id: spotlight.contentId } }"
                class="issue-btn issue-btn-dark"
              >
                读封面故事
                <ArrowUpRight class="h-4 w-4" />
              </RouterLink>
              <a href="#toc" class="issue-btn issue-btn-outline">本期目录</a>
            </div>
            <p class="issue-hero-note">
              <span v-if="picks.length">本期收录 {{ picks.length }} 条精选</span>
              <span v-else>本期封面已更新</span>
              <span class="issue-note-sep" aria-hidden="true" />
              <span>{{ briefDate }}</span>
            </p>
          </div>

          <div class="issue-cover-wrap">
            <div class="issue-cover" :class="{ 'has-image': heroImage && !heroFailed }">
              <img
                v-if="heroImage && !heroFailed"
                :src="heroImage"
                :alt="artistName"
                loading="eager"
                fetchpriority="high"
                decoding="async"
                @error="heroFailed = true"
              />
              <template v-else>
                <span class="cover-orbit cover-orbit-a" aria-hidden="true" />
                <span class="cover-orbit cover-orbit-b" aria-hidden="true" />
                <span class="cover-glyph" aria-hidden="true">{{ coverGlyph }}</span>
                <span class="cover-vertical">{{ artistName }}</span>
              </template>
              <span class="cover-shade" aria-hidden="true" />
              <div class="cover-top">
                <span>Xiwei Space</span>
                <span>Archive · {{ issueDate }}</span>
              </div>
              <div class="cover-bottom">
                <b>封面人物</b>
                <span>Portrait · 01</span>
              </div>
            </div>
            <p class="issue-cover-caption">{{ coverCaption }}</p>
          </div>
        </div>
      </section>

      <!-- Brief strip -->
      <div class="issue-brief">
        <div class="shell issue-brief-inner">
          <span class="issue-brief-item">
            <span class="issue-brief-dot" aria-hidden="true" />
            今日新增 {{ data.brief?.new_count ?? 0 }} 条
          </span>
          <span class="issue-brief-sep" aria-hidden="true" />
          <span class="issue-brief-item">
            最近更新 {{ data.brief?.last_updated_at ? relativeTime(data.brief.last_updated_at) : '等待首轮采集' }}
          </span>
          <span class="issue-brief-sep issue-brief-sep-wide" aria-hidden="true" />
          <span class="issue-brief-note">公开来源 · 摘要呈现 · 原文可溯</span>
          <RouterLink to="/discover" class="issue-brief-link">
            全部动态
            <ArrowUpRight class="h-3 w-3" />
          </RouterLink>
        </div>
      </div>

      <!-- Table of contents -->
      <nav id="toc" class="shell issue-toc" aria-label="本期目录">
        <a v-for="item in toc" :key="item.n" :href="item.href" class="issue-toc-item">
          <span class="issue-toc-n">{{ item.n }}</span>
          <span class="issue-toc-label">{{ item.label }}</span>
          <span class="issue-toc-meta">{{ item.meta }}</span>
        </a>
      </nav>

      <!-- Picks -->
      <section v-if="spotlight" id="picks" class="shell issue-section">
        <header class="issue-head">
          <div class="issue-head-title">
            <span class="issue-num">02</span>
            <h2>编辑精选</h2>
          </div>
          <RouterLink to="/discover" class="issue-more">
            全部动态
            <ArrowUpRight class="h-3 w-3" />
          </RouterLink>
        </header>

        <div class="issue-picks" :class="{ 'issue-picks-single': !morePicks.length }">
          <RouterLink
            :to="{ name: 'content', params: { id: spotlight.contentId } }"
            class="issue-lead group"
          >
            <div class="issue-lead-cover">
              <CoverArt
                :title="spotlight.title"
                subtitle="Xiwei Space Editorial"
                :meta="spotlight.scene"
                :src="spotlight.cover"
                aspect="wide"
                priority
              />
            </div>
            <div class="issue-lead-body">
              <p class="issue-lead-kind">{{ spotlight.kind }} · {{ spotlight.source }}</p>
              <h3>{{ spotlight.title }}</h3>
              <p v-if="spotlight.description" class="issue-lead-desc">{{ spotlight.description }}</p>
              <div class="issue-lead-meta">
                <span>{{ spotlight.time }}</span>
                <span class="issue-lead-read">
                  阅读全文
                  <ArrowUpRight class="h-3.5 w-3.5" />
                </span>
              </div>
            </div>
          </RouterLink>

          <div v-if="morePicks.length" class="issue-side">
            <RouterLink
              v-for="pick in morePicks"
              :key="pick.id"
              :to="{ name: 'content', params: { id: pick.contentId } }"
              class="issue-side-item group"
            >
              <div class="issue-side-cover">
                <CoverArt :title="pick.title" :src="pick.cover" aspect="square" compact />
              </div>
              <div class="min-w-0">
                <p class="issue-side-kind">{{ pick.kind }} · {{ pick.source }}</p>
                <h3>{{ pick.title }}</h3>
                <p v-if="pick.description" class="issue-side-desc">{{ pick.description }}</p>
                <p class="issue-side-time">{{ pick.time }}</p>
              </div>
            </RouterLink>
          </div>
        </div>
      </section>

      <!-- Timeline -->
      <section v-if="timeline.length" id="timeline" class="shell issue-section">
        <header class="issue-head">
          <div class="issue-head-title">
            <span class="issue-num">03</span>
            <h2>近期纪事</h2>
          </div>
          <RouterLink to="/discover" class="issue-more">
            完整时间线
            <ArrowUpRight class="h-3 w-3" />
          </RouterLink>
        </header>

        <div
          v-for="(group, groupIndex) in timelineGroups"
          :key="`${group.month}-${groupIndex}`"
          class="issue-month-group"
        >
          <div class="issue-month">
            <span>{{ group.month }}</span>
            <span class="issue-month-line" aria-hidden="true" />
            <span>{{ group.rows.length }} 条记录</span>
          </div>
          <div class="issue-timeline">
            <RouterLink
              v-for="row in group.rows"
              :key="row.id"
              :to="{ name: 'content', params: { id: row.id } }"
              class="issue-timeline-row group"
            >
              <time class="issue-date" :datetime="row.datetime || undefined">
                <b>{{ row.date }}</b>
                <span>{{ row.week }}</span>
              </time>
              <div class="issue-timeline-copy">
                <h3>{{ row.title }}</h3>
                <p>{{ row.meta }}</p>
              </div>
              <ArrowUpRight class="issue-row-arrow" aria-hidden="true" />
            </RouterLink>
          </div>
        </div>
      </section>

      <!-- Works -->
      <section v-if="works.length" id="works" class="shell issue-section">
        <header class="issue-head">
          <div class="issue-head-title">
            <span class="issue-num">04</span>
            <h2>作品档案</h2>
          </div>
          <RouterLink to="/works" class="issue-more">
            全部作品
            <ArrowUpRight class="h-3 w-3" />
          </RouterLink>
        </header>

        <div class="issue-works">
          <RouterLink
            v-for="work in works"
            :key="work.id"
            :to="{ name: 'work', params: { id: work.id } }"
            class="issue-work group"
          >
            <div class="issue-work-cover">
              <CoverArt
                :title="work.title"
                :subtitle="workTypeLabels[work.type] || '作品'"
                :meta="formatYear(work.release_date)"
                :src="work.cover"
                aspect="portrait"
              />
            </div>
            <h3>{{ work.title }}</h3>
            <p>{{ formatYear(work.release_date) }} · {{ workStatusLabels[work.status] || '公开资料' }}</p>
          </RouterLink>
        </div>
      </section>

      <!-- About -->
      <section id="about" class="issue-about">
        <div class="shell issue-about-grid">
          <div>
            <p class="issue-kicker">
              <span class="issue-kicker-line" aria-hidden="true" />
              05 · A Quiet Archive
            </p>
            <h2>不追逐喧闹，<br />只保留值得被记住的部分。</h2>
            <p class="issue-about-body">
              曦薇空间是一个单艺人公开信息档案。内容来自公开渠道，站内仅保留摘要与原文链接；发布时间与发现时间分开记录，时间不明确时会明确标注。图片与人物或作品对应，缺失时使用站内抽象封面。
            </p>
            <div class="issue-about-actions">
              <RouterLink to="/discover" class="issue-text-link">
                进入完整档案
                <ArrowUpRight class="h-3.5 w-3.5" />
              </RouterLink>
              <button class="issue-refresh" type="button" :disabled="loading" @click="load">
                <RefreshCw class="h-3.5 w-3.5" :class="loading ? 'animate-spin' : ''" />
                刷新数据
              </button>
            </div>
            <p v-if="error" class="issue-refresh-error" role="status">刷新失败：{{ error }}</p>
          </div>

          <dl class="issue-stats">
            <div v-for="stat in stats" :key="stat.label" class="issue-stat">
              <dt>{{ stat.label }}</dt>
              <dd>{{ stat.value }}</dd>
            </div>
          </dl>
        </div>
      </section>
    </template>
  </div>
</template>

<style scoped>
.issue-home {
  --paper: #faf7f2;
  --surface: #fffdf9;
  --ink: #2e292a;
  --muted: #6e6460;
  --faint: #7a706d;
  --deco: #9a8e8b;
  --line: #e4dcd3;
  --accent: #b06b74;
  --accent-deep: #8f4c56;
  --sage: #6f7a67;
  background: var(--paper);
  color: var(--ink);
}

.issue-hero {
  border-bottom: 1px solid var(--line);
  background:
    radial-gradient(56% 62% at 90% 14%, rgba(176, 107, 116, 0.09), transparent 70%),
    radial-gradient(42% 50% at 4% 94%, rgba(111, 122, 103, 0.07), transparent 70%),
    var(--paper);
}

.issue-hero-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.05fr) minmax(0, 0.95fr);
  gap: clamp(40px, 6vw, 88px);
  align-items: center;
  padding-top: clamp(56px, 7vw, 94px);
  padding-bottom: clamp(44px, 5vw, 62px);
}

.issue-kicker {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 11px;
  letter-spacing: 0.2em;
  text-transform: uppercase;
  color: var(--muted);
}

.issue-kicker-line {
  display: inline-block;
  width: 28px;
  height: 1px;
  background: var(--accent);
}

.issue-title {
  margin-top: 20px;
  font-family: Georgia, 'Times New Roman', 'Songti SC', STSong, 'Noto Serif SC', serif;
  font-size: clamp(56px, 7.4vw, 94px);
  font-weight: 400;
  line-height: 0.95;
  letter-spacing: -0.045em;
}

.issue-dek {
  margin-top: 18px;
  font-family: Georgia, 'Times New Roman', 'Songti SC', STSong, 'Noto Serif SC', serif;
  font-size: clamp(19px, 2vw, 23px);
  line-height: 1.55;
  letter-spacing: -0.015em;
  color: var(--ink);
}

.issue-lede {
  margin-top: 18px;
  max-width: 450px;
  font-size: 14px;
  line-height: 1.95;
  color: var(--muted);
}

.issue-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 22px;
  font-size: 11px;
  letter-spacing: 0.04em;
  color: var(--faint);
}

.issue-meta-dot {
  width: 4px;
  height: 4px;
  border-radius: 999px;
  background: var(--accent);
}

.issue-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-top: 28px;
}

.issue-btn {
  display: inline-flex;
  min-height: 44px;
  align-items: center;
  justify-content: center;
  gap: 8px;
  border: 1px solid transparent;
  padding: 0 20px;
  font-size: 12px;
  letter-spacing: 0.04em;
  transition: background-color 0.2s ease, border-color 0.2s ease, color 0.2s ease;
}

.issue-btn-dark {
  background: var(--ink);
  border-color: var(--ink);
  color: var(--paper);
}

.issue-btn-dark:hover {
  background: #443c3d;
}

.issue-btn-outline {
  border-color: var(--line);
  color: var(--ink);
}

.issue-btn-outline:hover {
  border-color: rgba(46, 41, 42, 0.35);
  background: rgba(255, 253, 249, 0.65);
}

.issue-hero-note {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  margin-top: 24px;
  font-size: 11px;
  color: var(--muted);
}

.issue-note-sep {
  width: 1px;
  height: 10px;
  background: var(--line);
}

.issue-cover-wrap {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
}

.issue-cover {
  position: relative;
  width: 100%;
  max-width: 410px;
  aspect-ratio: 3 / 4;
  overflow: hidden;
  padding: 24px;
  background: linear-gradient(158deg, #f0e2dc 0%, #e4cbc5 46%, #cfaeab 100%);
  box-shadow: 0 30px 64px -46px rgba(46, 41, 42, 0.6);
}

.issue-cover::before {
  content: '';
  position: absolute;
  inset: 12px;
  border: 1px solid rgba(46, 41, 42, 0.13);
  z-index: 6;
  pointer-events: none;
}

.issue-cover img {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  object-fit: cover;
  object-position: center 18%;
}

.cover-orbit {
  position: absolute;
  border-radius: 999px;
  border: 1px solid rgba(46, 41, 42, 0.18);
}

.cover-orbit-a {
  right: -22%;
  top: -10%;
  width: 74%;
  aspect-ratio: 1;
  box-shadow: 0 0 0 28px rgba(255, 255, 255, 0.12);
}

.cover-orbit-b {
  left: -18%;
  bottom: -16%;
  width: 52%;
  aspect-ratio: 1;
  border-color: rgba(255, 255, 255, 0.32);
}

.cover-glyph {
  position: absolute;
  left: 20px;
  bottom: 72px;
  font-family: Georgia, 'Times New Roman', 'Songti SC', STSong, 'Noto Serif SC', serif;
  font-size: clamp(150px, 19vw, 232px);
  line-height: 0.78;
  color: rgba(46, 41, 42, 0.12);
  z-index: 1;
}

.cover-vertical {
  position: absolute;
  right: 24px;
  top: 64px;
  z-index: 2;
  writing-mode: vertical-rl;
  text-orientation: mixed;
  font-family: Georgia, 'Times New Roman', 'Songti SC', STSong, 'Noto Serif SC', serif;
  font-size: 13px;
  letter-spacing: 0.42em;
  color: rgba(46, 41, 42, 0.62);
}

.cover-shade {
  position: absolute;
  inset: 0;
  z-index: 3;
  opacity: 0;
  pointer-events: none;
  background: linear-gradient(to bottom, rgba(30, 24, 23, 0.3), transparent 36%, transparent 52%, rgba(30, 24, 23, 0.66));
  transition: opacity 0.2s ease;
}

.issue-cover.has-image .cover-shade {
  opacity: 1;
}

.cover-top {
  position: absolute;
  z-index: 5;
  top: 24px;
  left: 24px;
  right: 24px;
  display: flex;
  justify-content: space-between;
  gap: 10px;
  font-size: 10px;
  letter-spacing: 0.16em;
  text-transform: uppercase;
  color: rgba(46, 41, 42, 0.64);
}

.cover-bottom {
  position: absolute;
  z-index: 5;
  bottom: 24px;
  left: 24px;
  right: 24px;
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
  border-top: 1px solid rgba(46, 41, 42, 0.18);
  padding-top: 12px;
  color: var(--ink);
}

.cover-bottom b {
  font-family: Georgia, 'Times New Roman', 'Songti SC', STSong, 'Noto Serif SC', serif;
  font-size: 23px;
  font-weight: 400;
  letter-spacing: 0.02em;
}

.cover-bottom span {
  font-size: 10px;
  letter-spacing: 0.18em;
  text-transform: uppercase;
  color: rgba(46, 41, 42, 0.6);
}

.issue-cover.has-image .cover-top,
.issue-cover.has-image .cover-bottom span,
.issue-cover.has-image .cover-vertical {
  color: rgba(255, 255, 255, 0.88);
}

.issue-cover.has-image .cover-bottom {
  border-top-color: rgba(255, 255, 255, 0.36);
  color: #fff;
}

.issue-cover.has-image .cover-bottom b {
  color: #fff;
}

.issue-cover-caption {
  margin-top: 12px;
  font-size: 11px;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--faint);
}

.issue-brief {
  border-bottom: 1px solid var(--line);
  background: rgba(255, 253, 249, 0.62);
}

.issue-brief-inner {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px 18px;
  padding-top: 13px;
  padding-bottom: 13px;
  font-size: 11px;
  color: var(--muted);
}

.issue-brief-item {
  display: inline-flex;
  align-items: center;
  gap: 7px;
}

.issue-brief-dot {
  width: 5px;
  height: 5px;
  border-radius: 999px;
  background: var(--sage);
}

.issue-brief-sep {
  width: 1px;
  height: 12px;
  background: var(--line);
}

.issue-brief-note {
  letter-spacing: 0.02em;
}

.issue-brief-link {
  margin-left: auto;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  color: var(--ink);
  transition: color 0.2s ease;
}

.issue-brief-link:hover {
  color: var(--accent-deep);
}

.issue-toc {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  border-top: 1px solid var(--line);
  border-bottom: 1px solid var(--line);
  scroll-margin-top: 92px;
}

.issue-toc-item {
  display: flex;
  flex-direction: column;
  gap: 7px;
  padding: 21px 18px 19px;
  border-left: 1px solid var(--line);
  transition: background-color 0.2s ease;
}

.issue-toc-item:first-child {
  padding-left: 0;
  border-left: 0;
}

.issue-toc-item:hover {
  background: rgba(255, 253, 249, 0.72);
}

.issue-toc-n {
  font-size: 10px;
  letter-spacing: 0.16em;
  color: var(--accent-deep);
}

.issue-toc-label {
  font-family: Georgia, 'Times New Roman', 'Songti SC', STSong, 'Noto Serif SC', serif;
  font-size: 15px;
  letter-spacing: -0.01em;
}

.issue-toc-meta {
  font-size: 11px;
  color: var(--faint);
}

.issue-section {
  padding-top: clamp(52px, 6vw, 76px);
  scroll-margin-top: 92px;
}

.issue-head {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 16px;
  border-bottom: 1px solid var(--line);
  padding-bottom: 18px;
  margin-bottom: 28px;
}

.issue-head-title {
  display: flex;
  align-items: baseline;
  gap: 14px;
}

.issue-num {
  font-size: 11px;
  letter-spacing: 0.18em;
  color: var(--accent-deep);
}

.issue-head h2 {
  font-family: Georgia, 'Times New Roman', 'Songti SC', STSong, 'Noto Serif SC', serif;
  font-size: clamp(26px, 2.4vw, 34px);
  font-weight: 400;
  line-height: 1;
  letter-spacing: -0.025em;
}

.issue-more {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 11px;
  color: var(--muted);
  transition: color 0.2s ease;
}

.issue-more:hover {
  color: var(--ink);
}

.issue-picks {
  display: grid;
  grid-template-columns: minmax(0, 1.32fr) minmax(0, 0.68fr);
  gap: clamp(28px, 4vw, 52px);
  align-items: start;
}

.issue-picks-single {
  grid-template-columns: minmax(0, 1fr);
}

.issue-lead {
  display: block;
}

.issue-lead-cover {
  overflow: hidden;
  border: 1px solid var(--line);
}

.issue-lead-cover :deep(.cover-art),
.issue-side-cover :deep(.cover-art),
.issue-work-cover :deep(.cover-art) {
  transition: transform 0.55s cubic-bezier(0.2, 0.7, 0.3, 1);
}

.issue-lead:hover :deep(.cover-art),
.issue-side-item:hover :deep(.cover-art),
.issue-work:hover :deep(.cover-art) {
  transform: scale(1.022);
}

.issue-lead-body {
  padding-top: 18px;
}

.issue-lead-kind {
  font-size: 11px;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  color: var(--accent-deep);
}

.issue-lead h3 {
  margin-top: 8px;
  font-family: Georgia, 'Times New Roman', 'Songti SC', STSong, 'Noto Serif SC', serif;
  font-size: clamp(24px, 2.6vw, 32px);
  font-weight: 400;
  line-height: 1.24;
  letter-spacing: -0.025em;
  transition: color 0.2s ease;
}

.issue-lead:hover h3 {
  color: var(--accent-deep);
}

.issue-lead-desc {
  display: -webkit-box;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 3;
  margin-top: 10px;
  max-width: 620px;
  font-size: 13.5px;
  line-height: 1.9;
  color: var(--muted);
}

.issue-lead-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-top: 16px;
  font-size: 11px;
  color: var(--faint);
}

.issue-lead-read {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  color: var(--ink);
}

.issue-side {
  border-top: 1px solid var(--line);
}

.issue-side-item {
  display: grid;
  grid-template-columns: 100px minmax(0, 1fr);
  gap: 16px;
  align-items: start;
  padding: 16px 0;
  border-bottom: 1px solid var(--line);
}

.issue-side-cover {
  overflow: hidden;
}

.issue-side-kind {
  font-size: 11px;
  letter-spacing: 0.11em;
  text-transform: uppercase;
  color: var(--accent-deep);
}

.issue-side h3 {
  display: -webkit-box;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  margin-top: 5px;
  font-family: Georgia, 'Times New Roman', 'Songti SC', STSong, 'Noto Serif SC', serif;
  font-size: 16px;
  font-weight: 400;
  line-height: 1.4;
  letter-spacing: -0.01em;
  transition: color 0.2s ease;
}

.issue-side-item:hover h3 {
  color: var(--accent-deep);
}

.issue-side-desc {
  display: -webkit-box;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  margin-top: 4px;
  font-size: 12px;
  line-height: 1.65;
  color: var(--muted);
}

.issue-side-time {
  margin-top: 6px;
  font-size: 11px;
  color: var(--faint);
}

.issue-month-group + .issue-month-group {
  margin-top: 28px;
}

.issue-month {
  display: flex;
  align-items: center;
  gap: 14px;
  font-size: 11px;
  letter-spacing: 0.12em;
  color: var(--muted);
}

.issue-month-line {
  flex: 1;
  height: 1px;
  background: var(--line);
}

.issue-timeline {
  margin-top: 2px;
  border-top: 1px solid var(--line);
}

.issue-timeline-row {
  display: grid;
  grid-template-columns: 88px minmax(0, 1fr) 24px;
  gap: 22px;
  align-items: center;
  padding: 18px 0;
  border-bottom: 1px solid var(--line);
}

.issue-date {
  display: flex;
  align-items: baseline;
  gap: 8px;
  font-variant-numeric: tabular-nums;
}

.issue-date b {
  font-family: Georgia, 'Times New Roman', 'Songti SC', STSong, 'Noto Serif SC', serif;
  font-size: 15px;
  font-weight: 400;
  letter-spacing: 0.02em;
}

.issue-date span {
  font-size: 11px;
  color: var(--faint);
}

.issue-timeline-copy {
  min-width: 0;
}

.issue-timeline-copy h3 {
  font-family: Georgia, 'Times New Roman', 'Songti SC', STSong, 'Noto Serif SC', serif;
  font-size: 16px;
  font-weight: 400;
  line-height: 1.45;
  letter-spacing: -0.01em;
  transition: color 0.2s ease;
}

.issue-timeline-row:hover .issue-timeline-copy h3 {
  color: var(--accent-deep);
}

.issue-timeline-copy p {
  margin-top: 4px;
  font-size: 11px;
  color: var(--muted);
}

.issue-row-arrow {
  justify-self: end;
  width: 15px;
  height: 15px;
  color: var(--deco);
  transition: color 0.2s ease, transform 0.2s ease;
}

.issue-timeline-row:hover .issue-row-arrow {
  color: var(--ink);
  transform: translate(2px, -2px);
}

.issue-works {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: clamp(14px, 2vw, 24px);
}

.issue-work {
  display: block;
}

.issue-work-cover {
  overflow: hidden;
  border: 1px solid var(--line);
}

.issue-work h3 {
  margin-top: 12px;
  font-family: Georgia, 'Times New Roman', 'Songti SC', STSong, 'Noto Serif SC', serif;
  font-size: 15px;
  font-weight: 400;
  line-height: 1.4;
  letter-spacing: -0.01em;
  transition: color 0.2s ease;
}

.issue-work:hover h3 {
  color: var(--accent-deep);
}

.issue-work p {
  margin-top: 3px;
  font-size: 11px;
  color: var(--muted);
}

/* Vary the generated cover composition so five posters do not read as one image. */
.issue-work:nth-child(2) :deep(.cover-art::before) {
  width: 108%;
  right: -58%;
  top: 26%;
  opacity: 0.72;
}

.issue-work:nth-child(2) :deep(.cover-art::after) {
  display: none;
}

.issue-work:nth-child(3) :deep(.cover-art::before) {
  width: 92%;
  right: -28%;
  top: 42%;
  border-radius: 0;
  transform: rotate(26deg);
}

.issue-work:nth-child(3) :deep(.cover-art::after) {
  left: auto;
  right: -30%;
  bottom: -34%;
  width: 118%;
}

.issue-work:nth-child(4) :deep(.cover-art::before) {
  width: 40%;
  right: 10%;
  top: 52%;
  background: currentColor;
  opacity: 0.08;
}

.issue-work:nth-child(4) :deep(.cover-art::after) {
  left: -24%;
  bottom: -36%;
  width: 92%;
  border-color: rgba(46, 41, 42, 0.16);
}

.issue-work:nth-child(5) :deep(.cover-art::before) {
  width: 118%;
  right: -62%;
  top: -30%;
  opacity: 0.45;
  box-shadow: 0 0 0 22px rgba(255, 255, 255, 0.1);
}

.issue-work:nth-child(5) :deep(.cover-art::after) {
  display: none;
}

.issue-about {
  margin-top: clamp(58px, 7vw, 88px);
  border-top: 1px solid var(--line);
  background: #f5efe7;
  scroll-margin-top: 92px;
}

.issue-about-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.12fr) minmax(0, 0.88fr);
  gap: clamp(32px, 6vw, 88px);
  align-items: start;
  padding-top: clamp(48px, 6vw, 72px);
  padding-bottom: clamp(48px, 6vw, 72px);
}

.issue-about h2 {
  margin-top: 14px;
  font-family: Georgia, 'Times New Roman', 'Songti SC', STSong, 'Noto Serif SC', serif;
  font-size: clamp(28px, 3.2vw, 42px);
  font-weight: 400;
  line-height: 1.34;
  letter-spacing: -0.03em;
}

.issue-about-body {
  margin-top: 16px;
  max-width: 560px;
  font-size: 13px;
  line-height: 1.95;
  color: var(--muted);
}

.issue-about-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 18px;
  margin-top: 24px;
}

.issue-text-link {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  border-bottom: 1px solid rgba(46, 41, 42, 0.32);
  padding-bottom: 2px;
  font-size: 12px;
  color: var(--ink);
  transition: border-color 0.2s ease, color 0.2s ease;
}

.issue-text-link:hover {
  border-color: var(--accent-deep);
  color: var(--accent-deep);
}

.issue-refresh {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 11px;
  color: var(--muted);
  transition: color 0.2s ease;
}

.issue-refresh:hover {
  color: var(--ink);
}

.issue-refresh-error {
  margin-top: 10px;
  font-size: 11px;
  line-height: 1.6;
  color: var(--accent-deep);
}

.issue-stats {
  border-top: 1px solid rgba(46, 41, 42, 0.2);
}

.issue-stat {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 20px;
  padding: 15px 0;
  border-bottom: 1px solid rgba(46, 41, 42, 0.12);
}

.issue-stat dt {
  font-size: 11px;
  letter-spacing: 0.06em;
  color: var(--muted);
}

.issue-stat dd {
  font-family: Georgia, 'Times New Roman', 'Songti SC', STSong, 'Noto Serif SC', serif;
  font-size: 21px;
  letter-spacing: -0.01em;
}

@media (max-width: 1024px) {
  .issue-hero-grid {
    gap: 48px;
  }

  .issue-cover {
    max-width: 370px;
  }

  .issue-side-item {
    grid-template-columns: 88px minmax(0, 1fr);
    gap: 13px;
  }
}

@media (max-width: 900px) {
  .issue-hero-grid {
    grid-template-columns: minmax(0, 1fr);
  }

  .issue-cover-wrap {
    align-items: center;
    margin-top: 6px;
  }

  .issue-cover {
    max-width: 400px;
  }

  .issue-cover-caption {
    text-align: center;
  }

  .issue-picks,
  .issue-about-grid {
    grid-template-columns: minmax(0, 1fr);
  }

  .issue-about-grid {
    gap: 36px;
  }
}

@media (max-width: 720px) {
  .issue-hero-grid {
    padding-top: 48px;
    padding-bottom: 40px;
  }

  .issue-title {
    font-size: 56px;
  }

  .issue-dek {
    font-size: 19px;
  }

  .issue-lede {
    font-size: 13.5px;
    line-height: 1.9;
  }

  .issue-cover {
    aspect-ratio: 4 / 5;
    max-width: 360px;
    padding: 20px;
  }

  .issue-cover::before {
    inset: 10px;
  }

  .cover-glyph {
    left: 16px;
    bottom: 62px;
    font-size: 148px;
  }

  .cover-vertical {
    top: 52px;
    right: 20px;
  }

  .cover-top {
    top: 20px;
    left: 20px;
    right: 20px;
    font-size: 10px;
    letter-spacing: 0.12em;
  }

  .cover-bottom {
    bottom: 20px;
    left: 20px;
    right: 20px;
  }

  .cover-bottom b {
    font-size: 21px;
  }

  .issue-brief-sep,
  .issue-brief-sep-wide {
    display: none;
  }

  .issue-brief-inner {
    gap: 6px 14px;
  }

  .issue-brief-note {
    width: 100%;
    order: 3;
  }

  .issue-brief-link {
    order: 2;
    margin-left: auto;
  }

  .issue-toc {
    display: flex;
    overflow-x: auto;
    scrollbar-width: none;
    margin-right: -20px;
    padding-right: 20px;
  }

  .issue-toc::-webkit-scrollbar {
    display: none;
  }

  .issue-toc-item {
    min-width: 148px;
    padding: 17px 16px;
  }

  .issue-toc-item:first-child {
    padding-left: 0;
  }

  .issue-head {
    padding-bottom: 14px;
    margin-bottom: 22px;
  }

  .issue-head-title {
    gap: 10px;
  }

  .issue-head h2 {
    font-size: 26px;
  }

  .issue-side-item {
    grid-template-columns: 82px minmax(0, 1fr);
    gap: 12px;
    padding: 14px 0;
  }

  .issue-side h3 {
    font-size: 15px;
  }

  .issue-date {
    flex-direction: column;
    align-items: flex-start;
    gap: 2px;
  }

  .issue-timeline-row {
    grid-template-columns: 58px minmax(0, 1fr) 16px;
    gap: 12px;
    padding: 15px 0;
  }

  .issue-timeline-copy h3 {
    font-size: 14.5px;
    line-height: 1.45;
  }

  .issue-works {
    display: flex;
    gap: 14px;
    overflow-x: auto;
    scrollbar-width: none;
    margin-right: -20px;
    padding-right: 20px;
  }

  .issue-works::-webkit-scrollbar {
    display: none;
  }

  .issue-work {
    width: 148px;
    flex: 0 0 148px;
  }

  .issue-about h2 {
    font-size: 28px;
  }

  .issue-about-actions {
    gap: 14px;
  }

  .issue-stat dd {
    font-size: 19px;
  }
}
</style>
