<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import {
  Check,
  Pencil,
  Plus,
  RefreshCw,
  Save,
  Trash2,
  X,
} from 'lucide-vue-next'
import { api } from '../../api/client'
import type { Artist, ArtistEntity, Character, RecommendWeights, Rule, Scene, Work } from '../../types'
import { formatDate, workStatusLabels, workTypeLabels } from '../../lib/format'
import { toast } from '../../stores/toast'
import AdminPageHeader from './AdminPageHeader.vue'
import EmptyState from '../../components/EmptyState.vue'
import LoadingBlock from '../../components/LoadingBlock.vue'
import Modal from '../../components/Modal.vue'
import SceneIcon from '../../components/SceneIcon.vue'

type TabKey = 'artist' | 'entity' | 'works' | 'characters' | 'scenes' | 'rules' | 'weights'

const tab = ref<TabKey>('artist')
const loading = ref(true)
const error = ref('')
const saving = ref('')
const deleting = ref('')

const artist = ref<Artist | null>(null)
const entity = ref<ArtistEntity | null>(null)
const works = ref<Work[]>([])
const characters = ref<Character[]>([])
const scenes = ref<Scene[]>([])
const rules = ref<Rule[]>([])
const weights = ref<RecommendWeights>({ scene: 40, recency: 30, trust: 20, popularity: 10 })

const artistExtra = ref<Record<string, unknown>>({})
const artistForm = reactive({
  name: '',
  name_en: '',
  bio: '',
  avatar: '',
  cover: '',
  birth_date: '',
  birth_place: '',
  education: '',
  profession: '',
  debut: '',
  source_url: '',
})

const entityForm = reactive({
  name: '',
  aliases: '',
  verified_accounts: '',
  excluded_terms: '',
})

const workModal = ref(false)
const editingWorkId = ref('')
const workForm = reactive({
  title: '',
  type: 'drama',
  cover: '',
  description: '',
  release_date: '',
  status: 'aired',
  platform: '',
  official_url: '',
})

const characterModal = ref(false)
const editingCharacterId = ref('')
const characterForm = reactive({
  work_id: '',
  name: '',
  avatar: '',
  description: '',
})

const ruleModal = ref(false)
const editingRuleId = ref('')
const ruleForm = reactive({
  scene_id: '',
  keywords: '',
  priority: 10,
  enabled: true,
})

const sceneDrafts = ref<Record<string, { name: string; description: string; enabled: boolean }>>({})
const weightForm = reactive({ scene: 40, recency: 30, trust: 20, popularity: 10 })

const tabs: { key: TabKey; label: string; count?: number }[] = [
  { key: 'artist', label: '艺人' },
  { key: 'entity', label: '人物实体' },
  { key: 'works', label: '作品', count: works.value.length },
  { key: 'characters', label: '角色', count: characters.value.length },
  { key: 'scenes', label: '场景', count: scenes.value.length },
  { key: 'rules', label: '分类规则', count: rules.value.length },
  { key: 'weights', label: '推荐权重' },
]

const workTitleMap = computed(() => Object.fromEntries(works.value.map((w) => [w.id, w.title])))
const weightTotal = computed(() => weightForm.scene + weightForm.recency + weightForm.trust + weightForm.popularity)
const weightValid = computed(
  () => weightTotal.value === 100 && [weightForm.scene, weightForm.recency, weightForm.trust, weightForm.popularity].every((n) => n >= 0),
)

function profileString(key: string): string {
  const value = artist.value?.profile_data?.[key]
  return typeof value === 'string' ? value : ''
}

function applyArtist(data: Artist) {
  artist.value = data
  artistExtra.value = { ...(data.profile_data || {}) }
  Object.assign(artistForm, {
    name: data.name || '',
    name_en: data.name_en || '',
    bio: data.bio || '',
    avatar: data.avatar || '',
    cover: data.cover || '',
    birth_date: profileString('birth_date'),
    birth_place: profileString('birth_place'),
    education: profileString('education'),
    profession: profileString('profession'),
    debut: profileString('debut'),
    source_url: profileString('source_url'),
  })
}

function splitList(value: string): string[] {
  return value
    .split(/[,，、;\n；]/)
    .map((item) => item.trim())
    .filter(Boolean)
}

function applyEntity(data: ArtistEntity) {
  entity.value = data
  Object.assign(entityForm, {
    name: data.name || '',
    aliases: (data.aliases || []).join('，'),
    verified_accounts: (data.verified_accounts || []).join('，'),
    excluded_terms: (data.excluded_terms || []).join('，'),
  })
}

function applyScenes(list: Scene[]) {
  scenes.value = list
  const drafts: Record<string, { name: string; description: string; enabled: boolean }> = {}
  for (const scene of list) {
    drafts[scene.id] = { name: scene.name, description: scene.description, enabled: scene.enabled }
  }
  sceneDrafts.value = drafts
}

function applyWeights(data: RecommendWeights) {
  weights.value = data
  Object.assign(weightForm, data)
}

