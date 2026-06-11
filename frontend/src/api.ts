// REST client for the platform API. All requests send the session cookie.

export interface User { id: string; displayName: string; avatarUrl?: string; mood?: string }
export interface Server { id: string; slug: string; name: string; ownerUserId: string; joinToken?: string }
export interface Room { id: string; serverId: string; slug: string; name: string; visibility: string; topic?: string }
export interface Persona { id: string; serverId: string; ownerUserId: string; slug: string; displayName: string; hostKind: string; hostRef?: string; status: string; dmEnabled: boolean; respondPolicy: string; avatarUrl?: string }
export interface Notification { id: string; kind: string; roomId: string; roomName?: string; messageId: string; from: string; snippet: string; createdAt: string; readAt?: string }
export interface Command { name: string; args?: string; help?: string }
export interface SubPersona { id: string; personaId: string; personaName?: string; roomId: string; displayName: string }
export interface InitiativeEntry { id: string; name: string; modifier: number; roll: number; total: number }
export interface InitiativeRound { id: string; roomId: string; round: number; currentEntryId?: string; starterId?: string; active: boolean; entries: InitiativeEntry[] | null }
export interface PersonaKey { id: string; label?: string; createdAt: string; lastUsedAt?: string; revokedAt?: string }
export interface Reaction { emoji: string; reactorId: string; reactor: string; reactorKind: string }
export interface Message { id: string; roomId?: string; dmId?: string; senderId: string; sender: string; senderKind: string; senderAvatar?: string; body: string; ts: string; reactions?: Reaction[] }
export interface Participant { id: string; name: string; kind: string; avatar?: string; mood?: string }
export interface RegistryMember { userId: string; displayName: string; avatarUrl?: string; role: string; personas: Persona[] }
export interface DMSummary { id: string; kind: string; otherId: string; otherName: string; otherKind: string }

// DH-4 — a dnd-tools character sheet (mirrors the service's JSON).
export interface DndAttack { name: string; ability?: string; proficient: boolean; magic_bonus?: number; damage: string; damage_type?: string; range?: string; notes?: string }
export interface DndSpell { name: string; level: number; key?: string; prepared?: boolean; notes?: string }
export interface DndItem { name: string; qty: number; notes?: string }
export interface DndCharacter {
  id: number; campaign: string; name: string; player: string; kind: string
  species: string; class: string; background: string; alignment: string
  level: number; xp: number
  abilities: Record<string, number>; skills: string[]; saves: string[]
  hp_max: number; hp_current: number; ac: number; speed: number
  inventory: DndItem[]; spell_slots: Record<string, number>
  attacks: DndAttack[]; spells: DndSpell[]; conditions: string[]
  features: string[]; notes: string
}

async function req<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(path, {
    method,
    credentials: 'include',
    headers: body ? { 'Content-Type': 'application/json' } : undefined,
    body: body ? JSON.stringify(body) : undefined,
  })
  if (!res.ok) {
    const msg = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(msg.error || `request failed (${res.status})`)
  }
  if (res.status === 204) return undefined as T
  return res.json() as Promise<T>
}

export const api = {
  config: () => req<{ authMode: string }>('GET', '/api/config'),
  me: () => req<User>('GET', '/api/me'),
  login: (name?: string) => req<User>('POST', '/api/auth/login', name ? { name } : {}),
  logout: () => req<void>('POST', '/api/auth/logout'),

  servers: () => req<Server[]>('GET', '/api/servers'),
  server: (id: string) => req<Server>('GET', `/api/servers/${id}`),
  createServer: (name: string) => req<Server>('POST', '/api/servers', { name }),
  joinServer: (token: string) => req<Server>('POST', '/api/servers/join', { token }),

  rooms: (serverId: string) => req<Room[]>('GET', `/api/servers/${serverId}/rooms`),
  createRoom: (serverId: string, name: string, visibility: string) =>
    req<Room>('POST', `/api/servers/${serverId}/rooms`, { name, visibility }),

  registry: (serverId: string) => req<RegistryMember[]>('GET', `/api/servers/${serverId}/registry`),
  personas: (serverId: string) => req<Persona[]>('GET', `/api/servers/${serverId}/personas`),
  createPersona: (serverId: string, displayName: string, hostRef: string) =>
    req<Persona>('POST', `/api/servers/${serverId}/personas`, { displayName, hostRef }),
  mintKey: (personaId: string, label?: string) =>
    req<{ key: string; personaId: string }>('POST', `/api/personas/${personaId}/keys`, { label }),
  grantPersona: (personaId: string, roomId: string) =>
    req<{ personaId: string; roomId: string }>('POST', `/api/personas/${personaId}/grants`, { roomId }),

  // AXR2 — persona management (owner or server admin).
  personaGrants: (personaId: string) => req<Room[]>('GET', `/api/personas/${personaId}/grants`),
  revokeGrant: (personaId: string, roomId: string) =>
    req<void>('DELETE', `/api/personas/${personaId}/grants/${roomId}`),
  personaKeys: (personaId: string) => req<PersonaKey[]>('GET', `/api/personas/${personaId}/keys`),
  revokeKey: (personaId: string, keyId: string) =>
    req<void>('DELETE', `/api/personas/${personaId}/keys/${keyId}`),
  setPersonaDM: (personaId: string, dmEnabled: boolean) =>
    req<Persona>('PATCH', `/api/personas/${personaId}`, { dmEnabled }),
  setPersonaRespondPolicy: (personaId: string, respondPolicy: string) =>
    req<Persona>('PATCH', `/api/personas/${personaId}`, { respondPolicy }),
  deletePersona: (personaId: string) => req<void>('DELETE', `/api/personas/${personaId}`),

  // DH-1 — slash command registry (drives the composer autocomplete).
  commands: () => req<Command[]>('GET', '/api/commands'),

  // REM-3 — mention notifications.
  notifications: () => req<Notification[]>('GET', '/api/notifications'),
  readNotifications: (ids?: string[]) =>
    req<void>('POST', '/api/notifications/read', ids?.length ? { ids } : {}),

  messages: (roomId: string) => req<Message[]>('GET', `/api/rooms/${roomId}/messages`),

  // AXR3 — direct messages.
  listDMs: () => req<DMSummary[]>('GET', '/api/dms'),
  openDMWithPersona: (personaId: string) =>
    req<DMSummary>('POST', '/api/dms', { kind: 'user_persona', personaId }),
  openDMWithUser: (serverId: string, userId: string) =>
    req<DMSummary>('POST', '/api/dms', { kind: 'user_user', serverId, userId }),
  dmMessages: (dmId: string) => req<Message[]>('GET', `/api/dms/${dmId}/messages`),
  deleteDM: (dmId: string) => req<void>('DELETE', `/api/dms/${dmId}`),

  // DH-4 — character sheets (proxied to the dnd-tools service; the API key
  // stays server-side).
  dndRoomCharacters: (roomId: string) =>
    req<{ campaign: string; characters: DndCharacter[] }>('GET', `/api/dnd/rooms/${roomId}/characters`),
  dndMyCharacter: (roomId: string) => req<DndCharacter>('GET', `/api/dnd/rooms/${roomId}/me`),
  dndCharacter: (roomId: string, name: string) =>
    req<DndCharacter>('GET', `/api/dnd/rooms/${roomId}/characters/${encodeURIComponent(name)}`),
  dndPatchCharacter: (roomId: string, name: string, patch: Partial<DndCharacter>) =>
    req<DndCharacter>('PATCH', `/api/dnd/rooms/${roomId}/characters/${encodeURIComponent(name)}`, patch),
}
