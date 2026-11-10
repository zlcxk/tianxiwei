<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, Eye, EyeOff, Loader2, LockKeyhole } from 'lucide-vue-next'
import { auth, login, register } from '../stores/auth'

const route = useRoute()
const router = useRouter()

const mode = ref<'login' | 'register'>('login')
const email = ref('')
const password = ref('')
const nickname = ref('')
const showPassword = ref(false)
const submitting = ref(false)
const error = ref('')

const canRegister = computed(() => auth.allowRegistration)
const heading = computed(() => (mode.value === 'login' ? '回到你的空间' : '创建一个账号'))
const submitLabel = computed(() => (mode.value === 'login' ? '登录' : '注册并登录'))

function safeRedirect(): string {
  const redirect = route.query.redirect
  if (typeof redirect === 'string' && redirect.startsWith('/') && !redirect.startsWith('//')) return redirect
  return '/'
}

async function submit() {
  if (submitting.value) return
  error.value = ''
  if (!email.value.trim() || !password.value) {
    error.value = '请输入邮箱和密码'
    return
  }
  if (mode.value === 'register' && password.value.length < 10) {
    error.value = '密码至少需要 10 个字符'
    return
  }
  submitting.value = true
  try {
    if (mode.value === 'login') {
      await login(email.value.trim(), password.value)
    } else {
      await register(email.value.trim(), password.value, nickname.value.trim())
    }
    router.replace(safeRedirect())
  } catch (err) {
    error.value = err instanceof Error ? err.message : '操作失败，请稍后再试'
  } finally {
    submitting.value = false
  }
}

function switchMode() {
  mode.value = mode.value === 'login' ? 'register' : 'login'
  error.value = ''
}
</script>

<template>
  <div class="grid min-h-dvh lg:grid-cols-[.9fr_1.1fr]">
    <!-- Visual side -->
    <aside class="hero-canvas relative hidden flex-col justify-between overflow-hidden border-r border-line p-10 lg:flex">
      <RouterLink to="/" class="relative z-10 flex items-baseline gap-2.5">
        <span class="font-display text-xl tracking-tight">曦薇空间</span>
        <span class="text-[9px] uppercase tracking-widest2 text-ink/45">Xiwei Space</span>
      </RouterLink>

      <div class="relative z-10 max-w-md">
        <div class="text-[10px] uppercase tracking-widest2 text-ink/45">A Quiet Archive</div>
        <p class="mt-5 font-display text-[42px] leading-[1.08] tracking-[-0.03em] text-ink/85">
          把喜欢的事，<br />留在一个安静的地方。
        </p>
        <p class="mt-5 max-w-sm text-[12px] leading-7 text-ink/50">
          作品、角色、公开动态与可追溯的资料。这里不追逐喧闹，只认真整理值得被记住的部分。
        </p>
      </div>

      <div class="relative z-10 flex items-center justify-between text-[10px] tracking-wide text-ink/45">
        <span>TIAN XIWEI · 2026</span>
        <span class="vertical-text tracking-[0.3em]">XIWEI</span>
      </div>
    </aside>

    <!-- Form side -->
    <main class="flex flex-col">
      <div class="flex items-center justify-between px-5 py-5 md:px-10">
        <RouterLink to="/" class="inline-flex items-center gap-2 text-xs text-muted transition hover:text-ink">
          <ArrowLeft class="h-3.5 w-3.5" />
          返回首页
        </RouterLink>
        <span class="font-display text-base lg:hidden">曦薇空间</span>
        <span class="text-[9px] uppercase tracking-widest2 text-muted">Member Access</span>
      </div>

      <div class="flex flex-1 items-center justify-center px-5 pb-16 pt-6 md:px-10">
        <div class="w-full max-w-[400px]">
          <div class="mb-8">
            <div class="eyebrow mb-3">{{ mode === 'login' ? 'Welcome Back' : 'Join The Space' }}</div>
            <h1 class="display-title text-[34px] md:text-[40px]">{{ heading }}</h1>
            <p class="mt-3 text-xs leading-6 text-muted">
              {{ mode === 'login'
                ? '登录后可以收藏内容、保存兴趣偏好，并在不同设备同步。'
                : '注册后可以收藏内容与作品，并设置你的兴趣场景。' }}
            </p>
          </div>

          <form class="space-y-5" @submit.prevent="submit">
            <div v-if="mode === 'register'">
              <label for="nickname" class="field-label">昵称（可选）</label>
              <input
                id="nickname"
                v-model="nickname"
                type="text"
                maxlength="40"
                class="field"
                placeholder="怎么称呼你"
                autocomplete="nickname"
              />
            </div>
            <div>
              <label for="email" class="field-label">邮箱</label>
              <input
                id="email"
                v-model="email"
                type="email"
                class="field"
                placeholder="you@example.com"
                autocomplete="email"
                required
              />
            </div>
            <div>
              <div class="flex items-center justify-between">
                <label for="password" class="field-label">密码</label>
                <span v-if="mode === 'register'" class="text-[10px] text-muted">至少 10 个字符</span>
              </div>
              <div class="relative">
                <input
                  id="password"
                  v-model="password"
                  :type="showPassword ? 'text' : 'password'"
                  class="field pr-11"
                  placeholder="请输入密码"
                  :autocomplete="mode === 'login' ? 'current-password' : 'new-password'"
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

            <div v-if="error" class="border border-accent/25 bg-blush/30 px-3.5 py-2.5 text-[11px] leading-5 text-accent">
              {{ error }}
            </div>

            <button type="submit" class="btn-primary w-full" :disabled="submitting">
              <Loader2 v-if="submitting" class="h-4 w-4 animate-spin" />
              {{ submitting ? '请稍候' : submitLabel }}
            </button>
          </form>

          <div class="mt-6 flex items-center justify-between gap-4 border-t border-line pt-5 text-[11px]">
            <button v-if="canRegister" class="text-muted transition hover:text-ink" @click="switchMode">
              {{ mode === 'login' ? '没有账号？注册' : '已有账号？登录' }}
            </button>
            <span v-else class="text-muted">当前未开放注册，请使用管理员创建的账号</span>
            <RouterLink to="/setup" class="inline-flex items-center gap-1 text-muted transition hover:text-ink">
              <LockKeyhole class="h-3 w-3" />
              初始化
            </RouterLink>
          </div>

          <p class="mt-8 text-[10px] leading-5 text-muted">
            登录即表示你同意仅将本站用于公开信息浏览。本站不会索取与追星无关的个人信息。
          </p>
        </div>
      </div>
    </main>
  </div>
</template>
