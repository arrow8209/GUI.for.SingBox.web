import { getStoredCsrf, useAuthStore } from '@/stores/auth'

const API_BASE = import.meta.env.VITE_API_BASE || '/api'

type RequestOptions = {
  method?: string
  body?: any
  headers?: Record<string, string>
  skipAuth?: boolean
}

async function request<T>(path: string, options: RequestOptions = {}) {
  const method = options.method || 'GET'
  const init: RequestInit = {
    method,
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      ...options.headers,
    },
  }

  // CSRF: 状态变更方法必须带 token
  if (!['GET', 'HEAD', 'OPTIONS'].includes(method)) {
    const csrf = getStoredCsrf()
    if (csrf) {
      ;(init.headers as Record<string, string>)['X-CSRF-Token'] = csrf
    }
  }

  if (options.body !== undefined) {
    init.body = JSON.stringify(options.body)
  }

  const res = await fetch(`${API_BASE}${path}`, init)
  const isJSON = res.headers.get('Content-Type')?.includes('application/json')
  const payload = isJSON ? await res.json() : await res.text()

  if (!res.ok) {
    if (res.status === 401) {
      const authStore = useAuthStore()
      authStore.forceLogout()
    }
    const message = typeof payload === 'string' ? payload : payload?.error || 'Request failed'
    throw new Error(message)
  }

  return payload as T
}

export const httpClient = {
  get: <T>(path: string) => request<T>(path, { method: 'GET' }),
  post: <T>(path: string, body?: any) => request<T>(path, { method: 'POST', body }),
  request,
}

export const apiBaseURL = API_BASE
