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
  error: '',
  ui: { drawer: false, rosterOpen: false, view: 'chat' },
})

let gateway: Gateway | null = null

function ensureGateway() {
  if (gateway) return
  gateway = new Gateway({
    onStatus: (c) => { state.connected = c },
    onHistory: (ch, msgs) => { state.messages[ch] = msgs },
    onMessage: (ch, msg) => { (state.messages[ch] ||= []).push(msg) },
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
    if (state.servers.length) await this.selectServer(state.servers[0].id)
  },

  async selectServer(id: string) {
    state.currentServerId = id
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

  toggleDrawer() { state.ui.drawer = !state.ui.drawer; if (state.ui.drawer) state.ui.rosterOpen = false },
  toggleRoster() { state.ui.rosterOpen = !state.ui.rosterOpen; if (state.ui.rosterOpen) state.ui.drawer = false },
  closeDrawers() { state.ui.drawer = false; state.ui.rosterOpen = false },
  openSettings() { state.ui.view = 'settings'; state.ui.drawer = false },
  openChat() { state.ui.view = 'chat' },

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