async function loadAll() {
  loading.value = true
  error.value = ''
  try {
    const [artistRes, entityRes, workRes, charRes, sceneRes, ruleRes, weightRes] = await Promise.all([
      api.get<Artist>('/admin/artist'),
      api.get<ArtistEntity>('/admin/artist-entity'),
      api.get<{ items: Work[] }>('/admin/works'),
      api.get<{ items: Character[] }>('/admin/characters'),
      api.get<{ items: Scene[] }>('/admin/scenes'),
      api.get<{ items: Rule[] }>('/admin/rules'),
      api.get<RecommendWeights>('/admin/settings/recommend'),
    ])
    applyArtist(artistRes)
    applyEntity(entityRes)
    works.value = workRes.items
    characters.value = charRes.items
    applyScenes(sceneRes.items)
    rules.value = ruleRes.items
    applyWeights(weightRes)
  } catch (err) {
    error.value = err instanceof Error ? err.message : '基础资料加载失败'
  } finally {
    loading.value = false
  }
}

async function reloadWorks() {
  const data = await api.get<{ items: Work[] }>('/admin/works')
  works.value = data.items
}

async function reloadCharacters() {
  const data = await api.get<{ items: Character[] }>('/admin/characters')
  characters.value = data.items
}

async function reloadRules() {
  const data = await api.get<{ items: Rule[] }>('/admin/rules')
  rules.value = data.items
}

async function reloadScenes() {
  const data = await api.get<{ items: Scene[] }>('/admin/scenes')
  applyScenes(data.items)
}

async function saveArtist() {
  if (saving.value) return
  if (!artistForm.name.trim()) {
    toast.error('艺人姓名不能为空')
    return
  }
  saving.value = 'artist'
  try {
    const profile: Record<string, unknown> = { ...artistExtra.value }
    const pairs: [string, string][] = [
      ['birth_date', artistForm.birth_date],
      ['birth_place', artistForm.birth_place],
      ['education', artistForm.education],
      ['profession', artistForm.profession],
      ['debut', artistForm.debut],
      ['source_url', artistForm.source_url],
    ]
    for (const [key, value] of pairs) {
      if (value.trim()) profile[key] = value.trim()
      else delete profile[key]
    }
    const updated = await api.put<Artist>('/admin/artist', {
      name: artistForm.name.trim(),
      name_en: artistForm.name_en.trim(),
      bio: artistForm.bio,
      avatar: artistForm.avatar,
      cover: artistForm.cover,
      profile_data: profile,
    })
    applyArtist(updated)
    toast.success('艺人资料已保存')
  } catch (err) {
    toast.error(err instanceof Error ? err.message : '保存失败')
  } finally {
    saving.value = ''
  }
}

async function saveEntity() {
  if (saving.value) return
  if (!entityForm.name.trim()) {
    toast.error('人物姓名不能为空')
    return
  }
  saving.value = 'entity'
  try {
    const updated = await api.put<ArtistEntity>('/admin/artist-entity', {
      name: entityForm.name.trim(),
      aliases: splitList(entityForm.aliases),
      verified_accounts: splitList(entityForm.verified_accounts),
      excluded_terms: splitList(entityForm.excluded_terms),
    })
    applyEntity(updated)
    toast.success('人物实体已保存，新的搜索任务将按此配置评分')
  } catch (err) {
    toast.error(err instanceof Error ? err.message : '保存失败')
  } finally {
    saving.value = ''
  }
}

function openWorkCreate() {
  editingWorkId.value = ''
  Object.assign(workForm, {
    title: '',
    type: 'drama',
    cover: '',
    description: '',
    release_date: '',
    status: 'aired',
    platform: '',
    official_url: '',
  })
  workModal.value = true
}

function openWorkEdit(work: Work) {
  editingWorkId.value = work.id
  Object.assign(workForm, {
    title: work.title,
    type: work.type,
    cover: work.cover,
    description: work.description,
    release_date: work.release_date || '',
    status: work.status,
    platform: work.platform,
    official_url: work.official_url,
  })
  workModal.value = true
}

async function saveWork() {
  if (saving.value) return
  if (!workForm.title.trim()) {
    toast.error('作品名称不能为空')
    return
  }
  saving.value = 'work'
  try {
    const body = {
      title: workForm.title.trim(),
      type: workForm.type,
      cover: workForm.cover,
      description: workForm.description,
      release_date: workForm.release_date,
      status: workForm.status,
      platform: workForm.platform,
      official_url: workForm.official_url,
    }
    if (editingWorkId.value) {
      await api.put(`/admin/works/${editingWorkId.value}`, body)
      toast.success('作品已更新')
    } else {
      await api.post('/admin/works', body)
      toast.success('作品已创建')
    }
    workModal.value = false
    await reloadWorks()
  } catch (err) {
    toast.error(err instanceof Error ? err.message : '保存失败')
  } finally {
    saving.value = ''
  }
}

async function deleteWork(work: Work) {
  if (deleting.value) return
  if (!window.confirm(`确定删除作品《${work.title}》？关联的角色也会受影响。`)) return
  deleting.value = 'work:' + work.id
  try {
    await api.del(`/admin/works/${work.id}`)
    toast.success('作品已删除')
    await Promise.all([reloadWorks(), reloadCharacters()])
  } catch (err) {
    toast.error(err instanceof Error ? err.message : '删除失败，可能仍有关联内容')
  } finally {
    deleting.value = ''
  }
}

