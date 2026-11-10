import { computed, reactive } from 'vue'
import { api, setCsrfToken } from '../api/client'
import type { AuthResponse, MeResponse, SetupStatus, User } from '../types'

const state = reactive({
  user: null as User | null,
  csrf: '',
  loaded: false,
  needsSetup: false,
  allowRegistration: false,
  error: '',
})

let bootPromise: Promise<void> | null = null

export const auth = state
export const isAdmin = computed(() => state.user?.role === 'admin' || state.user?.role === 'editor')

export async function ensureBootstrap(): Promise<void> {
  if (state.loaded) return
  if (!bootPromise) {
    bootPromise = (async () => {
      try {
        const [status, me] = await Promise.all([
          api.get<SetupStatus>('/setup/status'),
          api.get<MeResponse>('/auth/me'),
        ])
        state.needsSetup = status.needs_setup
        state.allowRegistration = me.allow_registration || status.allow_registration
        applySession(me)
        state.error = ''
      } catch (err) {
        state.error = err instanceof Error ? err.message : '服务暂时不可用'
        state.loaded = true
        throw err
      }
      state.loaded = true
    })()
  }
  return bootPromise
}

function applySession(me: MeResponse) {
  state.user = me.user
  state.csrf = me.csrf_token
  setCsrfToken(me.csrf_token)
}

export async function refreshMe() {
  const me = await api.get<MeResponse>('/auth/me')
  state.allowRegistration = me.allow_registration
  applySession(me)
  state.loaded = true
}

export async function login(email: string, password: string) {
  const res = await api.post<AuthResponse>('/auth/login', { email, password })
  state.user = res.user
  state.csrf = res.csrf_token
  state.needsSetup = false
  setCsrfToken(res.csrf_token)
}

export async function register(email: string, password: string, nickname: string) {
  const res = await api.post<AuthResponse>('/auth/register', { email, password, nickname })
  state.user = res.user
  state.csrf = res.csrf_token
  state.needsSetup = false
  state.loaded = true
  setCsrfToken(res.csrf_token)
}

export async function setupAdmin(email: string, password: string, nickname: string) {
  const res = await api.post<AuthResponse>('/setup', { email, password, nickname })
  state.user = res.user
  state.csrf = res.csrf_token
  state.needsSetup = false
  state.loaded = true
  setCsrfToken(res.csrf_token)
}

export async function logout() {
  try {
    await api.post('/auth/logout')
  } catch {
    /* clearing the local state is still correct */
  }
  state.user = null
  state.csrf = ''
  setCsrfToken('')
  bootPromise = null
}

export function resetBootstrap() {
  bootPromise = null
  state.loaded = false
}
