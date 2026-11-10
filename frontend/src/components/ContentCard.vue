<script setup lang="ts">
import { computed } from 'vue'
import { ArrowUpRight, BadgeCheck } from 'lucide-vue-next'
import type { Content } from '../types'
import { contentTimeLabel, contentTypeLabels, trustLabels } from '../lib/format'
import CoverArt from './CoverArt.vue'
import SceneIcon from './SceneIcon.vue'

const props = withDefaults(
  defineProps<{
    content: Content
    variant?: 'row' | 'card'
    showSummary?: boolean
  }>(),
  { variant: 'row', showSummary: true },
)

const typeLabel = computed(() => contentTypeLabels[props.content.content_type] || '资讯')
const showSourceName = computed(
  () => Boolean(props.content.source_name) && props.content.source_name !== typeLabel.value,
)
</script>

<template>
  <RouterLink
    :to="{ name: 'content', params: { id: content.id } }"
    class="group block"
    :class="variant === 'row' ? 'border-b border-line py-5' : ''"
  >
    <article class="flex gap-4 md:gap-5">
      <CoverArt
        :title="content.title"
        :subtitle="typeLabel"
        :src="content.cover_url"
        :class="variant === 'row' ? 'w-[88px] shrink-0 md:w-28' : 'w-full'"
        aspect="square"
      />
      <div class="flex min-w-0 flex-1 flex-col justify-between py-0.5">
        <div>
          <div class="flex flex-wrap items-center gap-x-2 gap-y-1 text-[10px] tracking-wide text-muted">
            <span class="uppercase tracking-[0.14em]">{{ typeLabel }}</span>
            <span v-if="showSourceName" class="h-2.5 w-px bg-line" />
            <span v-if="showSourceName" class="truncate">{{ content.source_name }}</span>
            <span v-if="content.is_official" class="inline-flex items-center gap-0.5 text-[#9a6c72]">
              <BadgeCheck class="h-3 w-3" />官方
            </span>
            <span v-if="content.published_precision !== 'verified'" class="text-[9px] text-[#9a6c72]">
              {{ content.published_precision === 'unknown' ? '时间未知' : '时间待核验' }}
            </span>
          </div>
          <h3 class="mt-2 line-clamp-2 font-display text-[17px] leading-[1.35] tracking-tight transition group-hover:text-accent md:text-lg">
            {{ content.title }}
          </h3>
          <p v-if="showSummary && content.summary" class="mt-1.5 line-clamp-2 hidden text-xs leading-[1.7] text-muted sm:block">
            {{ content.summary }}
          </p>
        </div>
        <div class="mt-2.5 flex items-end justify-between gap-3">
          <div class="flex flex-wrap gap-1.5">
            <span
              v-for="scene in content.scenes.slice(0, 2)"
              :key="scene.id"
              class="inline-flex items-center gap-1 text-[10px] text-muted"
            >
              <SceneIcon :code="scene.code" class="h-3 w-3" />
              {{ scene.name }}
            </span>
            <span v-if="content.scenes.length > 2" class="text-[10px] text-muted">+{{ content.scenes.length - 2 }}</span>
          </div>
          <div class="flex shrink-0 items-center gap-2 text-[10px] text-muted">
            <span class="hidden sm:inline">{{ trustLabels[content.trust_level] || content.trust_level }}</span>
            <time>{{ contentTimeLabel(content) }}</time>
            <ArrowUpRight class="h-3.5 w-3.5 opacity-0 transition group-hover:opacity-60" />
          </div>
        </div>
      </div>
    </article>
  </RouterLink>
</template>