function openCharacterCreate() {
  editingCharacterId.value = ''
  Object.assign(characterForm, { work_id: works.value[0]?.id || '', name: '', avatar: '', description: '' })
  characterModal.value = true
}

function openCharacterEdit(character: Character) {
  editingCharacterId.value = character.id
  Object.assign(characterForm, {
    work_id: character.work_id,
    name: character.name,
    avatar: character.avatar,
    description: character.description,
  })
  characterModal.value = true
}

async function saveCharacter() {
  if (saving.value) return
  if (!characterForm.work_id || !characterForm.name.trim()) {
    toast.error('请选择所属作品并填写角色名')
    return
  }
  saving.value = 'character'
  try {
    const body = {
      work_id: characterForm.work_id,
      name: characterForm.name.trim(),
      avatar: characterForm.avatar,
      description: characterForm.description,
    }
    if (editingCharacterId.value) {
      await api.put(`/admin/characters/${editingCharacterId.value}`, body)
      toast.success('角色已更新')
    } else {
      await api.post('/admin/characters', body)
      toast.success('角色已创建')
    }
    characterModal.value = false
    await Promise.all([reloadCharacters(), reloadWorks()])
  } catch (err) {
    toast.error(err instanceof Error ? err.message : '保存失败')
  } finally {
    saving.value = ''
  }
}

async function deleteCharacter(character: Character) {
  if (deleting.value) return
  if (!window.confirm(`确定删除角色「${character.name}」？`)) return
  deleting.value = 'char:' + character.id
  try {
    await api.del(`/admin/characters/${character.id}`)
    toast.success('角色已删除')
    await Promise.all([reloadCharacters(), reloadWorks()])
  } catch (err) {
    toast.error(err instanceof Error ? err.message : '删除失败')
  } finally {
    deleting.value = ''
  }
}

async function saveScene(sceneId: string) {
  const draft = sceneDrafts.value[sceneId]
  if (!draft || saving.value) return
  saving.value = 'scene:' + sceneId
  try {
    await api.patch(`/admin/scenes/${sceneId}`, {
      name: draft.name,
      description: draft.description,
      enabled: draft.enabled,
    })
    toast.success('场景已更新')
    await reloadScenes()
  } catch (err) {
    toast.error(err instanceof Error ? err.message : '保存失败')
    await reloadScenes()
  } finally {
    saving.value = ''
  }
}

function openRuleCreate() {
  editingRuleId.value = ''
  Object.assign(ruleForm, { scene_id: scenes.value.find((s) => s.code !== 'all')?.id || '', keywords: '', priority: 10, enabled: true })
  ruleModal.value = true
}

function openRuleEdit(rule: Rule) {
  editingRuleId.value = rule.id
  Object.assign(ruleForm, {
    scene_id: rule.scene_id,
    keywords: rule.keywords,
    priority: rule.priority,
    enabled: rule.enabled,
  })
  ruleModal.value = true
}

async function saveRule() {
  if (saving.value) return
  if (!ruleForm.scene_id || !ruleForm.keywords.trim()) {
    toast.error('请选择场景并填写关键词')
    return
  }
  saving.value = 'rule'
  try {
    const body = {
      scene_id: ruleForm.scene_id,
      keywords: ruleForm.keywords.trim(),
      priority: Number(ruleForm.priority) || 0,
      enabled: ruleForm.enabled,
    }
    if (editingRuleId.value) {
      await api.put(`/admin/rules/${editingRuleId.value}`, body)
      toast.success('分类规则已更新')
    } else {
      await api.post('/admin/rules', body)
      toast.success('分类规则已创建')
    }
    ruleModal.value = false
    await reloadRules()
  } catch (err) {
    toast.error(err instanceof Error ? err.message : '保存失败')
  } finally {
    saving.value = ''
  }
}

async function toggleRule(rule: Rule) {
  if (saving.value) return
  saving.value = 'rule:' + rule.id
  try {
    await api.put(`/admin/rules/${rule.id}`, {
      scene_id: rule.scene_id,
      keywords: rule.keywords,
      priority: rule.priority,
      enabled: !rule.enabled,
    })
    await reloadRules()
  } catch (err) {
    toast.error(err instanceof Error ? err.message : '操作失败')
  } finally {
    saving.value = ''
  }
}

async function deleteRule(rule: Rule) {
  if (deleting.value) return
  if (!window.confirm(`确定删除「${rule.scene_name}」的关键词规则？`)) return
  deleting.value = 'rule:' + rule.id
  try {
    await api.del(`/admin/rules/${rule.id}`)
    toast.success('规则已删除')
    await reloadRules()
  } catch (err) {
    toast.error(err instanceof Error ? err.message : '删除失败')
  } finally {
    deleting.value = ''
  }
}

