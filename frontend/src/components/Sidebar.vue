<script setup lang="ts">
import { ref } from 'vue'
import { state, actions } from '../store'
import LcarsModal from './LcarsModal.vue'

function initials(s: string) {
  return s.split(/\s+/).slice(0, 2).map((w) => w[0]?.toUpperCase() ?? '').join('')
}

const showAdd = ref(false)
const serverName = ref('')
const joinToken = ref('')
const joinErr = ref('')
const showRoom = ref(false)
const roomName = ref('')
const roomVis = ref<'public' | 'private'>('public')
const busy = ref(false)

async function createServer() {
  if (!serverName.value.trim()) return
  busy.value = true
  try { await actions.createServer(serverName.value.trim()); serverName.value = ''; showAdd.value = false }
  finally { busy.value = false }
}
async function joinServer() {
  const t = parseJoin(joinToken.value)
  if (!t) return
  joinErr.value = ''; busy.value = true
  try { await actions.joinByToken(t); joinToken.value = ''; showAdd.value = false }
  catch (e) { joinErr.value = (e as Error).message }
  finally { busy.value = false }
}
// Accept either a raw token/pin or a full invite URL (?join=<token>).
function parseJoin(v: string): string {
  v = v.trim()
  const m = v.match(/[?&]join=([^&\s]+)/)
  return m ? decodeURIComponent(m[1]) : v
}
async function createRoom() {
  if (!roomName.value.trim()) return
  busy.value = true
  try { await actions.createRoom(roomName.value.trim(), roomVis.value); roomName.value = ''; showRoom.value = false }
  finally { busy.value = false }
}

const showDM = ref(false)
const dmErr = ref('')
async function dmPersona(id: string) {
  dmErr.value = ''
  try { await actions.openDMWithPersona(id); showDM.value = false }
  catch (e) { dmErr.value = (e as Error).message }
}
async function dmUser(id: string) {
  dmErr.value = ''
  try { await actions.openDMWithUser(id); showDM.value = false }
  catch (e) { dmErr.value = (e as Error).message }
}
</script>

