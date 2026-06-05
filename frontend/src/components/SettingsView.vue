<script setup lang="ts">
import { ref, reactive, computed } from 'vue'
import { state, actions } from '../store'
import { api } from '../api'

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
  try { await api.grantPersona(id, room); grantMsg[id] = 'granted to #' + (state.rooms.find(r => r.id === room)?.name ?? '') }
  catch (e) { grantMsg[id] = (e as Error).message }
}
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
          </div>
          <span v-else class="pmeta">owned by another crew member</span>
        </div>
        <p v-if="grantMsg[p.id]" class="cm-hint" style="color:var(--lcars-teal)">{{ grantMsg[p.id] }}</p>
        <div v-if="revealed[p.id]" class="cm-keybox">
          <div class="warn">⚠ Copy now — shown once</div>
          <code>{{ revealed[p.id] }}</code>
          <p class="cm-hint">Granted to a channel + key issued. Configure your host (CHATTERMAX_PERSONAS):</p>
          <code style="margin-top:4px">{{ p.hostRef }}={{ revealed[p.id] }}@{{ revealedRoom[p.id] }}</code>
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