async function saveWeights() {
  if (!weightValid.value || saving.value) return
  saving.value = 'weights'
  try {
    const updated = await api.put<RecommendWeights>('/admin/settings/recommend', {
      scene: Number(weightForm.scene),
      recency: Number(weightForm.recency),
      trust: Number(weightForm.trust),
      popularity: Number(weightForm.popularity),
    })
    applyWeights(updated)
    toast.success('推荐权重已保存')
  } catch (err) {
    toast.error(err instanceof Error ? err.message : '保存失败')
  } finally {
    saving.value = ''
  }
}

onMounted(loadAll)
</script>

<template>
  <div>
    <AdminPageHeader
      eyebrow="Console / Base Data"
      title="基础资料"
      description="维护艺人、作品、角色、兴趣场景与分类规则。涉及公开事实的修改应保留可核验来源。"
    >
      <template #actions>
        <button class="btn-ghost !min-h-9 !px-3 text-[11px]" :disabled="loading" @click="loadAll">
          <RefreshCw class="h-3.5 w-3.5" :class="loading ? 'animate-spin' : ''" />
          刷新
        </button>
      </template>
    </AdminPageHeader>

    <div class="no-scrollbar mb-6 flex gap-1 overflow-x-auto border-b border-line pb-0">
      <button
        v-for="item in tabs"
        :key="item.key"
        class="relative shrink-0 px-4 py-2.5 text-[11px] transition"
        :class="tab === item.key ? 'text-ink' : 'text-muted hover:text-ink'"
        @click="tab = item.key"
      >
        {{ item.label }}
        <span v-if="item.count !== undefined" class="ml-1 text-[9px] opacity-50">{{ item.count }}</span>
        <span v-if="tab === item.key" class="absolute inset-x-2 bottom-0 h-[2px] bg-accent" />
      </button>
    </div>

    <LoadingBlock v-if="loading" type="text" :rows="8" />
    <EmptyState v-else-if="error" title="基础资料加载失败" :description="error" action-label="重新加载" @action="loadAll" />

    <template v-else>
      <!-- Artist -->
      <section v-if="tab === 'artist'" class="grid gap-6 lg:grid-cols-[.7fr_1.3fr]">
        <div class="border border-line bg-canvas p-5 h-fit">
          <div class="flex h-20 w-20 items-center justify-center rounded-full bg-blush font-display text-4xl text-[#8f5e66]">
            {{ artist?.name?.slice(0, 1) || '田' }}
          </div>
          <div class="mt-4 font-display text-2xl">{{ artist?.name || '未命名' }}</div>
          <div class="mt-1 text-[10px] uppercase tracking-widest2 text-muted">{{ artist?.name_en }}</div>
          <p class="mt-4 text-[11px] leading-6 text-muted">{{ artist?.bio }}</p>
          <div class="mt-4 border-t border-line pt-3 text-[10px] leading-5 text-muted">
            档案中的公开事实应来自可核验来源；修改后请在来源字段保留引用地址。
          </div>
        </div>

        <div class="border border-line bg-canvas">
          <div class="border-b border-line px-5 py-4">
            <div class="eyebrow">Profile</div>
            <h2 class="mt-1 font-display text-xl">公开档案</h2>
          </div>
          <div class="grid gap-5 p-5 sm:grid-cols-2 md:p-6">
            <div>
              <label for="artist-name" class="field-label">姓名 *</label>
              <input id="artist-name" v-model="artistForm.name" class="field" />
            </div>
            <div>
              <label for="artist-en" class="field-label">英文名</label>
              <input id="artist-en" v-model="artistForm.name_en" class="field" />
            </div>
            <div>
              <label for="artist-birth" class="field-label">出生日期</label>
              <input id="artist-birth" v-model="artistForm.birth_date" class="field" placeholder="YYYY-MM-DD" />
            </div>
            <div>
              <label for="artist-place" class="field-label">出生地</label>
              <input id="artist-place" v-model="artistForm.birth_place" class="field" />
            </div>
            <div>
              <label for="artist-edu" class="field-label">毕业院校</label>
              <input id="artist-edu" v-model="artistForm.education" class="field" />
            </div>
            <div>
              <label for="artist-job" class="field-label">职业</label>
              <input id="artist-job" v-model="artistForm.profession" class="field" />
            </div>
            <div>
              <label for="artist-debut" class="field-label">出道年份</label>
              <input id="artist-debut" v-model="artistForm.debut" class="field" />
            </div>
            <div>
              <label for="artist-source" class="field-label">来源链接</label>
              <input id="artist-source" v-model="artistForm.source_url" class="field" placeholder="https://..." />
            </div>
            <div class="sm:col-span-2">
              <label for="artist-bio" class="field-label">简介</label>
              <textarea id="artist-bio" v-model="artistForm.bio" rows="4" class="field resize-y" />
            </div>
            <div>
              <label for="artist-avatar" class="field-label">头像地址（需授权）</label>
              <input id="artist-avatar" v-model="artistForm.avatar" class="field" placeholder="留空使用站内视觉" />
            </div>
            <div>
              <label for="artist-cover" class="field-label">封面地址（需授权）</label>
              <input id="artist-cover" v-model="artistForm.cover" class="field" placeholder="留空使用站内视觉" />
            </div>
          </div>
          <div class="flex justify-end border-t border-line px-5 py-4">
            <button class="btn-primary !min-h-9 text-xs" :disabled="saving === 'artist'" @click="saveArtist">
              <Save class="h-3 w-3" />
              {{ saving === 'artist' ? '保存中' : '保存艺人资料' }}
            </button>
          </div>
        </div>
      </section>

      <!-- Person entity -->
      <section v-if="tab === 'entity'" class="grid gap-6 lg:grid-cols-[.8fr_1.2fr]">
        <div class="h-fit border border-line bg-canvas p-5">
          <div class="eyebrow">Entity Relevance</div>
          <h2 class="mt-1 font-display text-xl">人物相关性评分</h2>
          <p class="mt-3 text-[11px] leading-6 text-muted">
            采集结果入库前按下面规则打分。别名与认证账号必须人工核验后填写，系统不会自动推断。
          </p>
          <div class="mt-4 space-y-1.5 text-[10px]">
            <div class="flex justify-between border-b border-line py-1.5"><span class="text-muted">标题包含完整姓名 / 别名</span><span>+40</span></div>
            <div class="flex justify-between border-b border-line py-1.5"><span class="text-muted">摘要包含完整姓名 / 别名</span><span>+25</span></div>
            <div class="flex justify-between border-b border-line py-1.5"><span class="text-muted">来源为已验证账号</span><span>+25</span></div>
            <div class="flex justify-between py-1.5"><span class="text-muted">提及关联作品 / 角色 / 事件</span><span>+15 / 项</span></div>
          </div>
          <div class="mt-4 space-y-1.5 text-[10px] leading-5 text-muted">
            <p>≥ 70 分：高度相关，进入候选池；40–69 分：相关候选；低于 40 分：自动过滤并记录原因。</p>
            <p>单字姓氏、姓氏/字典百科、缺少人物上下文的页面会被硬性排除，不进入待审核队列。</p>
          </div>
        </div>

        <div class="border border-line bg-canvas">
          <div class="border-b border-line px-5 py-4">
            <div class="eyebrow">Verified Entity</div>
            <h2 class="mt-1 font-display text-xl">人物实体配置</h2>
          </div>
          <div class="grid gap-5 p-5 md:p-6">
            <div>
              <label for="entity-name" class="field-label">完整姓名 *</label>
              <input id="entity-name" v-model="entityForm.name" class="field" placeholder="例如：田曦薇" />
              <p class="mt-1 text-[10px] text-muted">搜索任务必须包含完整姓名或已验证别名，单独使用姓氏会被拒绝。</p>
            </div>
            <div>
              <label for="entity-aliases" class="field-label">已验证别名（逗号分隔）</label>
              <input id="entity-aliases" v-model="entityForm.aliases" class="field" placeholder="留空表示没有已核验别名" />
            </div>
            <div>
              <label for="entity-accounts" class="field-label">已验证账号（逗号分隔）</label>
              <input id="entity-accounts" v-model="entityForm.verified_accounts" class="field" placeholder="例如：田曦薇工作室" />
              <p class="mt-1 text-[10px] text-muted">仅填写已人工核验的官方或本人账号，命中后加 25 分。</p>
            </div>
            <div>
              <label for="entity-excluded" class="field-label">排除词（逗号或换行分隔）</label>
              <textarea
                id="entity-excluded"
                v-model="entityForm.excluded_terms"
                rows="4"
                class="field resize-y"
                placeholder="田姓氏，田字解释，汉字田，田字组词"
              />
              <p class="mt-1 text-[10px] leading-5 text-muted">
                命中排除词的页面会标记为「姓氏/字典内容」并自动过滤。仅在完整姓名未出现时生效。
              </p>
            </div>
          </div>
          <div class="flex justify-end border-t border-line px-5 py-4">
            <button class="btn-primary !min-h-9 text-xs" :disabled="saving === 'entity'" @click="saveEntity">
              <Save class="h-3 w-3" />
              {{ saving === 'entity' ? '保存中' : '保存人物实体' }}
            </button>
          </div>
        </div>
      </section>

      <!-- Works -->
      <section v-if="tab === 'works'" class="border border-line bg-canvas">
        <div class="flex items-center justify-between border-b border-line px-5 py-4">
          <div>
            <div class="eyebrow">Works</div>
            <h2 class="mt-1 font-display text-xl">作品档案</h2>
          </div>
          <button class="btn-primary !min-h-8 !px-3 text-[11px]" @click="openWorkCreate">
            <Plus class="h-3 w-3" />新增作品
          </button>
        </div>
        <div v-if="!works.length" class="px-5 py-12 text-center text-[11px] text-muted">暂无作品</div>
        <div v-else class="divide-y divide-line">
          <div v-for="work in works" :key="work.id" class="flex flex-wrap items-center gap-x-4 gap-y-2 px-5 py-3.5">
            <div class="min-w-0 flex-1">
              <div class="flex flex-wrap items-center gap-2">
                <span class="font-display text-[16px]">{{ work.title }}</span>
                <span class="tag !py-0.5">{{ workTypeLabels[work.type] || work.type }}</span>
                <span class="text-[9px] text-muted">{{ workStatusLabels[work.status] || work.status }}</span>
              </div>
              <div class="mt-1 flex flex-wrap gap-x-3 text-[10px] text-muted">
                <span>{{ formatDate(work.release_date) }}</span>
                <span v-if="work.platform">{{ work.platform }}</span>
                <span v-if="work.characters?.length">角色 {{ work.characters.map((c) => c.name).join(' / ') }}</span>
              </div>
            </div>
            <div class="flex items-center gap-1">
              <button class="btn-icon !h-7 !w-7" title="编辑" @click="openWorkEdit(work)">
                <Pencil class="h-3 w-3" />
              </button>
              <button
                class="btn-icon !h-7 !w-7 text-muted hover:text-accent"
                title="删除"
                :disabled="!!deleting"
                @click="deleteWork(work)"
              >
                <Trash2 class="h-3 w-3" />
              </button>
            </div>
          </div>
        </div>
      </section>

      <!-- Characters -->
      <section v-if="tab === 'characters'" class="border border-line bg-canvas">
        <div class="flex items-center justify-between border-b border-line px-5 py-4">
          <div>
            <div class="eyebrow">Characters</div>
            <h2 class="mt-1 font-display text-xl">角色档案</h2>
          </div>
          <button class="btn-primary !min-h-8 !px-3 text-[11px]" :disabled="!works.length" @click="openCharacterCreate">
            <Plus class="h-3 w-3" />新增角色
          </button>
        </div>
        <div v-if="!characters.length" class="px-5 py-12 text-center text-[11px] text-muted">暂无角色</div>
        <div v-else class="divide-y divide-line">
          <div v-for="character in characters" :key="character.id" class="flex flex-wrap items-center gap-x-4 gap-y-2 px-5 py-3.5">
            <span class="flex h-8 w-8 shrink-0 items-center justify-center border border-line bg-blush/30 font-display text-sm text-[#8f5e66]">
              {{ character.name.slice(0, 1) }}
            </span>
            <div class="min-w-0 flex-1">
              <div class="flex flex-wrap items-center gap-2">
                <span class="text-[13px]">{{ character.name }}</span>
                <span class="text-[10px] text-muted">《{{ workTitleMap[character.work_id] || '未知作品' }}》</span>
              </div>
              <p class="mt-0.5 line-clamp-1 text-[10px] text-muted">{{ character.description || '暂无简介' }}</p>
            </div>
            <div class="flex items-center gap-1">
              <button class="btn-icon !h-7 !w-7" title="编辑" @click="openCharacterEdit(character)">
                <Pencil class="h-3 w-3" />
              </button>
              <button
                class="btn-icon !h-7 !w-7 text-muted hover:text-accent"
                title="删除"
                :disabled="!!deleting"
                @click="deleteCharacter(character)"
              >
                <Trash2 class="h-3 w-3" />
              </button>
            </div>
          </div>
        </div>
      </section>

      <!-- Scenes -->
      <section v-if="tab === 'scenes'" class="border border-line bg-canvas">
        <div class="border-b border-line px-5 py-4">
          <div class="eyebrow">Scene Taxonomy</div>
          <h2 class="mt-1 font-display text-xl">兴趣场景</h2>
          <p class="mt-1 text-[10px] text-muted">停用后不再出现在前台筛选与兴趣设置中。场景编码不可修改。</p>
        </div>
        <div class="divide-y divide-line">
          <div v-for="scene in scenes" :key="scene.id" class="grid gap-3 px-5 py-4 md:grid-cols-[130px_1fr_auto] md:items-center">
            <div class="flex items-center gap-2.5">
              <span class="flex h-8 w-8 items-center justify-center border border-line bg-white/60 text-muted">
                <SceneIcon :code="scene.code" class="h-3.5 w-3.5" />
              </span>
              <div>
                <div class="text-[12px]">{{ sceneDrafts[scene.id]?.name || scene.name }}</div>
                <div class="text-[9px] uppercase tracking-widest text-muted">{{ scene.code }}</div>
              </div>
            </div>
            <div class="grid gap-2 sm:grid-cols-2">
              <input v-model="sceneDrafts[scene.id].name" class="field !py-1.5 text-xs" aria-label="场景名称" />
              <input v-model="sceneDrafts[scene.id].description" class="field !py-1.5 text-xs" aria-label="场景描述" />
            </div>
            <div class="flex items-center justify-end gap-2">
              <label class="flex cursor-pointer items-center gap-1.5 text-[10px] text-muted">
                <input v-model="sceneDrafts[scene.id].enabled" type="checkbox" class="h-3.5 w-3.5 accent-[#c98791]" />
                启用
              </label>
              <button
                class="btn-ghost !min-h-7 !px-2 text-[10px]"
                :disabled="saving === 'scene:' + scene.id"
                @click="saveScene(scene.id)"
              >
                保存
              </button>
            </div>
          </div>
        </div>
      </section>

      <!-- Rules -->
      <section v-if="tab === 'rules'" class="border border-line bg-canvas">
        <div class="flex items-center justify-between border-b border-line px-5 py-4">
          <div>
            <div class="eyebrow">Classification</div>
            <h2 class="mt-1 font-display text-xl">关键词分类规则</h2>
            <p class="mt-1 text-[10px] text-muted">基于关键词的确定性分类，不依赖大模型。优先级高的规则先命中。</p>
          </div>
          <button class="btn-primary !min-h-8 !px-3 text-[11px]" @click="openRuleCreate">
            <Plus class="h-3 w-3" />新增规则
          </button>
        </div>
        <div v-if="!rules.length" class="px-5 py-12 text-center text-[11px] text-muted">暂无分类规则</div>
        <div v-else class="divide-y divide-line">
          <div v-for="rule in rules" :key="rule.id" class="flex flex-wrap items-center gap-x-4 gap-y-2 px-5 py-3.5">
            <span class="w-16 shrink-0 text-[10px] uppercase tracking-widest text-muted">P{{ rule.priority }}</span>
            <span class="tag !py-0.5">{{ rule.scene_name }}</span>
            <span class="min-w-0 flex-1 truncate text-[11px] text-ink/70">{{ rule.keywords }}</span>
            <button
              class="relative h-4 w-8 shrink-0 rounded-full transition"
              :class="rule.enabled ? 'bg-accent' : 'bg-line'"
              :aria-pressed="rule.enabled"
              :disabled="!!saving"
              @click="toggleRule(rule)"
            >
              <span class="absolute top-0.5 h-3 w-3 rounded-full bg-white shadow transition" :class="rule.enabled ? 'left-[17px]' : 'left-0.5'" />
            </button>
            <div class="flex items-center gap-1">
              <button class="btn-icon !h-7 !w-7" title="编辑" @click="openRuleEdit(rule)"><Pencil class="h-3 w-3" /></button>
              <button class="btn-icon !h-7 !w-7 text-muted hover:text-accent" title="删除" @click="deleteRule(rule)">
                <Trash2 class="h-3 w-3" />
              </button>
            </div>
          </div>
        </div>
      </section>

      <!-- Weights -->
      <section v-if="tab === 'weights'" class="grid gap-6 lg:grid-cols-[1.2fr_.8fr]">
        <div class="border border-line bg-canvas">
          <div class="border-b border-line px-5 py-4">
            <div class="eyebrow">Recommendation</div>
            <h2 class="mt-1 font-display text-xl">推荐权重</h2>
            <p class="mt-1 text-[10px] text-muted">四项之和必须为 100。权重调整后立即影响「为你推荐」排序。</p>
          </div>
          <div class="grid gap-5 p-5 sm:grid-cols-2 md:p-6">
            <div v-for="key in (['scene', 'recency', 'trust', 'popularity'] as const)" :key="key">
              <label :for="`weight-${key}`" class="field-label">
                {{ { scene: '场景匹配', recency: '内容新鲜度', trust: '来源可信度', popularity: '热度（预留）' }[key] }}
              </label>
              <div class="flex items-center gap-2">
                <input
                  :id="`weight-${key}`"
                  v-model.number="weightForm[key]"
                  type="number"
                  min="0"
                  max="100"
                  class="field !py-2.5"
                />
                <span class="text-xs text-muted">%</span>
              </div>
            </div>
          </div>
          <div class="flex flex-wrap items-center justify-between gap-3 border-t border-line px-5 py-4">
            <p class="text-[11px]" :class="weightValid ? 'text-moss' : 'text-accent'">
              当前合计 {{ weightTotal }}%{{ weightValid ? '' : ' · 需等于 100%' }}
            </p>
            <button class="btn-primary !min-h-9 text-xs" :disabled="!weightValid || saving === 'weights'" @click="saveWeights">
              <Check class="h-3 w-3" />
              {{ saving === 'weights' ? '保存中' : '保存权重' }}
            </button>
          </div>
        </div>

        <div class="h-fit border border-line bg-canvas p-5">
          <div class="eyebrow">How It Works</div>
          <p class="mt-3 text-[11px] leading-6 text-muted">
            推荐分数由场景匹配、时间衰减、来源可信度与热度共同组成。目前没有可验证的热度信号，热度项保持预留，不参与实际计算。
          </p>
          <div class="mt-4 space-y-2 text-[10px]">
            <div class="flex justify-between border-b border-line py-1.5"><span class="text-muted">场景</span><span>{{ weightForm.scene }}%</span></div>
            <div class="flex justify-between border-b border-line py-1.5"><span class="text-muted">新鲜度</span><span>{{ weightForm.recency }}%</span></div>
            <div class="flex justify-between border-b border-line py-1.5"><span class="text-muted">可信度</span><span>{{ weightForm.trust }}%</span></div>
            <div class="flex justify-between py-1.5"><span class="text-muted">热度</span><span>{{ weightForm.popularity }}%</span></div>
          </div>
          <p class="mt-4 text-[10px] leading-5 text-muted">
            官方来源可信度最高；社区来源较低，重要信息进入人工审核。
          </p>
        </div>
      </section>
    </template>

    <!-- Work modal -->
    <Modal :open="workModal" :title="editingWorkId ? '编辑作品' : '新增作品'" size="lg" @close="workModal = false">
      <div class="grid gap-5 sm:grid-cols-2">
        <div class="sm:col-span-2">
          <label for="work-title" class="field-label">作品名称 *</label>
          <input id="work-title" v-model="workForm.title" class="field" />
        </div>
        <div>
          <label for="work-type" class="field-label">类型</label>
          <select id="work-type" v-model="workForm.type" class="select-field">
            <option v-for="(label, key) in workTypeLabels" :key="key" :value="key">{{ label }}</option>
          </select>
        </div>
        <div>
          <label for="work-status" class="field-label">状态</label>
          <select id="work-status" v-model="workForm.status" class="select-field">
            <option v-for="(label, key) in workStatusLabels" :key="key" :value="key">{{ label }}</option>
          </select>
        </div>
        <div>
          <label for="work-date" class="field-label">首播 / 上线日期</label>
          <input id="work-date" v-model="workForm.release_date" type="date" class="field" />
        </div>
        <div>
          <label for="work-platform" class="field-label">播出平台</label>
          <input id="work-platform" v-model="workForm.platform" class="field" />
        </div>
        <div class="sm:col-span-2">
          <label for="work-url" class="field-label">官方 / 公开资料链接</label>
          <input id="work-url" v-model="workForm.official_url" class="field" placeholder="https://..." />
        </div>
        <div class="sm:col-span-2">
          <label for="work-cover" class="field-label">封面地址（需授权）</label>
          <input id="work-cover" v-model="workForm.cover" class="field" />
        </div>
        <div class="sm:col-span-2">
          <label for="work-desc" class="field-label">简介</label>
          <textarea id="work-desc" v-model="workForm.description" rows="3" class="field resize-y" />
        </div>
      </div>
      <template #footer>
        <button class="btn-ghost !min-h-9 text-xs" @click="workModal = false">取消</button>
        <button class="btn-primary !min-h-9 text-xs" :disabled="saving === 'work'" @click="saveWork">
          {{ saving === 'work' ? '保存中' : '保存' }}
        </button>
      </template>
    </Modal>

    <!-- Character modal -->
    <Modal :open="characterModal" :title="editingCharacterId ? '编辑角色' : '新增角色'" size="md" @close="characterModal = false">
      <div class="space-y-5">
        <div>
          <label for="char-work" class="field-label">所属作品 *</label>
          <select id="char-work" v-model="characterForm.work_id" class="select-field">
            <option value="">请选择作品</option>
            <option v-for="work in works" :key="work.id" :value="work.id">{{ work.title }}</option>
          </select>
        </div>
        <div>
          <label for="char-name" class="field-label">角色名称 *</label>
          <input id="char-name" v-model="characterForm.name" class="field" />
        </div>
        <div>
          <label for="char-avatar" class="field-label">角色头像（可选，需授权）</label>
          <input id="char-avatar" v-model="characterForm.avatar" class="field" />
        </div>
        <div>
          <label for="char-desc" class="field-label">角色简介</label>
          <textarea id="char-desc" v-model="characterForm.description" rows="3" class="field resize-y" />
        </div>
      </div>
      <template #footer>
        <button class="btn-ghost !min-h-9 text-xs" @click="characterModal = false">取消</button>
        <button class="btn-primary !min-h-9 text-xs" :disabled="saving === 'character'" @click="saveCharacter">
          {{ saving === 'character' ? '保存中' : '保存' }}
        </button>
      </template>
    </Modal>

    <!-- Rule modal -->
    <Modal :open="ruleModal" :title="editingRuleId ? '编辑分类规则' : '新增分类规则'" size="md" @close="ruleModal = false">
      <div class="space-y-5">
        <div>
          <label for="rule-scene" class="field-label">目标场景 *</label>
          <select id="rule-scene" v-model="ruleForm.scene_id" class="select-field">
            <option value="">请选择场景</option>
            <option v-for="scene in scenes" :key="scene.id" :value="scene.id">{{ scene.name }}</option>
          </select>
        </div>
        <div>
          <label for="rule-keywords" class="field-label">关键词（逗号分隔）*</label>
          <textarea
            id="rule-keywords"
            v-model="ruleForm.keywords"
            rows="3"
            class="field resize-y"
            placeholder="例如：剧照, 杀青, 角色, 定档"
          />
          <p class="mt-1.5 text-[10px] leading-5 text-muted">命中任一关键词即归入该场景；同优先级按规则顺序处理。</p>
        </div>
        <div class="grid grid-cols-2 gap-4">
          <div>
            <label for="rule-priority" class="field-label">优先级</label>
            <input id="rule-priority" v-model.number="ruleForm.priority" type="number" class="field" />
          </div>
          <div class="flex items-end pb-2">
            <label class="flex cursor-pointer items-center gap-2 text-xs">
              <input v-model="ruleForm.enabled" type="checkbox" class="h-4 w-4 accent-[#c98791]" />
              启用规则
            </label>
          </div>
        </div>
      </div>
      <template #footer>
        <button class="btn-ghost !min-h-9 text-xs" @click="ruleModal = false">取消</button>
        <button class="btn-primary !min-h-9 text-xs" :disabled="saving === 'rule'" @click="saveRule">
          {{ saving === 'rule' ? '保存中' : '保存' }}
        </button>
      </template>
    </Modal>
  </div>
</template>
