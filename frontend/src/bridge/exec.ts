import { httpClient } from './http'
import { EventsOn, EventsOff } from './events'
import { sampleID } from '@/utils'

interface ExecOptions {
  Convert?: boolean
  Env?: Record<string, any>
  StopOutputKeyword?: string
  convert?: boolean
  env?: Record<string, any>
  stopOutputKeyword?: string
}

const mergeExecOptions = (options: ExecOptions = {}) => ({
  Convert: options.Convert ?? options.convert ?? false,
  Env: options.Env ?? options.env ?? {},
  StopOutputKeyword: options.StopOutputKeyword ?? options.stopOutputKeyword ?? '',
})

const assertFlag = (res: { flag: boolean; data: string }) => {
  if (!res.flag) throw res.data
  return res.data
}

export const Exec = async (path: string, args: string[], options: ExecOptions = {}) => {
  const res = await httpClient.post<{ flag: boolean; data: string }>('/exec/run', {
    path,
    args,
    options: mergeExecOptions(options),
  })
  return assertFlag(res)
}

export const ExecBackground = async (
  path: string,
  args: string[] = [],
  onOut?: (out: string) => void,
  onEnd?: () => void,
  options: ExecOptions = {},
) => {
  const mergedOptions = mergeExecOptions(options)
  const outEvent = onOut ? sampleID() : ''
  const endEvent = onEnd ? sampleID() : outEvent ? sampleID() : ''

  if (outEvent) {
    EventsOn(outEvent, onOut!)
  }

  if (endEvent) {
    EventsOn(endEvent, () => {
      outEvent && EventsOff(outEvent)
      EventsOff(endEvent)
      onEnd?.()
    })
  }

  try {
    const res = await httpClient.post<{ flag: boolean; data: string }>('/exec/background', {
      path,
      args,
      outEvent,
      endEvent,
      options: mergedOptions,
    })

    assertFlag(res)

    return Number(res.data)
  } catch (err) {
    outEvent && EventsOff(outEvent)
    endEvent && EventsOff(endEvent)
    throw err
  }
}

export const ProcessInfo = async (pid: number) => {
  const res = await httpClient.post<{ flag: boolean; data: string }>('/exec/process-info', { pid })
  return assertFlag(res)
}

export const ProcessMemory = async (pid: number) => {
  const res = await httpClient.post<{ flag: boolean; data: string }>('/exec/process-memory', { pid })
  return Number(assertFlag(res))
}

export const KillProcess = async (pid: number, timeout = 10) => {
  const res = await httpClient.post<{ flag: boolean; data: string }>('/exec/kill', { pid, timeout })
  return assertFlag(res)
}
