export interface User {
  id: string
  email: string
  nickname: string
  avatar: string
  role: 'admin' | 'editor' | 'user'
  status?: string
  created_at: string
}

export interface Artist {
  id: string
  name: string
  name_en: string
  avatar: string
  cover: string
  bio: string
  profile_data: Record<string, unknown>
}

export interface Scene {
  id: string
  code: string
  name: string
  description: string
  icon: string
  sort_order: number
  enabled: boolean
  weight?: number
  selected?: boolean
  is_default?: boolean
}

export interface WorkRef {
  id: string
  title: string
  type: string
  release_date: string
  cover: string
}

export interface CharacterRef {
  id: string
  name: string
  work_id: string
  work_title?: string
}

export interface Character {
  id: string
  work_id: string
  name: string
  avatar: string
  description: string
  work?: WorkRef
  favorited?: boolean
}

export interface Work {
  id: string
  artist_id: string
  title: string
  type: string
  cover: string
  description: string
  release_date: string
  status: string
  platform: string
  official_url: string
  characters?: Character[]
  favorited?: boolean
}

export type ContentType =
  | 'social'
  | 'drama'
  | 'variety'
  | 'media'
  | 'topic'
  | 'image'
  | 'fashion'
  | 'event'
  | 'internal'
  | 'other'

export interface Content {
  id: string
  title: string
  summary: string
  content_type: ContentType
  source_id?: string
  source_name: string
  source_platform: string
  source_type: string
  source_author: string
  source_url: string
  cover_url: string
  published_at: string
  collected_at: string
  trust_level: string
  is_official: boolean
  review_status: string
  content_status: string
  review_reason?: string
  scenes: Scene[]
  works: WorkRef[]
  characters: CharacterRef[]
  favorited: boolean
  unavailable?: boolean
  raw_data?: unknown
}

export interface HomeData {
  artist: Artist
  scenes: Scene[]
  default_scene: string
  latest: Content[]
  for_you: Content[]
  featured: Content[]
  works: Work[]
  brief: {
    date: string
    new_count: number
    last_updated_at: string | null
  }
}

export interface WorkDetail {
  work: Work
  contents: Content[]
}

export interface CharacterDetail {
  character: Character
  contents: Content[]
}

export interface FavoriteItem {
  id: string
  target_type: 'content' | 'work' | 'character'
  target_id: string
  title: string
  subtitle: string
  source_url: string
  cover: string
  created_at: string
  unavailable: boolean
  snapshot?: Record<string, unknown>
}

export interface Source {
  id: string
  name: string
  platform: string
  source_type: string
  adapter_type: string
  config: Record<string, unknown>
  trust_level: string
  enabled: boolean
  interval_minutes: number
  last_success_at: string | null
  last_failure_at: string | null
  last_error: string
  created_at: string
}

export interface Job {
  id: string
  source_id: string
  source_name: string
  status: 'queued' | 'running' | 'success' | 'failed' | 'skipped'
  trigger_type: string
  started_at: string | null
  finished_at: string | null
  fetched_count: number
  inserted_count: number
  duplicate_count: number
  failed_count: number
  error_message: string
  created_at: string
}

export interface Overview {
  today_collected: number
  today_inserted: number
  pending_reviews: number
  source_errors: number
  success_rate: number
  recent_jobs: Job[]
}

export interface Rule {
  id: string
  scene_id: string
  scene_name: string
  keywords: string
  priority: number
  enabled: boolean
}

export interface RecommendWeights {
  scene: number
  recency: number
  trust: number
  popularity: number
}

export interface Paged<T> {
  items: T[]
  page: number
  page_size: number
  total: number
}

export interface SetupStatus {
  needs_setup: boolean
  allow_registration: boolean
}

export interface AuthResponse {
  user: User
  csrf_token: string
}

export interface MeResponse {
  user: User | null
  csrf_token: string
  allow_registration: boolean
}
