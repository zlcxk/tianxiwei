<script setup lang="ts">
import { computed } from 'vue'
import {
  CalendarDays,
  Clapperboard,
  Coffee,
  Compass,
  Gem,
  Globe2,
  Mic,
  Sparkles,
  TrendingUp,
} from 'lucide-vue-next'

const props = withDefaults(defineProps<{ code: string; class?: string }>(), { class: 'h-4 w-4' })

const map: Record<string, unknown> = {
  drama: Clapperboard,
  beauty: Sparkles,
  variety: Mic,
  daily: Coffee,
  fashion: Gem,
  career: TrendingUp,
  event: CalendarDays,
  all: Globe2,
}

const icon = computed(() => map[props.code] || Compass)
</script>

<template>
  <component :is="icon" :class="props.class" :stroke-width="1.6" />
</template>
