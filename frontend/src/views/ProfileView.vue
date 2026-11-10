<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import {
  ArrowUpRight,
  Check,
  Heart,
  LayoutDashboard,
  LogOut,
  Settings2,
  ShieldCheck,
  Sparkles,
  Star,
} from 'lucide-vue-next'
import { api } from '../api/client'
import type { FavoriteItem, Scene } from '../types'
import { auth, isAdmin, logout } from '../stores/auth'
import { toast } from '../stores/toast'
import { formatDate } from '../lib/format'
import LoadingBlock from '../components/LoadingBlock.vue'
import SceneIcon from '../components/SceneIcon.vue'

const router = useRouter()

type PrefRow = {
  scene: Scene
  enabled: boolean
  weight: number
  isDefault: boolean
}

const prefs = ref<PrefRow[]>([])
const prefLoading = ref(false)
const saving = ref(false)
const favoriteCount = ref(0)
const loggingOut = ref(false)

const enabledCount = computed(() => prefs.value.filter((p) => p.enabled).length)
const roleLabel = computed(() => {
  const role = auth.user?.role
  return role === 'admin' ? '管理员' : role === 'editor' ? '编辑' : '用户'
})

function mapScenes(items: Scene[]): PrefRow[] {
  return items.map((scene) => ({
    scene,
    enabled: scene.code === 'all' ? true : Boolean(scene.selected),
    weight: scene.weight && scene.weight > 0 ? scene.weight : 3,
    isDefault: Boolean(scene.is_default),
  }))
}

async function loadPrefs() {
  if (!auth.user) {
    prefs.value = []
    return
  }
  prefLoading.value = true
  try {
    const [sceneRes, favRes] = await Promise.all([
      api.get<{ items: Scene[] }>('/preferences/scenes'),
      api.get<{ items: FavoriteItem[] }>('/favorites'),
    ])
    prefs.value = mapScenes(sceneRes.items)
    favoriteCount.value = favRes.items.length
  } catch (err) {
    toast.error(err instanceof Error ? err.message : '读取个人设置失败')
  } finally {
    prefLoading.value = false
  }
}

function toggle(row: PrefRow) {
  if (row.scene.code === 'all') return
  row.enabled = !row.enabled
  if (!row.enabled && row.isDefault) row.isDefault = false
}

function setDefault(row: PrefRow) {
  if (!row.enabled) return
  prefs.value.forEach((p) => {
    p.isDefault = p.scene.id === row.scene.id
  })
}

async function save() {
  if (saving.value) return
  saving.value = true
  try {
    const choices = prefs.value.filter((p) => p.enabled)
    if (choices.length && !choices.some((p) => p.isDefault)) choices[0].isDefault = true
    const body = {
      preferences: choices.map((p) => ({
        scene_id: p.scene.id,
        weight: p.weight,
        enabled: true,
        is_default: p.isDefault,
      })),
    }
    const res = await api.put<{ items: Scene[] }>('/preferences/scenes', body)
    prefs.value = mapScenes(res.items)
    toast.success('兴趣设置已保存')
  } catch (err) {
    toast.error(err instanceof Error ? err.message : '保存失败')
  } finally {
    saving.value = false
  }
}

async function signOut() {
  loggingOut.value = true
  await logout()
  loggingOut.value = false
  toast.success('已退出登录')
  router.push('/')
}

onMounted(loadPrefs)
watch(() => auth.user?.id, loadPrefs)
</script>

