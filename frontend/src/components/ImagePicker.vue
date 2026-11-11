<script setup lang="ts">
import { computed, ref, useId, watch } from 'vue'
import { ImageOff, Loader2, Trash2, Upload } from 'lucide-vue-next'
import { uploadImage } from '../api/client'

const props = withDefaults(
  defineProps<{
    modelValue: string
    label?: string
    id?: string
    /** Preview aspect ratio, matching the slot the image will occupy. */
    aspect?: 'square' | 'portrait' | 'wide'
    hint?: string
  }>(),
  { label: '图片', id: undefined, aspect: 'square', hint: '' },
)
const emit = defineEmits<{ (e: 'update:modelValue', value: string): void }>()

const uid = useId()
const inputId = computed(() => props.id || `image-picker-${uid}`)
const value = computed({
  get: () => props.modelValue,
  set: (next: string) => emit('update:modelValue', next),
})

const fileInput = ref<HTMLInputElement | null>(null)
const uploading = ref(false)
const error = ref('')
const failed = ref(false)

watch(
  () => props.modelValue,
  () => {
    failed.value = false
  },
)

const aspectClass = computed(
  () =>
    ({
      square: 'aspect-square',
      portrait: 'aspect-[3/4]',
      wide: 'aspect-[16/9]',
    })[props.aspect],
)

function openPicker() {
  fileInput.value?.click()
}

async function onFileChange(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  error.value = ''
  if (!['image/jpeg', 'image/png', 'image/webp'].includes(file.type)) {
    error.value = '仅支持 JPEG、PNG 或 WebP 图片'
    return
  }
  if (file.size > 10 * 1024 * 1024) {
    error.value = '图片不能超过 10MB'
    return
  }
  uploading.value = true
  try {
    const uploaded = await uploadImage(file)
    value.value = uploaded.url
  } catch (err) {
    error.value = err instanceof Error ? err.message : '图片上传失败'
  } finally {
    uploading.value = false
  }
}

function clearImage() {
  value.value = ''
  error.value = ''
}
</script>

<template>
  <div class="space-y-2.5">
    <label :for="inputId" class="field-label">{{ label }}</label>
    <div class="flex flex-col gap-3 sm:flex-row sm:items-start">
      <div
        class="relative w-full max-w-[210px] shrink-0 overflow-hidden border border-line bg-blush/25"
        :class="aspectClass"
      >
        <img
          v-if="modelValue && !failed"
          :src="modelValue"
          alt=""
          class="h-full w-full object-cover"
          loading="lazy"
          decoding="async"
          @error="failed = true"
        />
        <div v-else class="flex h-full w-full flex-col items-center justify-center gap-2 p-4 text-center text-muted">
          <ImageOff class="h-5 w-5 opacity-60" />
          <span class="text-[10px] leading-4">
            {{ failed ? '图片加载失败，请更换地址' : '暂无图片' }}
          </span>
        </div>
        <span
          v-if="uploading"
          class="absolute inset-0 flex items-center justify-center bg-ink/45 text-white"
        >
          <Loader2 class="h-5 w-5 animate-spin" />
        </span>
      </div>

      <div class="min-w-0 flex-1 space-y-2">
        <input
          :id="inputId"
          v-model="value"
          class="field"
          placeholder="图片地址，或上传本地图片"
        />
        <div class="flex flex-wrap items-center gap-2">
          <button
            type="button"
            class="btn-ghost !min-h-8 !px-3 text-[11px]"
            :disabled="uploading"
            @click="openPicker"
          >
            <Upload class="h-3.5 w-3.5" />
            {{ modelValue ? '替换图片' : '上传图片' }}
          </button>
          <button
            v-if="modelValue"
            type="button"
            class="btn-ghost !min-h-8 !px-3 text-[11px]"
            :disabled="uploading"
            @click="clearImage"
          >
            <Trash2 class="h-3.5 w-3.5" />
            清除
          </button>
          <span class="text-[10px] text-muted">JPEG / PNG / WebP，≤10MB</span>
        </div>
        <p v-if="error" class="text-[11px] leading-5 text-[#a13d3d]">{{ error }}</p>
        <p v-else-if="hint" class="text-[10px] leading-5 text-muted">{{ hint }}</p>
        <input
          ref="fileInput"
          type="file"
          accept="image/jpeg,image/png,image/webp"
          class="hidden"
          @change="onFileChange"
        />
      </div>
    </div>
  </div>
</template>
