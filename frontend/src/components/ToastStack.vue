<script setup lang="ts">
import { CircleCheck, CircleAlert, Info, X } from 'lucide-vue-next'
import { dismissToast, toasts } from '../stores/toast'
</script>

<template>
  <Teleport to="body">
    <div class="pointer-events-none fixed inset-x-0 bottom-24 z-[80] flex flex-col items-center gap-2 px-4 md:bottom-8">
      <TransitionGroup name="toast">
        <div
          v-for="item in toasts"
          :key="item.id"
          class="pointer-events-auto flex w-full max-w-sm items-start gap-3 border border-line bg-white px-4 py-3 text-sm shadow-soft"
        >
          <CircleCheck v-if="item.kind === 'success'" class="mt-0.5 h-4 w-4 shrink-0 text-moss" />
          <CircleAlert v-else-if="item.kind === 'error'" class="mt-0.5 h-4 w-4 shrink-0 text-accent" />
          <Info v-else class="mt-0.5 h-4 w-4 shrink-0 text-muted" />
          <p class="flex-1 leading-5">{{ item.message }}</p>
          <button class="text-muted transition hover:text-ink" aria-label="关闭提示" @click="dismissToast(item.id)">
            <X class="h-4 w-4" />
          </button>
        </div>
      </TransitionGroup>
    </div>
  </Teleport>
</template>

<style scoped>
.toast-enter-active,
.toast-leave-active {
  transition: opacity 0.2s ease, transform 0.2s ease;
}
.toast-enter-from,
.toast-leave-to {
  opacity: 0;
  transform: translateY(8px);
}
</style>
