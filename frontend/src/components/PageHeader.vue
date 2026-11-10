<script setup lang="ts">
import { ArrowLeft } from 'lucide-vue-next'

withDefaults(
  defineProps<{
    eyebrow?: string
    title: string
    description?: string
    backTo?: string
    backLabel?: string
  }>(),
  { eyebrow: '', description: '', backTo: '', backLabel: '返回' },
)
</script>

<template>
  <header class="mb-8 md:mb-12">
    <RouterLink
      v-if="backTo"
      :to="backTo"
      class="mb-6 inline-flex items-center gap-2 text-xs text-muted transition hover:text-ink"
    >
      <ArrowLeft class="h-3.5 w-3.5" />
      {{ backLabel }}
    </RouterLink>
    <div class="flex flex-col gap-5 md:flex-row md:items-end md:justify-between">
      <div class="max-w-2xl">
        <div v-if="eyebrow" class="eyebrow mb-3">{{ eyebrow }}</div>
        <h1 class="display-title text-[34px] md:text-[46px]">{{ title }}</h1>
        <p v-if="description" class="mt-3 max-w-xl text-sm leading-7 text-muted">{{ description }}</p>
      </div>
      <div v-if="$slots.actions" class="flex flex-wrap items-center gap-2">
        <slot name="actions" />
      </div>
    </div>
  </header>
</template>
