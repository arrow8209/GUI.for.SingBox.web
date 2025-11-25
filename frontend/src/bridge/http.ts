const API_BASE = import.meta.env.VITE_API_BASE || '/api'

type RequestOptions = {
  method?: string
  body?: any
  headers?: Record<string, string>
}

async function request<T>(path: string, options: RequestOptions = {}) {
  const init: RequestInit = {
    method: options.method || 'GET',
    headers: {
      'Content-Type': 'application/json',
      ...(options.headers || {}),
    },
  }

  if (options.body !== undefined) {
    init.body = JSON.stringify(options.body)
  }

  const res = await fetch(`${API_BASE}${path}`, init)
  const isJSON = res.headers.get('Content-Type')?.includes('application/json')
  const payload = isJSON ? await res.json() : await res.text()

  if (!res.ok) {
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
