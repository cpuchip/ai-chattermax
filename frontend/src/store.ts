// The app store: a reactive singleton wiring the REST api + the gateway client
// into view state. Components read `state` and call `actions`.
import { reactive } from 'vue'
import { api, type User, type Server, type Room, type Persona, type Message, type Participant, type RegistryMember, type DMSummary } from './api'
import { Gateway } from './gateway'

interface State {
  loading: boolean
  authMode: string
  me: User | null
  connected: boolean
  servers: Server[]
  currentServerId: string
  currentServerToken: string
  rooms: Room[]
  currentRoomId: string
  dms: DMSummary[]
  currentDMId: string
  personas: Persona[]
  registry: RegistryMember[]
  messages: Record<string, Message[]>
  roster: Record<string, Participant[]>
  typing: Record<string, Record<string, number>> // channel → who → expiry ms
  error: string
  ui: { drawer: boolean; rosterOpen: boolean; view: 'chat' | 'settings' }
}

export const state = reactive<State>({
  loading: true,
  authMode: 'dev',
  me: null,
  connected: false,
  servers: [],
  currentServerId: '',
  currentServerToken: '',
  rooms: [],
  currentRoomId: '',
  dms: [],
  currentDMId: '',
  personas: [],
  registry: [],
  messages: {},
  roster: {},
  typing: {},
  error: '',
  ui: { drawer: false, rosterOpen: false, view: 'chat' },
})

let gateway: Gateway | null = null

function ensureGateway() {
  if (gateway) return
  gateway = new Gateway({
    onStatus: (c) => { state.connected = c },
    onHistory: (ch, msgs) => { state.messages[ch] = msgs },
    onMessage: (ch, msg) => {
      (state.messages[ch] ||= []).push(msg)
      // A real message from a sender clears their "typing…" immediately.
      if (state.typing[ch]) delete state.typing[ch][msg.sender]
    },
    // "<persona> is typing…" — refreshed every ~3s by the persona-host; we keep
    // a short expiry so it clears on its own if the turn ends without a message.
    onTyping: (ch, who) => { (state.typing[ch] ||= {})[who] = Date.now() + 6000 },
    onReaction: (ch, messageId, emoji, op, who) => {
      const msg = state.messages[ch]?.find((m) => m.id === messageId)
      if (!msg) return
      const rs = msg.reactions ||= []
      const i = rs.findIndex((r) => r.emoji === emoji && r.reactorId === who.id)
      if (op === 'add' && i === -1) rs.push({ emoji, reactorId: who.id, reactor: who.name, reactorKind: who.kind })
      if (op === 'remove' && i !== -1) rs.splice(i, 1)
    },
    onPresenceSnapshot: (ch, roster) => { state.roster[ch] = roster },
    onPresence: (ch, st, who) => {
      const list = state.roster[ch] ||= []
      if (st === 'join') { if (!list.some((p) => p.id === who.id)) list.push(who) }
      else state.roster[ch] = list.filter((p) => p.id !== who.id)
    },
  })
  gateway.connect()
}

