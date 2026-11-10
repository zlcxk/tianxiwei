<script setup lang="ts">
import { onBeforeUnmount, watch } from 'vue'
import { X } from 'lucide-vue-next'

const props = withDefaults(
  defineProps<{ open: boolean; title?: string; description?: string; size?: 'sm' | 'md' | 'lg' | 'xl' }>(),
  { title: '', description: '', size: 'md' },
)
const emit = defineEmits<{ (e: 'close'): void }>()

const widths = { sm: 'max-w-sm', md: 'max-w-xl', lg: 'max-w-3xl', xl: 'max-w-5xl' }

function onKey(event: KeyboardEvent) {
  if (event.key === 'Escape') emit('close')
}

watch(
  () => props.open,
  (open) => {
    document.body.style.overflow = open ? 'hidden' : ''
    if (open) window.addEventListener('keydown', onKey)
    else window.removeEventListener('keydown', onKey)
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  document.body.style.overflow = ''
  window.removeEventListener('keydown', onKey)
})
</script>

<template>
  <Teleport to="body">
    <Transition name="modal">
      <div
        v-if="open"
        class="fixed inset-0 z-[90] flex items-end justify-center bg-ink/35 p-0 backdrop-blur-sm sm:items-center sm:p-6"
        role="dialog"
        aria-modal="true"
        :aria-label="title || '对话框'"
        @click.self="emit('close')"
      >
        <div
          class="flex max-h-[92dvh] w-full flex-col overflow-hidden border border-line bg-canvas shadow-lift"
          :class="[widths[size], 'sm:max-h-[86dvh]']"
        >
          <header v-if="title" class="flex items-start justify-between gap-4 border-b border-line px-5 py-4 md:px-6">
            <div>
              <h2 class="font-display text-xl">{{ title }}</h2>
              <p v-if="description" class="mt-1 text-xs leading-5 text-muted">{{ description }}</p>
            </div>
            <button class="btn-icon h-9 w-9 shrink-0 border-0 bg-transparent" aria-label="关闭" @click="emit('close')">
              <X class="h-4 w-4" />
            </button>
          </header>
          <div class="min-h-0 flex-1 overflow-y-auto px-5 py-5 md:px-6">
            <slot />
          </div>
          <footer v-if="$slots.footer" class="flex flex-wrap items-center justify-end gap-2 border-t border-line bg-white/50 px-5 py-3.5 md:px-6">
            <slot name="footer" />
          </footer>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.modal-enter-active,
.modal-leave-active {
  transition: opacity 0.2s ease;
}
.modal-enter-active > div,
.modal-leave-active > div {
  transition: transform 0.24s ease, opacity 0.24s ease;
}
.modal-enter-from,
.modal-leave-to {
  opacity: 0;
}
.modal-enter-from > div,
.modal-leave-to > div {
  opacity: 0;
  transform: translateY(14px);
}
@media (min-width: 640px) {
  .modal-enter-from > div,
  .modal-leave-to > div {
    transform: scale(0.985) translateY(6px);
  }
}
</style>
