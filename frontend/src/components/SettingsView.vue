<script setup lang="ts">
import { ref, reactive, computed } from 'vue'
import { state, actions } from '../store'
import { api, type Room, type PersonaKey, type Persona } from '../api'

const inviteLink = computed(() =>
  state.currentServerToken ? `${location.origin}/?join=${state.currentServerToken}` : '')
const copied = ref(false)
async function copyInvite() {
  try { await navigator.clipboard.writeText(inviteLink.value); copied.value = true; setTimeout(() => (copied.value = false), 1800) }
  catch { /* clipboard blocked */ }
}

const newName = ref('')
const newHostRef = ref('dm-assistant')
const busy = ref(false)
const revealed = reactive<Record<string, string>>({})     // personaId -> raw key
const revealedRoom = reactive<Record<string, string>>({}) // personaId -> roomId the key was minted for
const grantSel = reactive<Record<string, string>>({})     // personaId -> roomId
const grantMsg = reactive<Record<string, string>>({})

const mine = (ownerId: string) => state.me && ownerId === state.me.id

// DH-4: per-room D&D campaign binding (the feature switch). Admin actions
// mirror /dnd enable + /campaign; the binding lives in dnd-tools.
const campRoom = ref(state.currentRoomId || state.rooms[0]?.id || '')
const campaigns = ref<{ id: number; name: string; status: string; room_id: string }[]>([])
const campSel = ref('')
const campMsg = ref('')
const boundName = computed(() => state.dndCampaign[campRoom.value] || '')
async function loadCampaigns() {
  try { campaigns.value = await api.dndCampaigns() } catch { campaigns.value = [] }
}
loadCampaigns()
async function bindCampaign(name: string) {
  campMsg.value = ''
  try {
    await api.dndBindCampaign(campRoom.value, name)
    await actions.loadDndRoster(campRoom.value)
    await loadCampaigns()
    campMsg.value = name ? `bound ${name}` : 'unbound'
  } catch (e) {
    campMsg.value = e instanceof Error ? e.message : String(e)
  }
}

async function createPersona() {
  if (!newName.value.trim()) return
  busy.value = true
  try { await actions.createPersona(newName.value.trim(), newHostRef.value.trim()); newName.value = '' }
  finally { busy.value = false }
}
async function mint(id: string) {
  const room = grantSel[id] || state.rooms[0]?.id
  if (!room) { grantMsg[id] = 'create a channel first'; return }
  revealed[id] = ''
  // Mint = grant the persona into the chosen channel AND issue its key, so the
  // key is always usable (the two were separate steps and easy to miss).
  try { await api.grantPersona(id, room) } catch { /* may already be granted */ }
  const r = await actions.mintKey(id)
  revealed[id] = r.key
  revealedRoom[id] = room
}
async function grant(id: string) {
  const room = grantSel[id] || state.rooms[0]?.id
  if (!room) { grantMsg[id] = 'create a channel first'; return }
  try { await api.grantPersona(id, room); grantMsg[id] = 'granted to #' + (state.rooms.find(r => r.id === room)?.name ?? ''); if (manageOpen[id]) loadManage(id) }
  catch (e) { grantMsg[id] = (e as Error).message }
}

// --- AXR2: manage panel (grants, keys, DM) ---------------------------------
const manageOpen = reactive<Record<string, boolean>>({})
const grants = reactive<Record<string, Room[]>>({})
const keys = reactive<Record<string, PersonaKey[]>>({})

function toggleManage(id: string) {
  manageOpen[id] = !manageOpen[id]
  if (manageOpen[id] && !grants[id]) loadManage(id)
}
async function loadManage(id: string) {
  try { grants[id] = await api.personaGrants(id) } catch { grants[id] = [] }
  try { keys[id] = await api.personaKeys(id) } catch { keys[id] = [] }
}
async function revokeGrant(id: string, roomId: string, name: string) {
  if (!confirm(`Revoke #${name} from this persona? Its host will lose access to that channel.`)) return
  await api.revokeGrant(id, roomId); loadManage(id)
}
async function revokeKey(id: string, keyId: string) {
  if (!confirm('Revoke this key? Any host using it is disconnected immediately and cannot reconnect.')) return
  await api.revokeKey(id, keyId); loadManage(id)
}
async function toggleDM(p: Persona) {
  try { const np = await api.setPersonaDM(p.id, !p.dmEnabled); p.dmEnabled = np.dmEnabled }
  catch { /* ignore */ }
}
async function setPolicy(p: Persona, policy: string) {
  try { const np = await api.setPersonaRespondPolicy(p.id, policy); p.respondPolicy = np.respondPolicy }
  catch { /* ignore */ }
}
async function deletePersona(p: Persona) {
  if (!confirm(`Delete persona "${p.displayName}"? Its keys stop working and it leaves the server. Past messages are kept.`)) return
  await actions.deletePersona(p.id)
}
const fmtDate = (s?: string) => (s ? new Date(s).toLocaleString() : '')
</script>

