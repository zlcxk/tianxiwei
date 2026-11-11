let csrfToken = ''

export function setCsrfToken(token: string) {
  csrfToken = token
}

export class ApiError extends Error {
  status: number
  code: string

  constructor(status: number, code: string, message: string) {
    super(message)
    this.status = status
    this.code = code
  }
}

type RequestOptions = {
  method?: string
  body?: unknown
  signal?: AbortSignal
}

async function throwApiError(res: Response): Promise<never> {
  let code = 'request_failed'
  let message = `请求失败（${res.status}）`
  try {
    const data = await res.json()
    if (data?.error) {
      code = data.error.code || code
      message = data.error.message || message
    }
  } catch {
    /* keep fallback */
  }
  throw new ApiError(res.status, code, message)
}

async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const method = (options.method || 'GET').toUpperCase()
  const headers = new Headers({ Accept: 'application/json' })
  if (options.body !== undefined) headers.set('Content-Type', 'application/json')
  if (!['GET', 'HEAD', 'OPTIONS'].includes(method) && csrfToken) {
    headers.set('X-CSRF-Token', csrfToken)
  }
  const res = await fetch('/api/v1' + path, {
    method,
    headers,
    credentials: 'include',
    body: options.body === undefined ? undefined : JSON.stringify(options.body),
    signal: options.signal,
  })
  if (!res.ok) await throwApiError(res)
  if (res.status === 204) return undefined as T
  return (await res.json()) as T
}

export type UploadedImage = {
  url: string
  filename: string
  content_type: string
  size: number
}

/**
 * Uploads an image for admin/editor use. The API sniffs the real MIME type and
 * caps the file at 10MB; this client-side check is only a fast path.
 */
export async function uploadImage(file: File): Promise<UploadedImage> {
  const headers = new Headers({ Accept: 'application/json' })
  if (csrfToken) headers.set('X-CSRF-Token', csrfToken)
  const body = new FormData()
  body.append('file', file)
  const res = await fetch('/api/v1/admin/media/images', {
    method: 'POST',
    headers,
    credentials: 'include',
    body,
  })
  if (!res.ok) await throwApiError(res)
  return (await res.json()) as UploadedImage
}

export const api = {
  get<T>(path: string, signal?: AbortSignal) {
    return request<T>(path, { signal })
  },
  post<T>(path: string, body?: unknown) {
    return request<T>(path, { method: 'POST', body })
  },
  put<T>(path: string, body?: unknown) {
    return request<T>(path, { method: 'PUT', body })
  },
  patch<T>(path: string, body?: unknown) {
    return request<T>(path, { method: 'PATCH', body })
  },
  del<T>(path: string, body?: unknown) {
    return request<T>(path, { method: 'DELETE', body })
  },
}
