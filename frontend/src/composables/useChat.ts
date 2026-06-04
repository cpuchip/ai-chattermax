import { ref, type Ref } from 'vue'

export interface ChatMessage {
  sender: string
  body: string
}

export interface ChatOptions {
  displayName: string
  room: string
  wsBaseUrl?: string
  rosterBaseUrl?: string
}

export interface UseChatReturn {
  messages: Ref<ChatMessage[]>
  roster: Ref<Array<{ id: string; kind?: string; online?: boolean }>>
  connect: (opts: ChatOptions) => void
  send: (text: string) => void
  disconnect: () => void
}

export function useChat(): UseChatReturn {
  const messages = ref<ChatMessage[]>([])
  const roster = ref<Array<{ id: string; kind?: string; online?: boolean }>>([])

  let ws: WebSocket | null = null
  let intervalId: ReturnType<typeof setInterval> | null = null
  let currentOpts: ChatOptions | null = null

  function connect(opts: ChatOptions) {
    disconnect()
    currentOpts = opts

    const wsUrl = `${opts.wsBaseUrl || ''}/ws/${encodeURIComponent(opts.room)}?id=${encodeURIComponent(opts.displayName)}`
    ws = new WebSocket(wsUrl)

    ws.addEventListener('message', (event) => {
      try {
        const parsed = JSON.parse(event.data)
        if (parsed.sender && parsed.body) {
          messages.value.push({ sender: parsed.sender, body: parsed.body })
        } else if (typeof event.data === 'string') {
          messages.value.push({ sender: 'unknown', body: event.data })
        }
      } catch {
        messages.value.push({ sender: 'unknown', body: event.data })
      }
    })

    const rosterUrl = `${opts.rosterBaseUrl || ''}/roster/${encodeURIComponent(opts.room)}`
    const poll = async () => {
      try {
        const res = await fetch(rosterUrl)
        if (res.ok) {
          const data = await res.json()
          roster.value = Array.isArray(data) ? data : []
        }
      } catch {
        // ignore poll errors
      }
    }

    poll()
    intervalId = setInterval(poll, 3000)
  }

  function send(text: string) {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(text)
      if (currentOpts) {
        messages.value.push({ sender: currentOpts.displayName, body: text })
      }
    }
  }

  function disconnect() {
    if (intervalId) {
      clearInterval(intervalId)
      intervalId = null
    }
    if (ws) {
      ws.close()
      ws = null
    }
    messages.value = []
    roster.value = []
    currentOpts = null
  }

  return { messages, roster, connect, send, disconnect }
}