<template>
  <div class="shell pb-16 pt-8 md:pb-24 md:pt-12">
    <header class="mb-8">
      <div class="eyebrow mb-3">My Space</div>
      <h1 class="display-title text-[36px] md:text-[48px]">我的</h1>
    </header>

    <!-- Guest -->
    <template v-if="!auth.user">
      <section class="hero-canvas border border-line">
        <div class="relative z-10 flex max-w-xl flex-col gap-6 p-6 md:p-10">
          <div>
            <div class="eyebrow mb-3">A Place For Yourself</div>
            <h2 class="font-display text-[30px] leading-tight tracking-tight md:text-[40px]">
              登录后，把喜欢的内容留在自己的空间。
            </h2>
            <p class="mt-3 max-w-md text-[13px] leading-7 text-ink/60">
              收藏动态、作品与角色，设置兴趣场景，让推荐更贴近你的偏好。
            </p>
          </div>
          <div class="flex flex-wrap items-center gap-3">
            <RouterLink to="/login" class="btn-primary">登录账号</RouterLink>
            <RouterLink to="/discover" class="btn-ghost">先随便看看</RouterLink>
          </div>
        </div>
      </section>

      <section class="mt-10 grid gap-4 md:grid-cols-3">
        <div class="border border-line bg-white/40 p-5">
          <Heart class="h-4 w-4 text-accent" />
          <h3 class="mt-4 font-display text-lg">收藏与快照</h3>
          <p class="mt-2 text-[11px] leading-6 text-muted">收藏时保留标题与来源快照，原内容下架后仍可识别。</p>
        </div>
        <div class="border border-line bg-white/40 p-5">
          <Sparkles class="h-4 w-4 text-accent" />
          <h3 class="mt-4 font-display text-lg">场景化推荐</h3>
          <p class="mt-2 text-[11px] leading-6 text-muted">剧粉、颜值粉、综艺粉……选择你关心的场景，调整推荐权重。</p>
        </div>
        <div class="border border-line bg-white/40 p-5">
          <ShieldCheck class="h-4 w-4 text-moss" />
          <h3 class="mt-4 font-display text-lg">来源可追溯</h3>
          <p class="mt-2 text-[11px] leading-6 text-muted">每条内容保留来源与原文链接，重要信息经人工审核后公开。</p>
        </div>
      </section>

      <p class="mt-8 text-[10px] leading-6 text-muted">
        本应用仅面向艺人公开信息整理，不提供非公开行程、私人信息或未授权影像。
      </p>
    </template>

    <!-- Signed in -->
    <template v-else>
      <section class="border border-line bg-white/40">
        <div class="flex flex-col gap-6 p-5 sm:flex-row sm:items-center sm:justify-between md:p-7">
          <div class="flex items-center gap-4">
            <span class="flex h-16 w-16 shrink-0 items-center justify-center rounded-full bg-blush font-display text-3xl text-[#8f5e66]">
              {{ auth.user.nickname.slice(0, 1) || '我' }}
            </span>
            <div>
              <div class="flex flex-wrap items-center gap-2">
                <h2 class="font-display text-2xl tracking-tight">{{ auth.user.nickname }}</h2>
                <span class="tag">{{ roleLabel }}</span>
              </div>
              <p class="mt-1 text-xs text-muted">{{ auth.user.email }}</p>
              <p class="mt-1 text-[10px] text-muted">加入于 {{ formatDate(auth.user.created_at) }}</p>
            </div>
          </div>
          <div class="flex items-center gap-6 sm:gap-8">
            <div class="text-center">
              <div class="font-display text-2xl">{{ favoriteCount }}</div>
              <div class="mt-0.5 text-[9px] uppercase tracking-widest text-muted">收藏</div>
            </div>
            <div class="text-center">
              <div class="font-display text-2xl">{{ enabledCount }}</div>
              <div class="mt-0.5 text-[9px] uppercase tracking-widest text-muted">兴趣场景</div>
            </div>
            <RouterLink to="/favorites" class="btn-ghost !min-h-9 !px-3.5 text-xs">
              查看收藏
              <ArrowUpRight class="h-3 w-3" />
            </RouterLink>
          </div>
        </div>
      </section>

      <!-- Preferences -->
      <section class="mt-8 border border-line">
        <div class="flex flex-col gap-3 border-b border-line px-5 py-4 sm:flex-row sm:items-center sm:justify-between md:px-7">
          <div>
            <div class="eyebrow">Preferences</div>
            <h2 class="mt-1 font-display text-xl">我的兴趣场景</h2>
          </div>
          <p class="text-[10px] leading-5 text-muted">开启感兴趣的场景并调整权重，影响首页与发现页推荐排序。</p>
        </div>

        <div v-if="prefLoading" class="p-5 md:p-7">
          <LoadingBlock type="text" :rows="5" />
        </div>

        <div v-else class="divide-y divide-line">
          <div
            v-for="row in prefs"
            :key="row.scene.id"
            class="px-5 py-4 transition md:px-7"
            :class="row.enabled ? 'bg-white/30' : 'bg-transparent'"
          >
            <div class="flex flex-wrap items-center gap-x-4 gap-y-3">
              <button
                class="flex min-w-0 flex-1 items-center gap-3 text-left"
                :aria-pressed="row.enabled"
                @click="toggle(row)"
              >
                <span
                  class="flex h-10 w-10 shrink-0 items-center justify-center border transition"
                  :class="row.enabled ? 'border-accent/40 bg-blush/50 text-[#8f5e66]' : 'border-line bg-white/40 text-muted'"
                >
                  <SceneIcon :code="row.scene.code" class="h-4 w-4" />
                </span>
                <span class="min-w-0">
                  <span class="flex flex-wrap items-center gap-2">
                    <span class="text-[13px] font-medium">{{ row.scene.name }}</span>
                    <span v-if="row.isDefault" class="inline-flex items-center gap-0.5 text-[9px] uppercase tracking-widest text-[#9a6c72]">
                      <Star class="h-2.5 w-2.5" fill="currentColor" />默认
                    </span>
                    <span v-if="row.scene.code === 'all'" class="text-[9px] uppercase tracking-widest text-muted">始终开启</span>
                  </span>
                  <span class="mt-0.5 block truncate text-[10px] text-muted">{{ row.scene.description }}</span>
                </span>
              </button>

              <div class="flex flex-wrap items-center gap-3 sm:gap-5">
                <div v-if="row.enabled && row.scene.code !== 'all'" class="flex items-center gap-1.5">
                  <span class="mr-0.5 text-[9px] uppercase tracking-widest text-muted">权重</span>
                  <button
                    v-for="n in 5"
                    :key="n"
                    class="h-6 w-6 border text-[10px] transition"
                    :class="row.weight >= n ? 'border-accent/50 bg-accent/80 text-white' : 'border-line bg-white text-muted hover:border-ink/30'"
                    :aria-label="`权重 ${n}`"
                    @click="row.weight = n"
                  >
                    {{ n }}
                  </button>
                </div>
                <button
                  v-if="row.enabled && row.scene.code !== 'all'"
                  class="inline-flex items-center gap-1 text-[10px] transition"
                  :class="row.isDefault ? 'text-[#9a6c72]' : 'text-muted hover:text-ink'"
                  @click="setDefault(row)"
                >
                  <Check v-if="row.isDefault" class="h-3 w-3" />
                  {{ row.isDefault ? '默认场景' : '设为默认' }}
                </button>
                <button
                  class="relative h-6 w-11 rounded-full transition"
                  :class="row.enabled ? 'bg-accent' : 'bg-line'"
                  :aria-pressed="row.enabled"
                  :aria-label="`${row.enabled ? '关闭' : '开启'}${row.scene.name}`"
                  :disabled="row.scene.code === 'all'"
                  @click="toggle(row)"
                >
                  <span
                    class="absolute top-1 h-4 w-4 rounded-full bg-white shadow transition"
                    :class="row.enabled ? 'left-6' : 'left-1'"
                  />
                </button>
              </div>
            </div>
          </div>
        </div>

        <div class="flex flex-wrap items-center justify-between gap-3 border-t border-line bg-white/50 px-5 py-4 md:px-7">
          <p class="text-[10px] text-muted">
            {{ enabledCount ? `已开启 ${enabledCount} 个场景` : '还未开启场景，推荐将按时间与来源可信度排序' }}
          </p>
          <button class="btn-primary !min-h-9 !px-4 text-xs" :disabled="saving" @click="save">
            {{ saving ? '保存中' : '保存设置' }}
          </button>
        </div>
      </section>

      <!-- Account -->
      <section class="mt-8 grid gap-4 md:grid-cols-2">
        <div class="border border-line bg-white/40">
          <div class="flex items-center gap-2 border-b border-line px-5 py-3.5">
            <Settings2 class="h-3.5 w-3.5 text-muted" />
            <h2 class="text-[13px] font-medium">账号信息</h2>
          </div>
          <dl class="divide-y divide-line text-xs">
            <div class="flex items-center justify-between px-5 py-3">
              <dt class="text-muted">昵称</dt>
              <dd>{{ auth.user.nickname }}</dd>
            </div>
            <div class="flex items-center justify-between px-5 py-3">
              <dt class="text-muted">邮箱</dt>
              <dd class="truncate pl-4">{{ auth.user.email }}</dd>
            </div>
            <div class="flex items-center justify-between px-5 py-3">
              <dt class="text-muted">权限</dt>
              <dd>{{ roleLabel }}</dd>
            </div>
          </dl>
        </div>

        <div class="border border-line bg-white/40">
          <div class="flex items-center gap-2 border-b border-line px-5 py-3.5">
            <ShieldCheck class="h-3.5 w-3.5 text-muted" />
            <h2 class="text-[13px] font-medium">数据与隐私</h2>
          </div>
          <p class="px-5 py-4 text-[11px] leading-6 text-muted">
            收藏快照与兴趣设置仅用于你的个人体验。本站不采集与追星无关的个人信息，也不展示非公开行程或私人内容。
          </p>
          <div class="flex flex-wrap items-center gap-2 border-t border-line px-5 py-3">
            <RouterLink v-if="isAdmin" to="/admin" class="btn-soft !min-h-8 !px-3 text-[11px]">
              <LayoutDashboard class="h-3 w-3" />
              管理后台
            </RouterLink>
            <button class="btn-ghost !min-h-8 !px-3 text-[11px] text-accent" :disabled="loggingOut" @click="signOut">
              <LogOut class="h-3 w-3" />
              退出登录
            </button>
          </div>
        </div>
      </section>
    </template>
  </div>
</template>
