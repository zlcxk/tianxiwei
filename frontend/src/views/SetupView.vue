<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ArrowRight, Check, Eye, EyeOff, Loader2, ShieldCheck } from 'lucide-vue-next'
import { setupAdmin } from '../stores/auth'

const router = useRouter()

const email = ref('')
const nickname = ref('')
const password = ref('')
const confirm = ref('')
const showPassword = ref(false)
const submitting = ref(false)
const error = ref('')

const checks = computed(() => [
  { label: '至少 10 个字符', ok: password.value.length >= 10 },
  { label: '两次输入一致', ok: password.value.length > 0 && password.value === confirm.value },
  { label: '邮箱格式有效', ok: /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.value.trim()) },
])

const valid = computed(() => checks.value.every((check) => check.ok))

async function submit() {
  if (submitting.value || !valid.value) return
  error.value = ''
  submitting.value = true
  try {
    await setupAdmin(email.value.trim(), password.value, nickname.value.trim())
    router.replace('/admin')
  } catch (err) {
    error.value = err instanceof Error ? err.message : '初始化失败，请稍后再试'
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="grid min-h-dvh lg:grid-cols-[.85fr_1.15fr]">
    <aside class="hero-canvas relative hidden flex-col justify-between overflow-hidden border-r border-line p-10 lg:flex">
      <div class="relative z-10 flex items-baseline gap-2.5">
        <span class="font-display text-xl">曦薇空间</span>
        <span class="text-[9px] uppercase tracking-widest2 text-ink/45">First Run</span>
      </div>
      <div class="relative z-10 max-w-sm">
        <div class="mb-5 flex h-11 w-11 items-center justify-center rounded-full border border-ink/15">
          <ShieldCheck class="h-5 w-5 text-ink/60" />
        </div>
        <p class="font-display text-[36px] leading-[1.12] tracking-tight text-ink/85">
          先建立管理员，<br />再开始整理。
        </p>
        <p class="mt-4 text-[12px] leading-7 text-ink/55">
          这是站点的首次启动。初始化完成后，该入口会自动关闭，后续请使用你的管理员账号登录。
        </p>
      </div>
      <div class="relative z-10 text-[10px] uppercase tracking-widest2 text-ink/40">No Default Password · No Shared Account</div>
    </aside>

    <main class="flex flex-col">
      <div class="flex items-center justify-between px-5 py-5 md:px-10">
        <span class="font-display text-base lg:hidden">曦薇空间</span>
        <span class="ml-auto text-[9px] uppercase tracking-widest2 text-muted">Initial Setup</span>
      </div>

      <div class="flex flex-1 items-center justify-center px-5 pb-16 pt-4 md:px-10">
        <div class="w-full max-w-[420px]">
          <div class="mb-8">
            <div class="eyebrow mb-3">Step 01 / 01</div>
            <h1 class="display-title text-[34px] md:text-[40px]">初始化管理员</h1>
            <p class="mt-3 text-xs leading-6 text-muted">
              创建第一个管理员账号。系统不使用固定默认密码，请设置一个只有你知道的密码。
            </p>
          </div>

          <form class="space-y-5" @submit.prevent="submit">
            <div>
              <label for="email" class="field-label">管理员邮箱</label>
              <input
                id="email"
                v-model="email"
                type="email"
                class="field"
                placeholder="admin@example.com"
                autocomplete="email"
                required
              />
            </div>
            <div>
              <label for="nickname" class="field-label">显示昵称（可选）</label>
              <input
                id="nickname"
                v-model="nickname"
                type="text"
                maxlength="40"
                class="field"
                placeholder="例如：空间管理员"
                autocomplete="nickname"
              />
            </div>
            <div>
              <label for="password" class="field-label">密码</label>
              <div class="relative">
                <input
                  id="password"
                  v-model="password"
                  :type="showPassword ? 'text' : 'password'"
                  class="field pr-11"
                  placeholder="至少 10 个字符"
                  autocomplete="new-password"
                  required
                />
                <button
                  type="button"
                  class="absolute right-2 top-1/2 flex h-8 w-8 -translate-y-1/2 items-center justify-center text-muted hover:text-ink"
                  :aria-label="showPassword ? '隐藏密码' : '显示密码'"
                  @click="showPassword = !showPassword"
                >
                  <EyeOff v-if="showPassword" class="h-4 w-4" />
                  <Eye v-else class="h-4 w-4" />
                </button>
              </div>
            </div>
            <div>
              <label for="confirm" class="field-label">确认密码</label>
              <input
                id="confirm"
                v-model="confirm"
                :type="showPassword ? 'text' : 'password'"
                class="field"
                placeholder="再次输入密码"
                autocomplete="new-password"
                required
              />
            </div>

            <ul class="flex flex-wrap gap-x-4 gap-y-2 text-[10px]">
              <li
                v-for="check in checks"
                :key="check.label"
                class="inline-flex items-center gap-1 transition"
                :class="check.ok ? 'text-moss' : 'text-muted'"
              >
                <span
                  class="flex h-3.5 w-3.5 items-center justify-center rounded-full border"
                  :class="check.ok ? 'border-moss bg-moss text-white' : 'border-line'"
                >
                  <Check v-if="check.ok" class="h-2.5 w-2.5" />
                </span>
                {{ check.label }}
              </li>
            </ul>

            <div v-if="error" class="border border-accent/25 bg-blush/30 px-3.5 py-2.5 text-[11px] leading-5 text-accent">
              {{ error }}
            </div>

            <button type="submit" class="btn-primary w-full" :disabled="submitting || !valid">
              <Loader2 v-if="submitting" class="h-4 w-4 animate-spin" />
              {{ submitting ? '正在创建' : '创建管理员并进入后台' }}
              <ArrowRight v-if="!submitting" class="h-4 w-4" />
            </button>
          </form>

          <p class="mt-8 border-t border-line pt-5 text-[10px] leading-5 text-muted">
            初始化完成后，请妥善保管管理员密码。若需要开放普通用户注册，请通过服务端环境变量
            <code class="bg-white/60 px-1 py-0.5">ALLOW_REGISTRATION=true</code> 开启。
          </p>
        </div>
      </div>
    </main>
  </div>
</template>
