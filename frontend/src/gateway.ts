// Gateway client: one multiplexed WebSocket to /gateway. Reconnects with backoff
// and re-subscribes to the channels it had open.
import type { Message, Participant } from './api'

export interface GatewayHandlers {
  onReady?: (who: Participant) => void
  onMessage?: (channel: string, msg: Message) => void
  onHistory?: (channel: string, msgs: Message[]) => void
  onPresenceSnapshot?: (channel: string, roster: Participant[]) => void
  onPresence?: (channel: string, state: string, who: Participant) => void
  onStatus?: (connected: boolean) => void
}

export class Gateway {
  private ws: WebSocket | null = null
  private subs = new Set<string>()
  private closedByUser = false
  private backoff = 500

  constructor(private handlers: GatewayHandlers) {}

  connect() {
    this.closedByUser = false
    const proto = location.protocol === 'https:' ? 'wss' : 'ws'
    const ws = new WebSocket(`${proto}://${location.host}/gateway`)
    this.ws = ws
    ws.onopen = () => {
      this.backoff = 500
      this.handlers.onStatus?.(true)
      for (const ch of this.subs) this.rawSubscribe(ch)
    }
    ws.onclose = () => {
      this.handlers.onStatus?.(false)
      if (!this.closedByUser) setTimeout(() => this.connect(), this.backoff = Math.min(this.backoff * 2, 8000))
    }
    ws.onmessage = (ev) => this.handle(ev.data)
  }

  close() {
    this.closedByUser = true
    this.ws?.close()
    this.ws = null
  }

  subscribe(channel: string) {
    this.subs.add(channel)
    this.rawSubscribe(channel)
  }

  send(channel: string, body: string) {
    this.rawSend({ type: 'message', channel, body })
  }

  private rawSubscribe(channel: string) {
    this.rawSend({ type: 'subscribe', channels: [channel] })
  }

  private rawSend(obj: unknown) {
    if (this.ws?.readyState === WebSocket.OPEN) this.ws.send(JSON.stringify(obj))
  }

  private handle(data: string) {
    let f: any
    try { f = JSON.parse(data) } catch { return }
    switch (f.type) {
      case 'ready': this.handlers.onReady?.(f.session); break
      case 'message': this.handlers.onMessage?.(f.channel, f.message); break
      case 'history': this.handlers.onHistory?.(f.channel, f.messages ?? []); break
      case 'presence':
        if (f.roster) this.handlers.onPresenceSnapshot?.(f.channel, f.roster)
        else if (f.state && f.who) this.handlers.onPresence?.(f.channel, f.state, f.who)
        break
    }
  }
}
