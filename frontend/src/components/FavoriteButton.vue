<script setup lang="ts">
import { ref } from 'vue'
import { Heart } from 'lucide-vue-next'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api/client'
import { auth } from '../stores/auth'
import { toast } from '../stores/toast'

const props = withDefaults(
  defineProps<{
    targetType: 'content' | 'work' | 'character'
    targetId: string
    modelValue?: boolean
    label?: string
    compact?: boolean
  }>(),
  { modelValue: false, label: '收藏', compact: false },
)

const emit = defineEmits<{ (e: 'update:modelValue', value: boolean): void }>()
const router = useRouter()
const route = useRoute()
const pending = ref(false)
const favorited = ref(Boolean(props.modelValue))

async function toggle() {
  if (pending.value) return
  if (!auth.user) {
    router.push({ name: 'login', query: { redirect: route.fullPath } })
    return
  }
  pending.value = true
  const next = !favorited.value
  try {
    if (next) {
      await api.post('/favorites', { target_type: props.targetType, target_id: props.targetId })
    } else {
      await api.del(`/favorites?target_type=${props.targetType}&target_id=${props.targetId}`)
    }
    favorited.value = next
    emit('update:modelValue', next)
    toast.success(next ? '已加入收藏' : '已取消收藏')
  } catch (err) {
    toast.error(err instanceof Error ? err.message : '操作失败')
  } finally {
    pending.value = false
  }
}
</script>

<template>
  <button
    class="inline-flex items-center gap-2 transition"
    :class="compact ? 'text-muted hover:text-accent' : favorited ? 'text-accent' : 'text-muted hover:text-ink'"
    :aria-label="favorited ? '取消收藏' : '收藏'"
    :disabled="pending"
    @click.stop.prevent="toggle"
  >
    <Heart class="h-4 w-4" :fill="favorited ? 'currentColor' : 'none'" />
    <span v-if="!compact" class="text-xs">{{ favorited ? '已收藏' : label }}</span>
  </button>
</template>
