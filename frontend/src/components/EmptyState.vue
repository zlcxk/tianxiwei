<script setup lang="ts">
import type { Component } from 'vue'
import { Inbox } from 'lucide-vue-next'

withDefaults(
  defineProps<{
    title: string
    description?: string
    icon?: Component
    actionLabel?: string
    to?: string
    compact?: boolean
  }>(),
  { description: '', icon: () => Inbox, actionLabel: '', to: '', compact: false },
)

defineEmits<{ (e: 'action'): void }>()
</script>

<template>
  <div
    class="flex flex-col items-center justify-center border border-dashed border-line bg-white/40 text-center"
    :class="compact ? 'px-5 py-10' : 'px-6 py-16 md:py-20'"
  >
    <span class="mb-4 flex h-11 w-11 items-center justify-center rounded-full bg-blush/50 text-[#9a6c72]">
      <component :is="icon" class="h-5 w-5" :stroke-width="1.5" />
    </span>
    <h3 class="font-display text-lg">{{ title }}</h3>
    <p v-if="description" class="mt-2 max-w-sm text-xs leading-6 text-muted">{{ description }}</p>
    <RouterLink v-if="to && actionLabel" :to="to" class="btn-ghost mt-5 !min-h-9 text-xs">
      {{ actionLabel }}
    </RouterLink>
    <button v-else-if="actionLabel" class="btn-ghost mt-5 !min-h-9 text-xs" @click="$emit('action')">
      {{ actionLabel }}
    </button>
    <slot />
  </div>
</template>
