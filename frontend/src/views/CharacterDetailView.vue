<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { ArrowLeft, ArrowUpRight, Film, Quote } from 'lucide-vue-next'
import { api } from '../api/client'
import type { CharacterDetail } from '../types'
import { formatDate, workTypeLabels } from '../lib/format'
import ContentCard from '../components/ContentCard.vue'
import EmptyState from '../components/EmptyState.vue'
import FavoriteButton from '../components/FavoriteButton.vue'
import LoadingBlock from '../components/LoadingBlock.vue'

const route = useRoute()
const detail = ref<CharacterDetail | null>(null)
const loading = ref(true)
const error = ref('')

const characterId = computed(() => String(route.params.id || ''))
const character = computed(() => detail.value?.character || null)
const work = computed(() => character.value?.work || null)

async function load() {
  loading.value = true
  error.value = ''
  try {
    detail.value = await api.get<CharacterDetail>(`/characters/${characterId.value}`)
  } catch (err) {
    error.value = err instanceof Error ? err.message : '角色加载失败'
  } finally {
    loading.value = false
  }
}

onMounted(load)
watch(characterId, (next, prev) => {
  if (next && next !== prev) load()
})
</script>

<template>
  <div class="shell pb-16 pt-6 md:pb-24 md:pt-10">
    <RouterLink
      :to="work ? { name: 'work', params: { id: work.id } } : '/works'"
      class="mb-6 inline-flex items-center gap-2 text-xs text-muted transition hover:text-ink"
    >
      <ArrowLeft class="h-3.5 w-3.5" />
      {{ work ? `返回《${work.title}》` : '返回作品档案' }}
    </RouterLink>

    <div v-if="loading" class="grid gap-8 md:grid-cols-[.8fr_1.2fr]">
      <div class="skeleton aspect-square w-full max-w-[380px]" />
      <div class="space-y-4">
        <div class="skeleton h-3 w-24" />
        <div class="skeleton h-12 w-1/2" />
        <div class="skeleton h-24 w-full" />
      </div>
    </div>

    <EmptyState v-else-if="error" title="角色不可用" :description="error" action-label="回到作品档案" to="/works" />

    <template v-else-if="character">
      <header class="grid items-start gap-8 border-b border-line pb-10 md:grid-cols-[.8fr_1.2fr] md:gap-14 md:pb-14">
        <div class="relative aspect-square overflow-hidden border border-line bg-blush/30">
          <img
            v-if="character.avatar"
            :src="character.avatar"
            :alt="character.name"
            class="h-full w-full object-cover"
          />
          <div v-else class="flex h-full flex-col justify-between p-6 md:p-8">
            <div class="flex items-start justify-between text-[9px] uppercase tracking-[0.2em] text-[#8f5e66]/70">
              <span>Character File</span>
              <Quote class="h-5 w-5 opacity-50" />
            </div>
            <div>
              <span class="font-display text-[120px] leading-none text-[#8f5e66]/25">{{ character.name.slice(0, 1) }}</span>
            </div>
            <div class="flex items-end justify-between">
              <div>
                <div class="text-[9px] uppercase tracking-[0.18em] text-[#8f5e66]/60">Role</div>
                <div class="mt-1 font-display text-xl text-[#6f4b50]">{{ character.name }}</div>
              </div>
              <span class="vertical-text text-[9px] tracking-[0.3em] text-[#8f5e66]/50">XIWEI SPACE</span>
            </div>
          </div>
        </div>

        <div>
          <div class="eyebrow mb-3">Character Archive</div>
          <h1 class="display-title text-[42px] md:text-[64px]">{{ character.name }}</h1>
          <div v-if="work" class="mt-4 flex flex-wrap items-center gap-2 text-xs text-muted">
            <span>出自</span>
            <RouterLink
              :to="{ name: 'work', params: { id: work.id } }"
              class="inline-flex items-center gap-1 border-b border-line pb-0.5 text-ink transition hover:border-ink"
            >
              《{{ work.title }}》
              <ArrowUpRight class="h-3 w-3" />
            </RouterLink>
            <span class="tag">{{ workTypeLabels[work.type] || '作品' }}</span>
            <span v-if="work.release_date" class="tag">{{ formatDate(work.release_date) }}</span>
          </div>
          <p v-if="character.description" class="mt-6 max-w-xl text-[15px] leading-8 text-ink/75">{{ character.description }}</p>
          <p v-else class="mt-6 text-sm text-muted">角色简介整理中。</p>
          <div class="mt-7 flex flex-wrap items-center gap-3">
            <FavoriteButton
              v-model="character.favorited"
              target-type="character"
              :target-id="character.id"
              class="btn-ghost !min-h-10 !px-4 text-xs"
            />
            <RouterLink
              :to="{ name: 'discover', query: { character_id: character.id } }"
              class="inline-flex items-center gap-1.5 text-xs text-muted transition hover:text-ink"
            >
              相关内容
              <ArrowUpRight class="h-3 w-3" />
            </RouterLink>
          </div>
        </div>
      </header>

      <section class="pt-10 md:pt-14">
        <div class="mb-5 flex items-end justify-between">
          <div>
            <div class="eyebrow mb-2">Stories</div>
            <h2 class="font-display text-2xl tracking-tight md:text-[28px]">与 {{ character.name }} 相关</h2>
          </div>
          <span class="text-[11px] text-muted">{{ detail?.contents?.length || 0 }} 条内容</span>
        </div>
        <div v-if="detail?.contents?.length" class="grid border-t border-line md:grid-cols-2 md:gap-x-10">
          <ContentCard v-for="item in detail.contents" :key="item.id" :content="item" />
        </div>
        <EmptyState
          v-else
          compact
          :icon="Film"
          title="暂无相关动态"
          description="相关公开内容会在审核通过后出现在这里。"
        />
      </section>
    </template>
  </div>
</template>
