<script setup lang="ts">
withDefaults(defineProps<{ rows?: number; type?: 'list' | 'grid' | 'text' }>(), {
  rows: 4,
  type: 'list',
})
</script>

<template>
  <div v-if="type === 'grid'" class="grid grid-cols-2 gap-4 md:grid-cols-3 lg:grid-cols-4">
    <div v-for="i in rows" :key="i">
      <div class="skeleton aspect-[3/4] w-full" />
      <div class="skeleton mt-3 h-4 w-3/4" />
      <div class="skeleton mt-2 h-3 w-1/2" />
    </div>
  </div>
  <div v-else-if="type === 'text'" class="space-y-3">
    <div v-for="i in rows" :key="i" class="skeleton h-4" :style="{ width: `${92 - i * 7}%` }" />
  </div>
  <div v-else class="divide-y divide-line">
    <div v-for="i in rows" :key="i" class="flex gap-4 py-5">
      <div class="skeleton h-24 w-24 shrink-0" />
      <div class="flex-1 space-y-2.5 py-1">
        <div class="skeleton h-3 w-24" />
        <div class="skeleton h-4 w-4/5" />
        <div class="skeleton h-3 w-full" />
        <div class="skeleton h-3 w-1/3" />
      </div>
    </div>
  </div>
</template>
