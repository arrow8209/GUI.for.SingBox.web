import { httpClient } from './http'
import { message } from '@/utils'

interface IOOptions {
  Mode?: 'Binary' | 'Text'
}

const assertFlag = (res: { flag: boolean; data: string }) => {
  if (!res.flag) throw res.data
  return res.data
}

export const WriteFile = async (path: string, content: string, options: IOOptions = {}) => {
  const res = await httpClient.post<{ flag: boolean; data: string }>('/files/write', {
    path,
    content,
    mode: options.Mode || 'Text',
  })
  return assertFlag(res)
}

export const ReadFile = async (path: string, options: IOOptions = {}) => {
  const res = await httpClient.post<{ flag: boolean; data: string }>('/files/read', {
    path,
    mode: options.Mode || 'Text',
  })
  return assertFlag(res)
}

export const MoveFile = async (source: string, target: string) => {
  const res = await httpClient.post<{ flag: boolean; data: string }>('/files/move', { source, target })
  return assertFlag(res)
}

export const RemoveFile = async (path: string) => {
  const res = await httpClient.post<{ flag: boolean; data: string }>('/files/remove', { path })
  return assertFlag(res)
}

export const CopyFile = async (source: string, target: string) => {
  const res = await httpClient.post<{ flag: boolean; data: string }>('/files/copy', { source, target })
  return assertFlag(res)
}

export const FileExists = async (path: string) => {
  const res = await httpClient.post<{ flag: boolean; data: string }>('/files/exists', { path })
  return assertFlag(res) === 'true'
}

export const AbsolutePath = async (path: string) => {
  const res = await httpClient.post<{ flag: boolean; data: string }>('/files/absolute', { path })
  return assertFlag(res)
}

export const MakeDir = async (path: string) => {
  const res = await httpClient.post<{ flag: boolean; data: string }>('/files/mkdir', { path })
  return assertFlag(res)
}

export const ReadDir = async (path: string) => {
  const res = await httpClient.post<{ flag: boolean; data: string }>('/files/list', { path })
  const data = assertFlag(res)
  return data
    .split('|')
    .filter(Boolean)
    .map((entry) => {
      const [name, size, isDir] = entry.split(',')
      return { name, size: Number(size), isDir: isDir === 'true' }
    })
}

export const OpenDir = async (_path: string) => {
  message.warn('Opening directories is not supported in the web version.')
}

export const OpenURI = async (uri: string) => {
  if (/^https?:\/\//i.test(uri)) {
    window.open(uri, '_blank', 'noopener')
    return
  }
  message.warn('Opening local paths is not supported in the web version.')
}

export const UnzipZIPFile = async (path: string, output: string) => {
  const res = await httpClient.post<{ flag: boolean; data: string }>('/files/unzip/zip', { path, output })
  return assertFlag(res)
}

export const UnzipGZFile = async (path: string, output: string) => {
  const res = await httpClient.post<{ flag: boolean; data: string }>('/files/unzip/gz', { path, output })
  return assertFlag(res)
}

export const UnzipTarGZFile = async (path: string, output: string) => {
  const res = await httpClient.post<{ flag: boolean; data: string }>('/files/unzip/targz', { path, output })
  return assertFlag(res)
}

export const Writefile = WriteFile
export const Readfile = ReadFile
export const Movefile = MoveFile
export const Removefile = RemoveFile
export const Copyfile = CopyFile
export const Makedir = MakeDir
export const Readdir = ReadDir
