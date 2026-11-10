<script setup lang="ts">
import { ArrowLeft } from 'lucide-vue-next'
</script>

<template>
  <div class="shell flex min-h-[60vh] flex-col items-center justify-center py-20 text-center">
    <div class="font-display text-[88px] leading-none tracking-tight text-ink/15">404</div>
    <h1 class="mt-4 font-display text-2xl tracking-tight">这一页不在档案里</h1>
    <p class="mt-2 max-w-sm text-xs leading-6 text-muted">链接可能已经失效，或者内容尚未公开。</p>
    <RouterLink to="/" class="btn-primary mt-7 !min-h-10 text-xs">
      <ArrowLeft class="h-3.5 w-3.5" />
      回到首页
    </RouterLink>
  </div>
</template>