<template>
  <aside class="cm-sidebar" :class="{ open: state.ui.drawer }">
    <!-- servers -->
    <div class="cm-servers">
      <button
        v-for="s in state.servers" :key="s.id"
        class="cm-server" :class="{ active: s.id === state.currentServerId }"
        :title="s.name" @click="actions.selectServer(s.id)"
      >{{ initials(s.name) }}</button>
      <button class="cm-server add" title="Add a server" @click="showAdd = true">+</button>
    </div>

    <!-- channels -->
    <div class="cm-group">
      <span>Channels</span>
      <button title="Create channel" @click="showRoom = true">＋</button>
    </div>
    <button
      v-for="r in state.rooms" :key="r.id"
      class="cm-pill" :class="{ active: r.id === state.currentRoomId && state.ui.view === 'chat' }"
      @click="actions.selectRoom(r.id)"
    >
      <span class="ico">{{ r.visibility === 'private' ? '🔒' : '#' }}</span>
      <span style="overflow:hidden;text-overflow:ellipsis;white-space:nowrap">{{ r.name }}</span>
    </button>

    <!-- personas (click to DM a dm-enabled persona) -->
    <div class="cm-group"><span>Personas</span></div>
    <button
      v-for="p in state.personas" :key="p.id"
      class="cm-pill persona" :title="p.dmEnabled ? 'Direct message ' + p.displayName : p.displayName + ' (DMs off)'"
      @click="dmPersona(p.id)"
    >
      <span class="ico">◆</span>
      <span style="overflow:hidden;text-overflow:ellipsis;white-space:nowrap">{{ p.displayName }}</span>
    </button>
    <p v-if="!state.personas.length" class="cm-rempty">none yet — add in Settings</p>

    <!-- direct messages -->
    <div class="cm-group">
      <span>Direct Messages</span>
      <button title="New direct message" @click="showDM = true">＋</button>
    </div>
    <button
      v-for="d in state.dms" :key="d.id"
      class="cm-pill" :class="{ active: d.id === state.currentDMId && state.ui.view === 'chat' }"
      @click="actions.selectDM(d.id)"
    >
      <span class="ico">{{ d.otherKind === 'persona' ? '◆' : '@' }}</span>
      <span style="overflow:hidden;text-overflow:ellipsis;white-space:nowrap">{{ d.otherName }}</span>
    </button>
    <p v-if="dmErr" class="cm-rempty" style="color:var(--lcars-red)">{{ dmErr }}</p>

    <div class="cm-spacer" />

    <button class="cm-pill" :class="{ active: state.ui.view === 'settings' }" @click="actions.openSettings()">
      <span class="ico">⚙</span><span>Settings</span>
    </button>

    <div class="cm-user">
      <span class="cm-server" style="width:30px;height:30px;font-size:.75rem">{{ initials(state.me?.displayName ?? '?') }}</span>
      <span class="name">{{ state.me?.displayName }}<small :class="state.connected ? '' : ''">{{ state.connected ? 'online' : 'connecting…' }}</small></span>
      <button class="cm-link" @click="actions.logout()">Log out</button>
    </div>

    <!-- dialogs -->
    <LcarsModal v-if="showAdd" title="Add a Server" @close="showAdd = false">
      <label class="cm-label">Create a new server</label>
      <div class="cm-formrow">
        <input v-model="serverName" class="cm-field" placeholder="e.g. Bridge Crew" autofocus @keyup.enter="createServer" />
        <button class="cm-btn sm" :disabled="busy" @click="createServer">Create</button>
      </div>
      <hr style="border:none;border-top:1px solid #222;margin:18px 0" />
      <label class="cm-label">Join with an invite link or pin</label>
      <div class="cm-formrow">
        <input v-model="joinToken" class="cm-field" placeholder="paste invite link or pin" @keyup.enter="joinServer" />
        <button class="cm-btn sm alt" :disabled="busy" @click="joinServer">Join</button>
      </div>
      <p v-if="joinErr" class="cm-err" style="text-align:left">{{ joinErr }}</p>
    </LcarsModal>

    <LcarsModal v-if="showRoom" title="New Channel" @close="showRoom = false">
      <label class="cm-label">Channel name</label>
      <input v-model="roomName" class="cm-field" placeholder="e.g. main-game" autofocus @keyup.enter="createRoom" />
      <label class="cm-label" style="margin-top:14px">Visibility</label>
      <div class="cm-radio">
        <label :class="{ sel: roomVis === 'public' }" @click="roomVis = 'public'"># Public</label>
        <label :class="{ sel: roomVis === 'private' }" @click="roomVis = 'private'">🔒 Private</label>
      </div>
      <template #footer>
        <button class="cm-btn sm alt" @click="showRoom = false">Cancel</button>
        <button class="cm-btn sm" :disabled="busy" @click="createRoom">Create</button>
      </template>
    </LcarsModal>

    <LcarsModal v-if="showDM" title="New Direct Message" @close="showDM = false">
      <label class="cm-label">Message a persona</label>
      <button
        v-for="p in state.personas.filter((p) => p.dmEnabled)" :key="p.id"
        class="cm-pill" style="margin-bottom:4px" @click="dmPersona(p.id)"
      ><span class="ico">◆</span><span>{{ p.displayName }}</span></button>
      <p v-if="!state.personas.some((p) => p.dmEnabled)" class="cm-hint">No personas accept DMs yet — enable it in Settings.</p>

      <label class="cm-label" style="margin-top:14px">Message a person</label>
      <button
        v-for="m in state.registry.filter((m) => m.userId !== state.me?.id)" :key="m.userId"
        class="cm-pill" style="margin-bottom:4px" @click="dmUser(m.userId)"
      ><span class="ico">@</span><span>{{ m.displayName }}</span></button>
      <p v-if="state.registry.filter((m) => m.userId !== state.me?.id).length === 0" class="cm-hint">No other members in this server yet.</p>

      <p v-if="dmErr" class="cm-err" style="text-align:left">{{ dmErr }}</p>
    </LcarsModal>
  </aside>
</template>