<template>
  <section class="cm-settings">
    <div class="cm-settings-head">
      <span>Settings</span>
      <button class="cm-btn sm alt" @click="actions.openChat()">← Back to chat</button>
    </div>
    <div class="cm-settings-body">
      <div class="cm-sec">Account</div>
      <div class="cm-card">
        <div class="row">
          <div><span class="pname">{{ state.me?.displayName }}</span><div class="pmeta">{{ state.connected ? 'connected' : 'offline' }}</div></div>
          <button class="cm-btn sm alt" @click="actions.logout()">Log out</button>
        </div>
      </div>

      <div class="cm-sec">Server — {{ actions.currentServer()?.name }}</div>
      <div class="cm-card">
        <template v-if="inviteLink">
          <label class="cm-label">Invite link</label>
          <div class="cm-formrow">
            <input class="cm-field" :value="inviteLink" readonly @focus="($event.target as HTMLInputElement).select()" />
            <button class="cm-btn sm" @click="copyInvite">{{ copied ? 'Copied!' : 'Copy' }}</button>
          </div>
          <p class="cm-hint">Share this — anyone who opens it and signs in joins this server.</p>
        </template>
        <p v-else class="cm-hint">Only the owner or admins can see the invite link.</p>
        <label class="cm-label" style="margin-top:14px">Members ({{ state.registry.length }})</label>
        <div v-for="m in state.registry" :key="m.userId" class="cm-rrow" style="margin-bottom:4px">
          <span class="on" /><span style="flex:1">{{ m.displayName }}</span><span class="pmeta">{{ m.role }}</span>
        </div>
      </div>

      <div class="cm-sec">D&D Campaign</div>
      <div class="cm-card">
        <label class="cm-label">Room</label>
        <div class="cm-formrow">
          <select v-model="campRoom" class="cm-field" style="width:auto;padding:6px 8px" @change="campMsg = ''">
            <option v-for="r in state.rooms" :key="r.id" :value="r.id">#{{ r.name }}</option>
          </select>
          <span class="pmeta">{{ boundName ? `plays ${boundName}` : 'D&D off' }}</span>
        </div>
        <div class="cm-formrow" style="margin-top:8px">
          <select v-model="campSel" class="cm-field" style="width:auto;padding:6px 8px">
            <option value="">— pick a campaign —</option>
            <option v-for="c in campaigns" :key="c.id" :value="c.name">{{ c.name }}{{ c.room_id ? ' (bound elsewhere)' : '' }}</option>
          </select>
          <button class="cm-btn sm" :disabled="!campSel" @click="bindCampaign(campSel)">Bind</button>
          <button v-if="boundName" class="cm-btn sm alt" @click="bindCampaign('')">Unbind</button>
        </div>
        <p class="cm-hint">Binding a campaign turns on the table commands (/attack, /check, /char…) in that room — same as /dnd enable. Room admins only.</p>
        <p v-if="campMsg" class="cm-hint">{{ campMsg }}</p>
      </div>

      <div class="cm-sec">Personas in {{ actions.currentServer()?.name }}</div>
      <p class="cm-hint" style="margin-bottom:12px">
        A persona's mind is supplied by a host (pg-ai-stewards). Mint a key, give it to your host,
        and it joins the channels you grant.
      </p>

      <div v-for="p in state.personas" :key="p.id" class="cm-card">
        <div class="row">
          <div>
            <span class="pname">◆ {{ p.displayName }}</span>
            <div class="pmeta">host: {{ p.hostRef || '—' }} · {{ p.hostKind }}</div>
          </div>
          <div v-if="mine(p.ownerUserId)" style="display:flex;gap:8px;align-items:center;flex-wrap:wrap">
            <select v-model="grantSel[p.id]" class="cm-field" style="width:auto;padding:6px 8px">
              <option v-for="r in state.rooms" :key="r.id" :value="r.id">#{{ r.name }}</option>
            </select>
            <button class="cm-btn sm alt" @click="grant(p.id)">Grant</button>
            <button class="cm-btn sm" @click="mint(p.id)">Grant + mint key</button>
            <button class="cm-btn sm alt" @click="toggleManage(p.id)">{{ manageOpen[p.id] ? 'Close' : 'Manage' }}</button>
          </div>
          <span v-else class="pmeta">owned by another crew member</span>
        </div>
        <p v-if="grantMsg[p.id]" class="cm-hint" style="color:var(--lcars-teal)">{{ grantMsg[p.id] }}</p>
        <div v-if="revealed[p.id]" class="cm-keybox">
          <div class="warn">⚠ Copy now — shown once</div>
          <code>{{ revealed[p.id] }}</code>
          <p class="cm-hint">Key issued + granted. Give it to your host (it discovers granted channels itself — see <code>examples/</code>):</p>
          <code style="margin-top:4px">CHATTERMAX_PERSONAS={{ p.hostRef || 'my-bot' }}={{ revealed[p.id] }}</code>
        </div>

        <!-- AXR2: manage grants, keys, DM (owner/admin) -->
        <div v-if="mine(p.ownerUserId) && manageOpen[p.id]" class="cm-manage">
          <label class="cm-check">
            <input type="checkbox" :checked="p.dmEnabled" @change="toggleDM(p)" />
            Allow direct messages to this persona
          </label>

          <label class="cm-label" style="margin-top:12px">Responds to</label>
          <select class="cm-select" :value="p.respondPolicy || 'all'" @change="setPolicy(p, ($event.target as HTMLSelectElement).value)">
            <option value="all">Every message (may stay silent)</option>
            <option value="mentioned">Only when mentioned by name</option>
            <option value="judgment">Its own judgment (licensed to chime in)</option>
          </select>

          <label class="cm-label" style="margin-top:12px">Granted channels</label>
          <div v-if="!grants[p.id]?.length" class="cm-hint">Not in any channel yet.</div>
          <div v-for="r in grants[p.id]" :key="r.id" class="cm-mrow">
            <span style="flex:1">#{{ r.name }}</span>
            <button class="cm-btn xs danger" @click="revokeGrant(p.id, r.id, r.name)">Revoke</button>
          </div>

          <label class="cm-label" style="margin-top:14px">Keys</label>
          <div v-if="!keys[p.id]?.length" class="cm-hint">No keys minted.</div>
          <div v-for="k in keys[p.id]" :key="k.id" class="cm-mrow">
            <div style="flex:1;min-width:0">
              <span>{{ k.label || 'key' }}</span>
              <span class="pmeta">
                {{ k.revokedAt ? 'revoked ' + fmtDate(k.revokedAt) : (k.lastUsedAt ? 'last used ' + fmtDate(k.lastUsedAt) : 'never used') }}
              </span>
            </div>
            <button v-if="!k.revokedAt" class="cm-btn xs danger" @click="revokeKey(p.id, k.id)">Revoke</button>
            <span v-else class="pmeta">revoked</span>
          </div>

          <div style="margin-top:14px;display:flex;justify-content:flex-end">
            <button class="cm-btn xs danger" @click="deletePersona(p)">Delete persona</button>
          </div>
        </div>
      </div>
      <p v-if="!state.personas.length" class="cm-hint">No personas yet.</p>

      <div class="cm-sec">New persona</div>
      <div class="cm-card">
        <div class="cm-formrow">
          <div>
            <label class="cm-label">Display name</label>
            <input v-model="newName" class="cm-field" placeholder="Gandalf" @keyup.enter="createPersona" />
          </div>
          <div>
            <label class="cm-label">Host ref</label>
            <input v-model="newHostRef" class="cm-field" placeholder="dm-assistant" />
          </div>
          <button class="cm-btn sm" :disabled="busy" @click="createPersona">Create</button>
        </div>
        <p class="cm-hint">Host ref = the pg-ai-stewards persona supplying this one's mind.</p>
      </div>
    </div>
  </section>
</template>
