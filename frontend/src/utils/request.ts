import { parse } from 'yaml'

import { useAuthStore } from '@/stores/auth'

type Method = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'

enum ResponseType {
  JSON = 'JSON',
  TEXT = 'TEXT',
  YAML = 'YAML',
}

type RequestOptions = {
  base?: string
  headers?: Record<string, string>
  timeout?: number
  responseType?: keyof typeof ResponseType
  beforeRequest?: () => void
}

export class Request {
  public base: string
  public timeout: number
  public responseType: string
  public beforeRequest: () => void
  public headers: Record<string, string>

  constructor(options: RequestOptions = {}) {
    this.base = options.base || ''
    this.timeout = options.timeout || 10000
    this.responseType = options.responseType || ResponseType.JSON
    this.beforeRequest = options.beforeRequest || (() => 0)
    this.headers = options.headers || {}
  }

  private request = async <T>(
    url: string,
    options: { method: Method; body?: Record<string, any> },
  ) => {
    this.beforeRequest()

    const controller = new AbortController()

    const init: RequestInit = {
      method: options.method,
      signal: controller.signal,
      headers: { ...this.headers },
    }

    if (this.base) {
      url = this.base + url
    }

    const authStore = useAuthStore()
    if (authStore.token) {
      if (!init.headers) init.headers = {}
      Object.assign(init.headers, { Authorization: `Bearer ${authStore.token}` })
    }

    if (['GET'].includes(options.method)) {
      const query = new URLSearchParams(options.body || {}).toString()
      query && (url += '?' + query)
    }

    if (['POST', 'PUT', 'PATCH'].includes(options.method)) {
      if (!init.headers) init.headers = {}
      Object.assign(init.headers, { 'Content-Type': 'application/json' })
      init.body = JSON.stringify(options.body || {})
    }

    const id = setTimeout(() => controller.abort(), this.timeout)

    const res = await fetch(url, init)

    clearTimeout(id)

    if (res.status === 204) {
      return null as T
    }

    if ([504, 401, 503].includes(res.status)) {
      const { message } = await res.json()
      throw message
    }

    if (this.responseType === ResponseType.TEXT) {
      const text = await res.text()
      return text as T
    }

    if (this.responseType === ResponseType.YAML) {
      const text = await res.text()
      return parse(text) as T
    }

    const json = await res.json()
    return json as T
  }

  public get = <T>(url: string, body = {}) => this.request<T>(url, { method: 'GET', body })
  public post = <T>(url: string, body = {}) => this.request<T>(url, { method: 'POST', body })
  public put = <T>(url: string, body = {}) => this.request<T>(url, { method: 'PUT', body })
  public patch = <T>(url: string, body = {}) => this.request<T>(url, { method: 'PATCH', body })
  public delete = <T>(url: string) => this.request<T>(url, { method: 'DELETE' })
}
