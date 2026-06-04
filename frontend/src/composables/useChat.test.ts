import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { useChat } from './useChat'

describe('useChat', () => {
  let fakeWS: any
  let wsConstructor: any

  beforeEach(() => {
    fakeWS = {
      readyState: 1, // OPEN
      send: vi.fn(),
      close: vi.fn(),
      addEventListener: vi.fn(),
    }
    wsConstructor = vi.fn(() => fakeWS)
    ;(wsConstructor as any).OPEN = 1
    ;(wsConstructor as any).CLOSED = 3
    globalThis.WebSocket = wsConstructor as any
    globalThis.fetch = vi.fn()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('opens WebSocket to the correct URL', () => {
    const { connect } = useChat()
    connect({ displayName: 'alice', room: 'lobby', wsBaseUrl: 'ws://localhost:1234' })
    expect(wsConstructor).toHaveBeenCalledWith('ws://localhost:1234/ws/lobby?id=alice')
  })

  it('uses same-origin when wsBaseUrl is omitted', () => {
    const { connect } = useChat()
    connect({ displayName: 'bob', room: 'general' })
    expect(wsConstructor).toHaveBeenCalledWith('/ws/general?id=bob')
  })

  it('pushes incoming messages to messages ref', () => {
    const { connect, messages } = useChat()
    connect({ displayName: 'alice', room: 'lobby' })

    const handler = fakeWS.addEventListener.mock.calls.find(
      (call: any) => call[0] === 'message'
    )[1]

    handler({ data: '{"sender":"bob","body":"hello"}' })
    expect(messages.value).toEqual([{ sender: 'bob', body: 'hello' }])
  })

  it('pushes raw string data when JSON parse fails', () => {
    const { connect, messages } = useChat()
    connect({ displayName: 'alice', room: 'lobby' })

    const handler = fakeWS.addEventListener.mock.calls.find(
      (call: any) => call[0] === 'message'
    )[1]

    handler({ data: 'plain text' })
    expect(messages.value).toEqual([{ sender: 'unknown', body: 'plain text' }])
  })

  it('sends text over the socket and echoes locally', () => {
    const { connect, send, messages } = useChat()
    connect({ displayName: 'alice', room: 'lobby' })

    send('hi there')
    expect(fakeWS.send).toHaveBeenCalledWith('hi there')
    expect(messages.value).toEqual([{ sender: 'alice', body: 'hi there' }])
  })

  it('polls roster on connect and every ~3s', async () => {
    vi.useFakeTimers()
    const fetchMock = globalThis.fetch as ReturnType<typeof vi.fn>
    fetchMock.mockResolvedValue({ ok: true, json: async () => [{ id: 'alice' }] })

    const { connect, roster } = useChat()
    connect({ displayName: 'alice', room: 'lobby', rosterBaseUrl: 'http://localhost:5678' })

    await vi.waitFor(() => expect(fetchMock).toHaveBeenCalledWith('http://localhost:5678/roster/lobby'))
    await vi.waitFor(() => expect(roster.value).toEqual([{ id: 'alice' }]))

    fetchMock.mockClear()
    vi.advanceTimersByTime(3000)
    await vi.waitFor(() => expect(fetchMock).toHaveBeenCalledWith('http://localhost:5678/roster/lobby'))

    vi.useRealTimers()
  })

  it('cleans up websocket and interval on disconnect', () => {
    const { connect, disconnect } = useChat()
    connect({ displayName: 'alice', room: 'lobby' })
    disconnect()
    expect(fakeWS.close).toHaveBeenCalled()
  })
})
