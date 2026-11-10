<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import {
  CheckCircle2,
  Database,
  Pencil,
  Play,
  Plus,
  RefreshCw,
  Search,
  Trash2,
  XCircle,
} from 'lucide-vue-next'
import { api } from '../../api/client'
import type { SearchProvider, SearchRun, SearchTask, SearchTestResult } from '../../types'
import {
  contentTypeLabels,
  formatDateTime,
  relativeTime,
  sourceTypeLabels,
  trustLabels,
} from '../../lib/format'
import { toast } from '../../stores/toast'
import AdminPageHeader from './AdminPageHeader.vue'
import EmptyState from '../../components/EmptyState.vue'
import LoadingBlock from '../../components/LoadingBlock.vue'
import Modal from '../../components/Modal.vue'

const providers = ref<SearchProvider[]>([])
const tasks = ref<SearchTask[]>([])
const runs = ref<SearchRun[]>([])
const loading = ref(true)
const error = ref('')
const saving = ref(false)
const testingId = ref('')
const runningId = ref('')

const providerOpen = ref(false)
const providerEditingId = ref('')
const taskOpen = ref(false)
const taskEditingId = ref('')

const typeOptions = Object.entries(contentTypeLabels)
const sourceTypeOptions = Object.entries(sourceTypeLabels)
const trustOptions = Object.entries(trustLabels)

const providerForm = reactive({
  name: '',
  provider_type: 'bing_rss',
  endpoint: 'https://www.bing.com/search',
  api_key_env: '',
  enabled: true,
})

const taskForm = reactive({
  name: '',
  provider_id: '',
  query: '田曦薇',
  domains_text: '',
  content_type_hint: '',
  source_type: 'media',
  trust_level: 'media',
  interval_minutes: 360,
  max_results: 20,
  enabled: true,
})

function resetProviderForm() {
  Object.assign(providerForm, {
    name: '',
    provider_type: 'bing_rss',
    endpoint: 'https://www.bing.com/search',
    api_key_env: '',
    enabled: true,
  })
}

function openProviderCreate() {
  providerEditingId.value = ''
  resetProviderForm()
  providerOpen.value = true
}

function openProviderEdit(provider: SearchProvider) {
  providerEditingId.value = provider.id
  Object.assign(providerForm, {
    name: provider.name,
    provider_type: provider.provider_type,
    endpoint: provider.endpoint,
    api_key_env: provider.api_key_env,
    enabled: provider.enabled,
  })
  providerOpen.value = true
}

function onProviderTypeChange() {
  if (!providerEditingId.value) {
    providerForm.endpoint =
      providerForm.provider_type === 'searxng'
        ? 'http://127.0.0.1:8080'
        : 'https://www.bing.com/search'
    if (providerForm.provider_type === 'searxng') providerForm.api_key_env = ''
  }
}

async function saveProvider() {
  if (saving.value) return
  if (!providerForm.name.trim()) {
    toast.error('请填写搜索服务名称')
    return
  }
  saving.value = true
  try {
    const payload = {
      name: providerForm.name.trim(),
      provider_type: providerForm.provider_type,
      endpoint: providerForm.endpoint.trim(),
      api_key_env: providerForm.api_key_env.trim(),
      enabled: providerForm.enabled,
    }
    if (providerEditingId.value) {
      await api.patch(`/admin/search/providers/${providerEditingId.value}`, payload)
      toast.success('搜索服务已更新')
    } else {
      await api.post('/admin/search/providers', payload)
      toast.success('搜索服务已创建')
    }
    providerOpen.value = false
    await load()
  } catch (err) {
    toast.error(err instanceof Error ? err.message : '保存搜索服务失败')
  } finally {
    saving.value = false
  }
}

async function toggleProvider(provider: SearchProvider) {
  try {
    await api.patch(`/admin/search/providers/${provider.id}`, { enabled: !provider.enabled })
    toast.success(provider.enabled ? '已停用搜索服务' : '已启用搜索服务')
    await load()
  } catch (err) {
    toast.error(err instanceof Error ? err.message : '操作失败')
  }
}

async function testProvider(provider: SearchProvider) {
  if (testingId.value) return
  testingId.value = provider.id
  try {
    const result = await api.post<SearchTestResult>(`/admin/search/providers/${provider.id}/test`, {})
    if (result.ok) {
      toast.success(`连接成功 · ${result.result_count} 条结果 · ${result.latency_ms}ms`)
    } else {
      toast.error(`连接失败：${result.message}`)
    }
    await loadProviders()
  } catch (err) {
    toast.error(err instanceof Error ? err.message : '连接测试失败')
  } finally {
    testingId.value = ''
  }
}

