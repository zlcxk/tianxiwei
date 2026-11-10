<script setup lang="ts">
withDefaults(defineProps<{ eyebrow?: string; title: string; description?: string }>(), {
  eyebrow: '',
  description: '',
})
</script>

<template>
  <header class="mb-7 flex flex-col gap-4 border-b border-line pb-5 md:mb-9 md:flex-row md:items-end md:justify-between">
    <div>
      <div v-if="eyebrow" class="eyebrow mb-2">{{ eyebrow }}</div>
      <h1 class="font-display text-[28px] leading-tight tracking-tight md:text-[34px]">{{ title }}</h1>
      <p v-if="description" class="mt-1.5 max-w-2xl text-[11px] leading-5 text-muted">{{ description }}</p>
    </div>
    <div v-if="$slots.actions" class="flex flex-wrap items-center gap-2">
      <slot name="actions" />
    </div>
  </header>
</template>
