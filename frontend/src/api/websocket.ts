type WebSocketsOptions = {
  base?: string
  params?: Record<string, string>
  beforeConnect?: () => void
}

type URLType = { name: string; url: string; cb: (data: any) => void; params?: Record<string, any> }

export class WebSockets {
  public base: string
  public params: Record<string, string>
  public beforeConnect: () => void

  constructor(options: WebSocketsOptions) {
    this.base = options.base || ''
    this.params = options.params || {}
    this.beforeConnect = options.beforeConnect || (() => 0)
  }

  private buildURL(url: string, params: Record<string, any>) {
    const finalParams = new URLSearchParams({ ...this.params, ...params }).toString()
    return this.base + url + (finalParams ? `?${finalParams}` : '')
  }

  public createWS(urls: URLType[]) {
    const wsMap: Record<string, { ready: boolean; close: () => void; open: () => void }> = {}

    urls.forEach(({ name, url, params = {}, cb }) => {
      const open = () => {
        if (!wsMap[name]!.ready) return
        const ws = new WebSocket(this.buildURL(url, params))
        ws.onmessage = (e) => cb(JSON.parse(e.data))
        ws.onerror = () => (wsMap[name]!.ready = true)
        ws.onclose = () => (wsMap[name]!.ready = true)
        wsMap[name]!.close = () => {
          ws.close()
          wsMap[name]!.ready = true
        }
        wsMap[name]!.ready = false
      }

      wsMap[name] = { ready: true, open, close: () => (wsMap[name]!.ready = false) }
    })

    return {
      connect: () => {
        this.beforeConnect()
        Object.values(wsMap).forEach((ws) => ws.open())
      },
      disconnect: () => Object.values(wsMap).forEach((ws) => ws.close()),
    }
  }
}
