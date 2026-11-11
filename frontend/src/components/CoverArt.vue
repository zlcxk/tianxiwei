<script setup lang="ts">
import { computed, ref, watch } from 'vue'

const props = withDefaults(
  defineProps<{
    title: string
    subtitle?: string
    meta?: string
    aspect?: 'portrait' | 'square' | 'wide' | 'auto'
    src?: string
    tone?: number
    /** Above-the-fold images load eagerly and at high priority. */
    priority?: boolean
  }>(),
  { subtitle: '', meta: '', aspect: 'square', src: '', tone: undefined, priority: false },
)

// A broken remote image must fall back to the editorial template instead of
// showing the browser's broken-image icon. The container keeps its aspect ratio
// throughout, so a failed image never shifts the layout.
const failed = ref(false)
watch(
  () => props.src,
  () => {
    failed.value = false
  },
)
function onError() {
  failed.value = true
}
const showImage = computed(() => Boolean(props.src) && !failed.value)

const palettes = [
  { bg: 'linear-gradient(155deg, #ead8d0 0%, #d9bab4 52%, #c08f93 100%)', ink: '#4b3638', line: 'rgba(75,54,56,.22)' },
  { bg: 'linear-gradient(150deg, #dfe3d6 0%, #c3cbb9 55%, #9aa58f 100%)', ink: '#3b4237', line: 'rgba(59,66,55,.2)' },
  { bg: 'linear-gradient(150deg, #ece2d3 0%, #d8c5a8 55%, #b69d7f 100%)', ink: '#4a3e30', line: 'rgba(74,62,48,.22)' },
  { bg: 'linear-gradient(150deg, #e3dce7 0%, #c9bfd3 55%, #a598b4 100%)', ink: '#403846', line: 'rgba(64,56,70,.2)' },
  { bg: 'linear-gradient(150deg, #e5e0d8 0%, #cbc3b7 50%, #8f8a82 100%)', ink: '#38342f', line: 'rgba(56,52,47,.22)' },
  { bg: 'linear-gradient(150deg, #ecd9d3 0%, #d9b3a9 50%, #a98679 100%)', ink: '#4a3731', line: 'rgba(74,55,49,.2)' },
]

const palette = computed(() => {
  if (props.tone !== undefined) return palettes[props.tone % palettes.length]
  let hash = 0
  for (const ch of props.title) hash = (hash * 31 + ch.charCodeAt(0)) >>> 0
  return palettes[hash % palettes.length]
})

const aspectClass = computed(
  () =>
    ({
      portrait: 'aspect-[3/4]',
      square: 'aspect-square',
      wide: 'aspect-[16/10]',
      auto: 'h-full min-h-full',
    })[props.aspect],
)

const initial = computed(() => props.title.trim().slice(0, 1) || '曦')
</script>

<template>
  <div
    class="cover-art grain flex select-none flex-col justify-between p-3.5 text-left"
    :class="aspectClass"
    :style="{ background: palette.bg, color: palette.ink }"
  >
    <img
      v-if="showImage"
      :src="src"
      :alt="title"
      class="absolute inset-0 h-full w-full object-cover"
      :loading="priority ? 'eager' : 'lazy'"
      :fetchpriority="priority ? 'high' : 'auto'"
      decoding="async"
      @error="onError"
    />
    <template v-else>
      <div class="relative z-10 flex items-start justify-between gap-2">
        <span class="text-[9px] uppercase leading-none tracking-[0.18em] opacity-70">Xiwei Archive</span>
        <span class="h-1 w-1 rounded-full bg-current opacity-40" />
      </div>
      <span
        class="pointer-events-none absolute -right-3 top-8 z-0 font-display text-[104px] leading-none opacity-[0.13]"
        aria-hidden="true"
      >{{ initial }}</span>
      <div class="relative z-10">
        <span class="mb-1.5 block h-px w-6" :style="{ background: palette.line }" />
        <div v-if="subtitle" class="mb-1 text-[9px] uppercase tracking-[0.16em] opacity-75">{{ subtitle }}</div>
        <div class="font-display text-[17px] leading-[1.2] tracking-tight">{{ title }}</div>
        <div v-if="meta" class="mt-1.5 text-[9px] tracking-wide opacity-70">{{ meta }}</div>
      </div>
    </template>
  </div>
</template>
