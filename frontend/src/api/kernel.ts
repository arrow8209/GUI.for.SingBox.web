import { Request } from '@/api/request'
import { WebSockets } from '@/api/websocket'
import { apiBaseURL } from '@/bridge/http'

import type {
  CoreApiConfig,
  CoreApiProxies,
  CoreApiConnections,
  CoreApiWsDataMap,
} from '@/types/kernel'

export enum Api {
  Configs = '/configs',
  Memory = '/memory',
  Proxies = '/proxies',
  ProxyDelay = '/proxies/{0}/delay',
  Connections = '/connections',
  Traffic = '/traffic',
  Logs = '/logs',
}

type WsKey = keyof CoreApiWsDataMap
type WsChannel<K extends WsKey> = {
  url: string
  params?: Recordable
  handlers: Array<(data: CoreApiWsDataMap[K]) => void>
  isActive: boolean
  connect?: () => void
  disconnect?: () => void
}

// HTTP/WS 都走本地 Core Proxy；bearer 由服务端从 active profile 读取，
// 前端不再传 X-Core-Bearer / token。Cookie 自动带 session。
const setupKernelApi = () => {
  request.base = getCoreProxyBase()
}

const setupKernelWs = () => {
  websocket.base = getCoreProxyBase().replace(/^http/, 'ws')
  websocket.params = {}
}

// selectProfile 在切换 profile 时调用，让服务端持有 bearer。
export const selectProfile = (profileId: string) =>
  request.post<{ status: string }>('/select-profile', { profileId })

const request = new Request({ beforeRequest: setupKernelApi, timeout: 60 * 1000 })
const websocket = new WebSockets({ beforeConnect: setupKernelWs })

const wsChannels: { [K in WsKey]: WsChannel<K> } = {
  logs: { url: Api.Logs, isActive: false, handlers: [], params: { level: 'debug' } },
  memory: { url: Api.Memory, isActive: false, handlers: [] },
  traffic: { url: Api.Traffic, isActive: false, handlers: [] },
  connections: { url: Api.Connections, isActive: false, handlers: [] },
}

const createCoreWSHandlerRegister = <K extends WsKey>(key: K) => {
  const channel = wsChannels[key]
  return (cb: (data: CoreApiWsDataMap[K]) => void) => {
    channel.handlers.push(cb)
    if (!channel.isActive && channel.connect) {
      channel.connect()
      channel.isActive = true
    }
    const unregister = () => {
      const idx = channel.handlers.indexOf(cb)
      idx !== -1 && channel.handlers.splice(idx, 1)
      if (channel.isActive && channel.disconnect && channel.handlers.length === 0) {
        channel.disconnect()
        channel.isActive = false
      }
    }
    return unregister
  }
}

// restful api
export const getConfigs = () => request.get<CoreApiConfig>(Api.Configs)
export const setConfigs = (body = {}) => request.patch<null>(Api.Configs, body)
export const getProxies = () => request.get<CoreApiProxies>(Api.Proxies)
export const getConnections = () => request.get<CoreApiConnections>(Api.Connections)
export const deleteConnection = (id: string) => request.delete<null>(Api.Connections + '/' + id)
export const useProxy = (group: string, proxy: string) => {
  return request.put<null>(Api.Proxies + '/' + group, { name: proxy })
}
export const getProxyDelay = (proxy: string, url: string, timeout: number) => {
  return request.get<Record<string, number>>(Api.ProxyDelay.replace('{0}', proxy), {
    url,
    timeout,
  })
}

// websocket api
export const onLogs = createCoreWSHandlerRegister('logs')
export const onMemory = createCoreWSHandlerRegister('memory')
export const onTraffic = createCoreWSHandlerRegister('traffic')
export const onConnections = createCoreWSHandlerRegister('connections')

export const initWebsocket = () => {
  Object.values(wsChannels).forEach((channel) => {
    const { connect, disconnect } = websocket.createWS([
      {
        name: channel.url,
        url: channel.url,
        params: channel.params,
        cb: (data: any) => channel.handlers.forEach((cb) => cb(data)),
      },
    ])
    channel.connect = connect
    channel.disconnect = disconnect
    channel.isActive = false
    if (channel.handlers.length > 0) {
      channel.connect()
      channel.isActive = true
    }
  })
}

export const destroyWebsocket = () => {
  Object.values(wsChannels).forEach((channel) => {
    channel.disconnect?.()
    channel.connect = undefined
    channel.disconnect = undefined
    channel.isActive = false
  })
}

// resolveCoreConnection 已废弃：bearer/coreBase 由服务端从 active profile 读取。
// 仅保留导出名以避免破坏现有 import；返回空值。
export const resolveCoreConnection = (): { coreBase: string; coreBearer: string } => {
  return { coreBase: '', coreBearer: '' }
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
    url.pathname =
      (url.pathname.endsWith('/') ? url.pathname.slice(0, -1) : url.pathname) + '/core'
    url.search = ''
    url.hash = ''
    return url.toString()
  } catch {
    return '/api/core'
  }
}
