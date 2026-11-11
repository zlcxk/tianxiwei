const dateFmt = new Intl.DateTimeFormat('zh-CN', {
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
})

const dateTimeFmt = new Intl.DateTimeFormat('zh-CN', {
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
  hour12: false,
})

export function formatDate(value?: string | null): string {
  if (!value) return '—'
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return value
  return dateFmt.format(d).replaceAll('/', '.')
}

export function formatDateTime(value?: string | null): string {
  if (!value) return '—'
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return value
  return dateTimeFmt.format(d).replaceAll('/', '.').replace(',', '')
}

export function formatYear(value?: string | null): string {
  if (!value) return ''
  return value.slice(0, 4)
}

export function relativeTime(value?: string | null): string {
  if (!value) return ''
  const then = new Date(value).getTime()
  if (Number.isNaN(then)) return ''
  const diff = Date.now() - then
  const minute = 60_000
  const hour = 60 * minute
  const day = 24 * hour
  if (diff < minute) return '刚刚'
  if (diff < hour) return `${Math.floor(diff / minute)} 分钟前`
  if (diff < day) return `${Math.floor(diff / hour)} 小时前`
  if (diff < 7 * day) return `${Math.floor(diff / day)} 天前`
  return formatDate(value)
}

export function todayLabel(): string {
  const d = new Date()
  const week = ['日', '一', '二', '三', '四', '五', '六'][d.getDay()]
  return `${d.getFullYear()}年${d.getMonth() + 1}月${d.getDate()}日 · 周${week}`
}

export const contentTypeLabels: Record<string, string> = {
  social: '社交动态',
  post: '社交帖子',
  news: '新闻资讯',
  drama: '影视资讯',
  variety: '综艺',
  media: '新闻媒体',
  video: '视频',
  interview: '采访',
  image: '图片影像',
  fashion: '时尚商务',
  event: '公开活动',
  work: '作品信息',
  official: '官方公告',
  topic: '热门话题',
  internal: '站内资料',
  other: '其他',
}

export const precisionLabels: Record<string, string> = {
  verified: '来源明确',
  estimated: '时间待核验',
  unknown: '发布时间未知',
}

export interface TimelineCategoriesItem {
  key: string
  label: string
  types?: string[]
  scene?: string
}

export const timelineCategories: TimelineCategoriesItem[] = [
  { key: 'all', label: '全部' },
  { key: 'news', label: '新闻', types: ['news', 'media', 'official'] },
  { key: 'post', label: '帖子', types: ['post', 'social'] },
  { key: 'image', label: '图片', types: ['image'] },
  { key: 'video', label: '视频', types: ['video', 'interview'] },
  { key: 'work', label: '作品', types: ['work', 'drama', 'variety'] },
  { key: 'event', label: '活动', types: ['event'] },
  { key: 'daily', label: '日常', scene: 'daily' },
  { key: 'business', label: '商务', scene: 'fashion' },
]

export function contentTimeLabel(content: {
  published_at: string | null
  discovered_at: string
  published_precision: string
}): string {
  if (content.published_precision === 'unknown' || !content.published_at) {
    return `发现于 ${relativeTime(content.discovered_at) || '近期'}`
  }
  if (content.published_precision === 'estimated') {
    return `约 ${relativeTime(content.published_at)}`
  }
  return relativeTime(content.published_at)
}

export function contentDateLabel(content: {
  published_at: string | null
  discovered_at: string
  published_precision: string
}): string {
  if (content.published_precision === 'unknown' || !content.published_at) {
    return `发布时间未知 · 发现于 ${formatDate(content.discovered_at)}`
  }
  if (content.published_precision === 'estimated') {
    return `约 ${formatDateTime(content.published_at)}（待核验）`
  }
  return formatDateTime(content.published_at)
}

export const workTypeLabels: Record<string, string> = {
  drama: '剧集',
  movie: '电影',
  variety: '综艺',
  other: '其他',
}

export const workStatusLabels: Record<string, string> = {
  aired: '已播出',
  airing: '播出中',
  upcoming: '待播出',
}

export const sourceTypeLabels: Record<string, string> = {
  official: '官方',
  media: '媒体',
  community: '社区',
  reference: '资料',
  internal: '站内',
}

export const trustLabels: Record<string, string> = {
  official: '官方来源',
  media: '媒体报道',
  community: '社区内容',
  reference: '参考资料',
  internal: '站内整理',
}

export const contentStatusLabels: Record<string, string> = {
  pending: '待处理',
  reviewing: '待审核',
  published: '已发布',
  rejected: '已拒绝',
  expired: '已过期',
  archived: '已归档',
}

export const reviewStatusLabels: Record<string, string> = {
  none: '无需审核',
  pending: '待审核',
  approved: '已通过',
  rejected: '已拒绝',
}

export const relevanceLabels: Record<string, string> = {
  high: '高度相关',
  candidate: '相关候选',
  filtered: '已过滤',
}

export const filterReasonLabels: Record<string, string> = {
  irrelevant_person: '与人物无关',
  ambiguous_entity: '实体指向不明确',
  dictionary_content: '姓氏/字典内容',
  unrelated_person: '其他人物内容',
  insufficient_context: '缺少人物上下文',
  duplicate_content: '重复内容',
  invalid_source: '来源无效',
}

export const jobStatusLabels: Record<string, string> = {
  queued: '排队中',
  running: '采集中',
  success: '成功',
  partial: '部分成功',
  failed: '失败',
  skipped: '已跳过',
}

export const favoriteTypeLabels: Record<string, string> = {
  content: '动态',
  work: '作品',
  character: '角色',
}