function resetTaskForm() {
  Object.assign(taskForm, {
    name: '',
    provider_id: providers.value.find((p) => p.enabled)?.id || providers.value[0]?.id || '',
    query: '田曦薇',
    domains_text: '',
    content_type_hint: '',
    source_type: 'media',
    trust_level: 'media',
    interval_minutes: 360,
    max_results: 20,
    enabled: true,
  })
}

function openTaskCreate() {
  taskEditingId.value = ''
  resetTaskForm()
  taskOpen.value = true
}

function openTaskEdit(task: SearchTask) {
  taskEditingId.value = task.id
  Object.assign(taskForm, {
    name: task.name,
    provider_id: task.provider_id,
    query: task.query,
    domains_text: (task.domains || []).join('\n'),
    content_type_hint: task.content_type_hint,
    source_type: task.source_type,
    trust_level: task.trust_level,
    interval_minutes: task.interval_minutes,
    max_results: task.max_results,
    enabled: task.enabled,
  })
  taskOpen.value = true
}

async function saveTask() {
  if (saving.value) return
  if (!taskForm.name.trim() || !taskForm.query.trim() || !taskForm.provider_id) {
    toast.error('任务名称、搜索服务与关键词不能为空')
    return
  }
  const domains = taskForm.domains_text
    .split(/[\n,，]+/)
    .map((d) => d.trim())
    .filter(Boolean)
  saving.value = true
  try {
    const payload = {
      name: taskForm.name.trim(),
      provider_id: taskForm.provider_id,
      query: taskForm.query.trim(),
      domains,
      content_type_hint: taskForm.content_type_hint,
      source_type: taskForm.source_type,
      trust_level: taskForm.trust_level,
      interval_minutes: taskForm.interval_minutes,
      max_results: taskForm.max_results,
      enabled: taskForm.enabled,
    }
    if (taskEditingId.value) {
      await api.patch(`/admin/search/tasks/${taskEditingId.value}`, payload)
      toast.success('搜索任务已更新')
    } else {
      await api.post('/admin/search/tasks', payload)
      toast.success('搜索任务已创建')
    }
    taskOpen.value = false
    await load()
  } catch (err) {
    toast.error(err instanceof Error ? err.message : '保存搜索任务失败')
  } finally {
    saving.value = false
  }
}

async function toggleTask(task: SearchTask) {
  try {
    await api.patch(`/admin/search/tasks/${task.id}`, { enabled: !task.enabled })
    toast.success(task.enabled ? '已停用搜索任务' : '已启用搜索任务')
    await load()
  } catch (err) {
    toast.error(err instanceof Error ? err.message : '操作失败')
  }
}

async function runTask(task: SearchTask) {
  if (runningId.value) return
  runningId.value = task.id
  try {
    await api.post(`/admin/search/tasks/${task.id}/run`, {})
    toast.success('已加入搜索队列，Worker 会尽快执行')
    await loadRuns()
  } catch (err) {
    toast.error(err instanceof Error ? err.message : '触发搜索失败')
  } finally {
    runningId.value = ''
  }
}

async function deleteTask(task: SearchTask) {
  if (!window.confirm(`删除搜索任务「${task.name}」？已采集的候选内容不会被删除。`)) return
  try {
    await api.del(`/admin/search/tasks/${task.id}`)
    toast.success('搜索任务已删除')
    await load()
  } catch (err) {
    toast.error(err instanceof Error ? err.message : '删除失败')
  }
}

async function loadProviders() {
  const data = await api.get<{ items: SearchProvider[] }>('/admin/search/providers')
  providers.value = data.items
}

async function loadRuns() {
  const data = await api.get<{ items: SearchRun[] }>('/admin/search/runs')
  runs.value = data.items
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const [providerData, taskData, runData] = await Promise.all([
      api.get<{ items: SearchProvider[] }>('/admin/search/providers'),
      api.get<{ items: SearchTask[] }>('/admin/search/tasks'),
      api.get<{ items: SearchRun[] }>('/admin/search/runs'),
    ])
    providers.value = providerData.items
    tasks.value = taskData.items
    runs.value = runData.items
  } catch (err) {
    error.value = err instanceof Error ? err.message : '搜索配置加载失败'
  } finally {
    loading.value = false
  }
}

