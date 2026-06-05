<script setup lang="ts">
import { ref } from 'vue'
import { state, actions } from '../store'
import LcarsModal from './LcarsModal.vue'

function initials(s: string) {
  return s.split(/\s+/).slice(0, 2).map((w) => w[0]?.toUpperCase() ?? '').join('')
}

const showServer = ref(false)
const serverName = ref('')
const showRoom = ref(false)
const roomName = ref('')
const roomVis = ref<'public' | 'private'>('public')
const busy = ref(false)

async function createServer() {
  if (!serverName.value.trim()) return
  busy.value = true
  try { await actions.createServer(serverName.value.trim()); serverName.value = ''; showServer.value = false }
  finally { busy.value = false }
}
async function createRoom() {
  if (!roomName.value.trim()) return
  busy.value = true
  try { await actions.createRoom(roomName.value.trim(), roomVis.value); roomName.value = ''; showRoom.value = false }
  finally { busy.value = false }
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
      <button class="cm-server add" title="Create server" @click="showServer = true">+</button>
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

    <!-- personas -->
    <div class="cm-group"><span>Personas</span></div>
    <div v-for="p in state.personas" :key="p.id" class="cm-pill persona" style="cursor:default">
      <span class="ico">◆</span>
      <span style="overflow:hidden;text-overflow:ellipsis;white-space:nowrap">{{ p.displayName }}</span>
    </div>
    <p v-if="!state.personas.length" class="cm-rempty">none yet — add in Settings</p>

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
    <LcarsModal v-if="showServer" title="New Server" @close="showServer = false">
      <label class="cm-label">Server name</label>
      <input v-model="serverName" class="cm-field" placeholder="e.g. Bridge Crew" autofocus @keyup.enter="createServer" />
      <template #footer>
        <button class="cm-btn sm alt" @click="showServer = false">Cancel</button>
        <button class="cm-btn sm" :disabled="busy" @click="createServer">Create</button>
      </template>
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
  </aside>
</template>
