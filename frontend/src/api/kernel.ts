import { apiBaseURL } from '@/bridge/http'
import { useAppSettingsStore, useProfilesStore } from '@/stores'
import { Request } from '@/utils/request'

import type { CoreApiConfig, CoreApiProxies, CoreApiConnections } from '@/types/kernel'

export enum Api {
  Configs = '/configs',
  Memory = '/memory',
  Proxies = '/proxies',
  ProxyDelay = '/proxies/{0}/delay',
  Connections = '/connections',
  Traffic = '/traffic',
  Logs = '/logs',
}

type CoreConnectionOptions = {
  coreBase: string
  coreBearer: string
}

const setupKernelApi = () => {
  const { coreBase, coreBearer } = resolveCoreConnection()
  request.base = getCoreProxyBase()
  request.headers = {
    'X-Core-Base': coreBase,
    ...(coreBearer ? { 'X-Core-Bearer': coreBearer } : {}),
  }
}

const request = new Request({ beforeRequest: setupKernelApi, timeout: 60 * 1000 })

export const getConfigs = () => request.get<CoreApiConfig>(Api.Configs)

export const setConfigs = (body = {}) => request.patch<null>(Api.Configs, body)

export const getProxies = () => request.get<CoreApiProxies>(Api.Proxies)

export const getConnections = () => request.get<CoreApiConnections>(Api.Connections)

export const deleteConnection = (id: string) => request.delete<null>(Api.Connections + '/' + id)

export const useProxy = (group: string, proxy: string) => {
  return request.put<null>(Api.Proxies + '/' + group, { name: proxy })
}

export const getProxyDelay = (proxy: string, url: string) => {
  return request.get<Record<string, number>>(Api.ProxyDelay.replace('{0}', proxy), {
    url,
    timeout: 5000,
  })
}

export const resolveCoreConnection = (): CoreConnectionOptions => {
  const appSettingsStore = useAppSettingsStore()
  const profilesStore = useProfilesStore()
  const profile = profilesStore.getProfileById(appSettingsStore.app.kernel.profile)
  const controller = (profile?.experimental.clash_api.external_controller || '127.0.0.1:20123').trim()
  let normalized = controller
  if (!normalized.includes('://')) {
    normalized = `http://${normalized}`
  }
  let coreBase = 'http://127.0.0.1:20123'
  try {
    const url = new URL(normalized)
    let host = url.hostname || '127.0.0.1'
    if (host === '0.0.0.0') host = '127.0.0.1'
    if (host === '' || host === '*') host = '127.0.0.1'
    if (host === '::') host = '::1'
    if (!host.startsWith('127.') && host !== '::1' && host !== 'localhost') {
      host = '127.0.0.1'
    }
    const port = url.port || '20123'
    const protocol = url.protocol === 'https:' ? 'https' : 'http'
    coreBase = `${protocol}://${host}:${port}`
  } catch (error) {
    console.error('[kernelApi] failed to parse controller address, fallback to loopback', error)
  }
  return {
    coreBase,
    coreBearer: profile?.experimental.clash_api.secret || '',
  }
}

export const getCoreProxyBase = () => {
  let base = apiBaseURL || '/api'
  if (base.endsWith('/')) {
    base = base.slice(0, -1)
  }
  if (!base.startsWith('http')) {
    if (!base.startsWith('/')) {
      base = '/' + base
    }
    return `${base}/core`
  }
  try {
    const url = new URL(base, window.location.origin)
    url.pathname = (url.pathname.endsWith('/') ? url.pathname.slice(0, -1) : url.pathname) + '/core'
    url.search = ''
    url.hash = ''
    return url.toString()
  } catch {
    return '/api/core'
  }
}
