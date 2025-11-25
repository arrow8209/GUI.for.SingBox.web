import { httpClient } from './http'

import type { TrayContent } from '@/types/app'

export const RestartApp = async () => {
  return httpClient.post<{ flag: boolean; data: string }>('/restart').then((res) => {
    if (!res.flag) throw res.data
    return res
  })
}

export const ExitApp = async () => {
  await httpClient.post('/exit')
}

export const ShowMainWindow = async () => {
  // No concept of native window in B/S mode.
}

export const UpdateTray = async (_tray: TrayContent) => {
  // Tray not supported in web mode.
}

export const UpdateTrayMenus = async () => {
  // Tray menus not supported in web mode.
}

export const GetEnv = () => httpClient.get<EnvResult>('/env')

export const IsStartup = () => httpClient.get<{ startup: boolean }>('/startup').then((res) => res.startup)

export const GetInterfaces = async () => {
  const { flag, data } = await httpClient.get<{ flag: boolean; data: string }>('/interfaces')
  if (!flag) throw data
  return data.split('|').filter(Boolean)
}

export type EnvResult = {
  appName: string
  appVersion: string
  basePath: string
  os: string
  arch: string
}
