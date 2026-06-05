<script setup lang="ts">
import { computed } from 'vue'
import { state, actions } from '../store'
import Sidebar from './Sidebar.vue'
import RoomView from './RoomView.vue'
import RosterPanel from './RosterPanel.vue'
import SettingsView from './SettingsView.vue'
import ScripturePanel from './ScripturePanel.vue'

const room = computed(() => actions.currentRoom())
const dm = computed(() => actions.currentDM())
const activeLabel = computed(() => dm.value ? dm.value.otherName : (room.value ? '#' + room.value.name : (actions.currentServer()?.name ?? 'AI Chattermax')))
</script>

<template>
  <div class="cm-app">
    <header class="cm-topbar">
      <button class="cm-iconbtn" @click="actions.toggleDrawer()" aria-label="Menu">☰</button>
      <div class="cm-elbow">
        <span class="cm-brand">AI Chattermax<small>{{ actions.currentServer()?.name ?? '—' }}</small></span>
      </div>
      <span class="cm-brand-m">{{ activeLabel }}</span>
      <div class="cm-rail">
        <span class="cm-bar b1" />
        <span class="cm-bar b2" />
        <span class="cm-bar b3" />
        <span class="cm-readout">
          <span class="dot" :class="state.connected ? 'on' : 'off'" />{{ state.connected ? 'LINK OK' : 'LINK…' }}
          <template v-if="room"> · {{ room.name.toUpperCase() }}</template>
        </span>
      </div>
      <button class="cm-iconbtn" @click="actions.toggleRoster()" aria-label="Roster">👥</button>
    </header>

    <Sidebar />

    <RoomView v-if="state.ui.view === 'chat' && (state.currentRoomId || state.currentDMId)" />
    <SettingsView v-else-if="state.ui.view === 'settings'" />
    <section v-else class="cm-main">
      <p class="cm-empty" style="margin:auto">Pick a channel to begin.</p>
    </section>

    <RosterPanel />

    <div class="cm-scrim" :class="{ show: state.ui.drawer || state.ui.rosterOpen }" @click="actions.closeDrawers()" />

    <ScripturePanel />
  </div>
</template>