export const actions = {
  async init() {
    state.loading = true
    try {
      state.authMode = (await api.config()).authMode
    } catch { /* ignore */ }
    try {
      state.me = await api.me()
    } catch {
      state.me = null
    }
    if (state.me) await this.afterAuth()
    state.loading = false
  },

  async login(name?: string) {
    state.error = ''
    try {
      state.me = await api.login(name)
      await this.afterAuth()
    } catch (e) {
      state.error = (e as Error).message
    }
  },

  async logout() {
    try { await api.logout() } catch { /* ignore */ }
    gateway?.close(); gateway = null
    state.me = null
    state.servers = []; state.rooms = []; state.personas = []; state.registry = []; state.dms = []
    state.currentServerId = ''; state.currentRoomId = ''; state.currentDMId = ''
    state.messages = {}; state.roster = {}
  },

  async afterAuth() {
    ensureGateway()
    state.servers = await api.servers()
    // Invite link: chat.ibeco.me/?join=<token> auto-joins that server.
    const joinTok = new URLSearchParams(location.search).get('join')
    if (joinTok) {
      try { await this.joinByToken(joinTok) } catch { /* invalid/expired */ }
      history.replaceState({}, '', location.pathname)
      if (state.currentServerId) return
    }
    // Sticky server: restore the last server selected on this device, if you're
    // still a member; otherwise fall back to the first server.
    const remembered = localStorage.getItem('cm.lastServerId')
    const pick = (remembered && state.servers.some(s => s.id === remembered))
      ? remembered
      : state.servers[0]?.id
    if (pick) await this.selectServer(pick)
  },

  async selectServer(id: string) {
    state.currentServerId = id
    try { localStorage.setItem('cm.lastServerId', id) } catch { /* private mode */ }
    state.currentServerToken = ''
    state.currentDMId = ''
    state.rooms = await api.rooms(id)
    try { state.personas = await api.personas(id) } catch { state.personas = [] }
    try { state.registry = await api.registry(id) } catch { state.registry = [] }
    try { state.dms = await api.listDMs() } catch { state.dms = [] }
    try { state.currentServerToken = (await api.server(id)).joinToken ?? '' } catch { /* non-admin */ }
    if (state.rooms.length) this.selectRoom(state.rooms[0].id)
    else state.currentRoomId = ''
  },

  async joinByToken(token: string) {
    const sv = await api.joinServer(token.trim())
    state.servers = await api.servers()
    await this.selectServer(sv.id)
    return sv
  },

  selectRoom(id: string) {
    state.currentRoomId = id
    state.currentDMId = ''
    gateway?.subscribe(id) // idempotent server-side; ensures presence + history
    state.ui.view = 'chat'
    state.ui.drawer = false
  },

  selectDM(id: string) {
    state.currentDMId = id
    gateway?.subscribe(id)
    state.ui.view = 'chat'
    state.ui.drawer = false
  },

  async openDMWithPersona(personaId: string) {
    const dm = await api.openDMWithPersona(personaId)
    if (!state.dms.some((d) => d.id === dm.id)) state.dms.unshift(dm)
    this.selectDM(dm.id)
  },

  async openDMWithUser(userId: string) {
    const dm = await api.openDMWithUser(state.currentServerId, userId)
    if (!state.dms.some((d) => d.id === dm.id)) state.dms.unshift(dm)
    this.selectDM(dm.id)
  },

  activeChannelId(): string { return state.currentDMId || state.currentRoomId },
  currentDM(): DMSummary | undefined { return state.dms.find((d) => d.id === state.currentDMId) },

  async deletePersona(id: string) {
    await api.deletePersona(id)
    state.personas = state.personas.filter((p) => p.id !== id)
    try { state.registry = await api.registry(state.currentServerId) } catch { /* ignore */ }
  },

  async deleteDM(id: string) {
    await api.deleteDM(id)
    state.dms = state.dms.filter((d) => d.id !== id)
    delete state.messages[id]
    if (state.currentDMId === id) {
      state.currentDMId = ''
      if (state.rooms.length) this.selectRoom(state.rooms[0].id)
    }
  },

  toggleDrawer() { state.ui.drawer = !state.ui.drawer; if (state.ui.drawer) state.ui.rosterOpen = false },
  toggleRoster() { state.ui.rosterOpen = !state.ui.rosterOpen; if (state.ui.rosterOpen) state.ui.drawer = false },
  closeDrawers() { state.ui.drawer = false; state.ui.rosterOpen = false },
  openSettings() { state.ui.view = 'settings'; state.ui.drawer = false },
  openChat() { state.ui.view = 'chat' },

  // Toggle my reaction on a message. The server broadcasts back to everyone
  // including me, and onReaction's idempotent patch applies it — but we also
  // patch optimistically so the chip responds instantly.
  toggleReaction(messageId: string, emoji: string) {
    const ch = state.currentDMId || state.currentRoomId
    if (!ch || !state.me || messageId.startsWith('local-')) return
    const msg = state.messages[ch]?.find((m) => m.id === messageId)
    if (!msg) return
    const rs = msg.reactions ||= []
    const i = rs.findIndex((r) => r.emoji === emoji && r.reactorId === state.me!.id)
    const op = i === -1 ? 'add' : 'remove'
    if (op === 'add') rs.push({ emoji, reactorId: state.me.id, reactor: state.me.displayName, reactorKind: 'human' })
    else rs.splice(i, 1)
    gateway?.sendReaction(ch, messageId, emoji, op)
  },

  send(body: string) {
    const ch = state.currentDMId || state.currentRoomId
    if (!ch || !body.trim() || !state.me) return
    // optimistic — the server broadcasts to everyone except us.
    ;(state.messages[ch] ||= []).push({
      id: 'local-' + Date.now(), senderId: state.me.id,
      sender: state.me.displayName, senderKind: 'human', body, ts: new Date().toISOString(),
    })
    gateway?.send(ch, body)
  },

  async createServer(name: string) {
    const sv = await api.createServer(name)
    state.servers.push(sv)
    await this.selectServer(sv.id)
  },

  async createRoom(name: string, visibility: string) {
    const r = await api.createRoom(state.currentServerId, name, visibility)
    state.rooms.push(r)
    this.selectRoom(r.id)
  },

  async createPersona(displayName: string, hostRef: string) {
    await api.createPersona(state.currentServerId, displayName, hostRef)
    state.personas = await api.personas(state.currentServerId)
    state.registry = await api.registry(state.currentServerId)
  },

  mintKey(personaId: string) {
    return api.mintKey(personaId)
  },

  currentRoom(): Room | undefined {
    return state.rooms.find((r) => r.id === state.currentRoomId)
  },
  currentServer(): Server | undefined {
    return state.servers.find((s) => s.id === state.currentServerId)
  },
}