function runTone(status: string): string {
  switch (status) {
    case 'success':
      return 'border-moss/30 bg-moss/10 text-moss'
    case 'failed':
      return 'border-accent/30 bg-blush/40 text-accent'
    case 'running':
      return 'border-[#b99a5e]/30 bg-[#f3ead7] text-[#8a6a32]'
    default:
      return 'border-line bg-white text-muted'
  }
}

const runStatusLabels: Record<string, string> = {
  queued: '排队中',
  running: '搜索中',
  success: '成功',
  failed: '失败',
}

onMounted(load)
</script>

<template>
  <div>
    <AdminPageHeader
      eyebrow="Console / Discovery"
      title="内容发现"
      description="配置搜索服务与搜索任务。搜索结果先进入候选池，经人工审核后才会公开；API Key 只通过环境变量引用，不保存、不返回前端。"
    >
      <template #actions>
        <button class="btn-ghost !min-h-9 !px-3 text-[11px]" :disabled="loading" @click="load">
          <RefreshCw class="h-3.5 w-3.5" :class="loading ? 'animate-spin' : ''" />
          刷新
        </button>
        <button class="btn-primary !min-h-9 !px-3 text-[11px]" @click="openTaskCreate">
          <Plus class="h-3.5 w-3.5" />
          新建搜索任务
        </button>
      </template>
    </AdminPageHeader>

    <div class="mb-6 flex items-start gap-3 border border-line bg-white/50 p-4">
      <Search class="mt-0.5 h-4 w-4 shrink-0 text-muted" />
      <p class="text-[10px] leading-5 text-muted">
        当前内置必应网页搜索 RSS 与可选 SearXNG。必应 RSS 结果受其服务条款约束，仅用于个人非商业内容聚合；社交平台收录能力有限，
        页面元数据解析可能被来源拦截，失败时仍保留搜索摘要与原始链接，不会伪装成原文全文。
      </p>
    </div>

    <LoadingBlock v-if="loading" :rows="6" />
    <EmptyState v-else-if="error" title="加载失败" :description="error" action-label="重新加载" @action="load" />

    <template v-else>
      <!-- Providers -->
      <section class="mb-10">
        <div class="mb-4 flex items-end justify-between gap-4">
          <div>
            <div class="eyebrow mb-1.5">Search Providers</div>
            <h2 class="font-display text-xl tracking-tight md:text-2xl">搜索服务</h2>
          </div>
          <button class="btn-ghost !min-h-8 !px-3 text-[10px]" @click="openProviderCreate">
            <Plus class="h-3 w-3" />添加服务
          </button>
        </div>
        <div v-if="providers.length" class="grid gap-3 lg:grid-cols-2">
          <article v-for="provider in providers" :key="provider.id" class="border border-line bg-white/40 p-4">
            <div class="flex items-start justify-between gap-3">
              <div class="min-w-0">
                <div class="flex flex-wrap items-center gap-2">
                  <h3 class="font-display text-[16px] tracking-tight">{{ provider.name }}</h3>
                  <span class="border border-line px-1.5 py-0.5 text-[9px] text-muted">
                    {{ provider.provider_type === 'bing_rss' ? 'Bing RSS' : 'SearXNG' }}
                  </span>
                  <span
                    class="border px-1.5 py-0.5 text-[9px]"
                    :class="provider.enabled ? 'border-moss/30 bg-moss/10 text-moss' : 'border-line bg-white text-muted'"
                  >
                    {{ provider.enabled ? '已启用' : '已停用' }}
                  </span>
                </div>
                <p class="mt-1.5 truncate text-[10px] text-muted" :title="provider.endpoint">{{ provider.endpoint || '使用默认地址' }}</p>
                <p class="mt-0.5 text-[10px] text-muted">
                  密钥环境变量：{{ provider.api_key_env || '无需密钥' }}
                </p>
                <p v-if="provider.last_test_at" class="mt-1 text-[9px]" :class="provider.last_test_ok ? 'text-moss' : 'text-[#9a6c72]'">
                  最近测试 {{ relativeTime(provider.last_test_at) }} ·
                  {{ provider.last_test_ok ? `成功 ${provider.last_latency_ms}ms` : provider.last_test_error || '失败' }}
                </p>
              </div>
              <Database class="h-4 w-4 shrink-0 text-muted" />
            </div>
            <div class="mt-3 flex flex-wrap items-center gap-2 border-t border-line pt-3">
              <button class="btn-ghost !min-h-7 !px-2.5 text-[10px]" :disabled="testingId === provider.id" @click="testProvider(provider)">
                <RefreshCw class="h-3 w-3" :class="testingId === provider.id ? 'animate-spin' : ''" />
                {{ testingId === provider.id ? '测试中' : '连接测试' }}
              </button>
              <button class="btn-ghost !min-h-7 !px-2.5 text-[10px]" @click="openProviderEdit(provider)">
                <Pencil class="h-3 w-3" />编辑
              </button>
              <button class="btn-ghost !min-h-7 !px-2.5 text-[10px]" @click="toggleProvider(provider)">
                {{ provider.enabled ? '停用' : '启用' }}
              </button>
            </div>
          </article>
        </div>
        <EmptyState v-else compact title="还没有搜索服务" description="添加必应 RSS 或自建 SearXNG 实例后即可创建搜索任务。" />
      </section>

      <!-- Tasks -->
      <section class="mb-10">
        <div class="mb-4 flex items-end justify-between gap-4">
          <div>
            <div class="eyebrow mb-1.5">Search Tasks</div>
            <h2 class="font-display text-xl tracking-tight md:text-2xl">搜索任务</h2>
          </div>
          <span class="text-[10px] text-muted">共 {{ tasks.length }} 个</span>
        </div>
        <div v-if="tasks.length" class="border-t border-line">
          <article v-for="task in tasks" :key="task.id" class="border-b border-line py-4">
            <div class="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
              <div class="min-w-0">
                <div class="flex flex-wrap items-center gap-2">
                  <h3 class="font-display text-[16px] tracking-tight">{{ task.name }}</h3>
                  <span
                    class="border px-1.5 py-0.5 text-[9px]"
                    :class="task.enabled ? 'border-moss/30 bg-moss/10 text-moss' : 'border-line bg-white text-muted'"
                  >
                    {{ task.enabled ? '运行中' : '已停用' }}
                  </span>
                  <span class="text-[9px] text-muted">{{ task.provider_name }}</span>
                </div>
                <p class="mt-1 text-[11px] text-ink/70">关键词：{{ task.query }}</p>
                <div class="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-[9px] text-muted">
                  <span v-if="task.domains?.length">站点：{{ task.domains.join(', ') }}</span>
                  <span v-else>站点：不限</span>
                  <span v-if="task.content_type_hint">分类：{{ contentTypeLabels[task.content_type_hint] || task.content_type_hint }}</span>
                  <span>间隔：{{ task.interval_minutes }} 分钟</span>
                  <span>单次上限：{{ task.max_results }} 条</span>
                  <span>来源：{{ sourceTypeLabels[task.source_type] || task.source_type }} / {{ trustLabels[task.trust_level] || task.trust_level }}</span>
                </div>
                <div class="mt-1.5 flex flex-wrap gap-x-3 gap-y-1 text-[9px] text-muted">
                  <span>上次运行：{{ task.last_run_at ? formatDateTime(task.last_run_at) : '未运行' }}</span>
                  <span>下次运行：{{ formatDateTime(task.next_run_at) }}</span>
                  <span v-if="task.last_success_at" class="text-moss">上次成功 {{ relativeTime(task.last_success_at) }}</span>
                </div>
                <p v-if="task.last_error" class="mt-1 line-clamp-2 text-[9px] text-[#9a6c72]">失败原因：{{ task.last_error }}</p>
              </div>
              <div class="flex shrink-0 flex-wrap items-center gap-1.5">
                <button class="btn-ghost !min-h-7 !px-2.5 text-[10px]" :disabled="runningId === task.id" @click="runTask(task)">
                  <Play class="h-3 w-3" />
                  {{ runningId === task.id ? '已加入' : '立即搜索' }}
                </button>
                <button class="btn-ghost !min-h-7 !px-2.5 text-[10px]" @click="openTaskEdit(task)">
                  <Pencil class="h-3 w-3" />编辑
                </button>
                <button class="btn-ghost !min-h-7 !px-2.5 text-[10px]" @click="toggleTask(task)">
                  {{ task.enabled ? '停用' : '启用' }}
                </button>
                <button class="btn-icon !h-7 !w-7 hover:!text-accent" title="删除任务" @click="deleteTask(task)">
                  <Trash2 class="h-3 w-3" />
                </button>
              </div>
            </div>
          </article>
        </div>
        <EmptyState v-else compact title="还没有搜索任务" description="创建任务后，Worker 会按间隔执行真实搜索，并把结果送入候选池。" />
      </section>

      <!-- Runs -->
      <section>
        <div class="mb-4 flex items-end justify-between gap-4">
          <div>
            <div class="eyebrow mb-1.5">Recent Runs</div>
            <h2 class="font-display text-xl tracking-tight md:text-2xl">最近搜索记录</h2>
          </div>
          <button class="btn-ghost !min-h-7 !px-2.5 text-[10px]" @click="loadRuns">
            <RefreshCw class="h-3 w-3" />刷新
          </button>
        </div>
        <div v-if="runs.length" class="overflow-x-auto border border-line">
          <table class="w-full min-w-[720px] text-left text-[10px]">
            <thead class="border-b border-line bg-white/50 text-[9px] uppercase tracking-widest text-muted">
              <tr>
                <th class="px-3 py-2 font-normal">任务</th>
                <th class="px-3 py-2 font-normal">状态</th>
                <th class="px-3 py-2 font-normal">触发</th>
                <th class="px-3 py-2 font-normal">结果</th>
                <th class="px-3 py-2 font-normal">元数据</th>
                <th class="px-3 py-2 font-normal">开始 / 结束</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-line">
              <tr v-for="run in runs" :key="run.id">
                <td class="px-3 py-2.5">
                  <div class="max-w-[180px] truncate">{{ run.task_name }}</div>
                  <div v-if="run.error_message" class="mt-0.5 max-w-[220px] truncate text-[9px] text-[#9a6c72]" :title="run.error_message">
                    {{ run.error_message }}
                  </div>
                </td>
                <td class="px-3 py-2.5">
                  <span class="inline-flex items-center gap-1 border px-1.5 py-0.5" :class="runTone(run.status)">
                    <CheckCircle2 v-if="run.status === 'success'" class="h-2.5 w-2.5" />
                    <XCircle v-else-if="run.status === 'failed'" class="h-2.5 w-2.5" />
                    {{ runStatusLabels[run.status] || run.status }}
                  </span>
                </td>
                <td class="px-3 py-2.5 text-muted">{{ run.trigger_type }}</td>
                <td class="px-3 py-2.5 text-muted">
                  抓取 {{ run.fetched_count }} / 新增 {{ run.inserted_count }} / 重复 {{ run.duplicate_count }} / 失败 {{ run.failed_count }}
                </td>
                <td class="px-3 py-2.5 text-muted">
                  成功 {{ run.parsed_ok_count }} / 失败 {{ run.parse_failed_count }}
                </td>
                <td class="px-3 py-2.5 text-muted">
                  <div>{{ run.started_at ? formatDateTime(run.started_at) : '—' }}</div>
                  <div class="text-[9px]">{{ run.finished_at ? formatDateTime(run.finished_at) : '—' }}</div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <EmptyState v-else compact title="暂无搜索记录" description="手动触发任务或等待 Worker 按计划执行。" />
      </section>
    </template>

    <!-- Provider modal -->
    <Modal
      :open="providerOpen"
      :title="providerEditingId ? '编辑搜索服务' : '添加搜索服务'"
      description="API Key 不在此填写。请把密钥写入服务端环境变量，然后在此填写变量名。"
      @close="providerOpen = false"
    >
      <div class="grid gap-4">
        <div>
          <label for="provider-name" class="field-label">名称 *</label>
          <input id="provider-name" v-model="providerForm.name" class="field" placeholder="例如：必应网页搜索 RSS" />
        </div>
        <div>
          <label for="provider-type" class="field-label">服务类型</label>
          <select id="provider-type" v-model="providerForm.provider_type" class="select-field" @change="onProviderTypeChange">
            <option value="bing_rss">必应网页搜索 RSS</option>
            <option value="searxng">SearXNG 实例</option>
          </select>
        </div>
        <div>
          <label for="provider-endpoint" class="field-label">服务地址</label>
          <input
            id="provider-endpoint"
            v-model="providerForm.endpoint"
            class="field"
            placeholder="https://www.bing.com/search 或自建 SearXNG 地址"
          />
          <p class="mt-1 text-[10px] text-muted">自建 SearXNG 可填写内网地址；抓取第三方页面时仍会执行 SSRF 防护。</p>
        </div>
        <div>
          <label for="provider-key" class="field-label">API Key 环境变量名（可选）</label>
          <input id="provider-key" v-model="providerForm.api_key_env" class="field" placeholder="例如：SEARXNG_API_KEY" />
          <p class="mt-1 text-[10px] text-muted">只保存变量名，不保存密钥本身；留空表示无需密钥。</p>
        </div>
        <label class="flex cursor-pointer items-center gap-2.5 text-xs">
          <input v-model="providerForm.enabled" type="checkbox" class="h-4 w-4 accent-[#c98791]" />
          启用该搜索服务
        </label>
      </div>
      <template #footer>
        <button class="btn-ghost !min-h-9 text-xs" @click="providerOpen = false">取消</button>
        <button class="btn-primary !min-h-9 text-xs" :disabled="saving" @click="saveProvider">
          {{ saving ? '保存中' : '保存' }}
        </button>
      </template>
    </Modal>

    <!-- Task modal -->
    <Modal
      :open="taskOpen"
      :title="taskEditingId ? '编辑搜索任务' : '新建搜索任务'"
      description="任务按间隔执行真实搜索，结果全部进入待审核候选池，不会自动公开。"
      size="lg"
      @close="taskOpen = false"
    >
      <div class="grid gap-4 md:grid-cols-2">
        <div class="md:col-span-2">
          <label for="task-name" class="field-label">任务名称 *</label>
          <input id="task-name" v-model="taskForm.name" class="field" placeholder="例如：田曦薇 · 综合资讯" />
        </div>
        <div>
          <label for="task-provider" class="field-label">搜索服务 *</label>
          <select id="task-provider" v-model="taskForm.provider_id" class="select-field">
            <option value="">请选择</option>
            <option v-for="provider in providers" :key="provider.id" :value="provider.id">
              {{ provider.name }}{{ provider.enabled ? '' : '（已停用）' }}
            </option>
          </select>
        </div>
        <div>
          <label for="task-query" class="field-label">关键词 *</label>
          <input id="task-query" v-model="taskForm.query" class="field" placeholder="田曦薇" />
        </div>
        <div class="md:col-span-2">
          <label for="task-domains" class="field-label">限定站点（可选，每行一个）</label>
          <textarea
            id="task-domains"
            v-model="taskForm.domains_text"
            rows="2"
            class="field resize-y"
            placeholder="例如：weibo.com&#10;bilibili.com"
          />
          <p class="mt-1 text-[10px] text-muted">最多 10 个。不要填写完整 URL 路径，只填域名。</p>
        </div>
        <div>
          <label for="task-hint" class="field-label">内容分类提示</label>
          <select id="task-hint" v-model="taskForm.content_type_hint" class="select-field">
            <option value="">自动判断</option>
            <option v-for="[key, label] in typeOptions" :key="key" :value="key">{{ label }}</option>
          </select>
        </div>
        <div>
          <label for="task-source-type" class="field-label">来源类型</label>
          <select id="task-source-type" v-model="taskForm.source_type" class="select-field">
            <option v-for="[key, label] in sourceTypeOptions" :key="key" :value="key">{{ label }}</option>
          </select>
        </div>
        <div>
          <label for="task-trust" class="field-label">可信等级</label>
          <select id="task-trust" v-model="taskForm.trust_level" class="select-field">
            <option v-for="[key, label] in trustOptions" :key="key" :value="key">{{ label }}</option>
          </select>
        </div>
        <div>
          <label for="task-interval" class="field-label">运行间隔（分钟）</label>
          <input id="task-interval" v-model.number="taskForm.interval_minutes" type="number" min="15" class="field" />
        </div>
        <div>
          <label for="task-max" class="field-label">单次最大结果数</label>
          <input id="task-max" v-model.number="taskForm.max_results" type="number" min="1" max="50" class="field" />
        </div>
        <div class="flex items-end pb-1">
          <label class="flex cursor-pointer items-center gap-2.5 text-xs">
            <input v-model="taskForm.enabled" type="checkbox" class="h-4 w-4 accent-[#c98791]" />
            启用该任务
          </label>
        </div>
      </div>
      <template #footer>
        <button class="btn-ghost !min-h-9 text-xs" @click="taskOpen = false">取消</button>
        <button class="btn-primary !min-h-9 text-xs" :disabled="saving" @click="saveTask">
          {{ saving ? '保存中' : '保存' }}
        </button>
      </template>
    </Modal>
  </div>
</template>
