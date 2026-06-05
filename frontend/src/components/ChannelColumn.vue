<script setup lang="ts">
import { ref } from 'vue'
import { state, actions } from '../store'
import MyPersonas from './MyPersonas.vue'

const showPersonas = ref(false)

async function createRoom() {
  const name = window.prompt('New channel name')
  if (!name || !name.trim()) return
  const priv = window.confirm('Make this channel private? (OK = private, Cancel = public)')
  await actions.createRoom(name.trim(), priv ? 'private' : 'public')
}
</script>

<template>
  <div class="flex w-60 flex-col bg-slate-800">
    <header class="flex h-12 items-center border-b border-black/30 px-4 font-semibold shadow-sm">
      {{ actions.currentServer()?.name ?? 'No server' }}
    </header>

    <div class="flex-1 overflow-y-auto px-2 py-3">
      <!-- Channels -->
      <div class="mb-1 flex items-center justify-between px-2">
        <span class="text-xs font-semibold uppercase tracking-wide text-slate-400">Channels</span>
        <button @click="createRoom" class="text-slate-400 hover:text-white" title="Create channel">＋</button>
      </div>
      <button
        v-for="r in state.rooms"
        :key="r.id"
        @click="actions.selectRoom(r.id)"
        class="flex w-full items-center gap-1 rounded px-2 py-1 text-left text-sm transition"
        :class="r.id === state.currentRoomId ? 'bg-slate-700 text-white' : 'text-slate-400 hover:bg-slate-700/50 hover:text-slate-200'"
      >
        <span class="text-slate-500">{{ r.visibility === 'private' ? '🔒' : '#' }}</span>
        <span class="truncate">{{ r.name }}</span>
      </button>

      <!-- DMs (Stage 4) -->
      <div class="mb-1 mt-5 px-2 text-xs font-semibold uppercase tracking-wide text-slate-400">Direct Messages</div>
      <p class="px-2 text-xs text-slate-600">coming soon</p>

      <!-- Personas -->
      <div class="mb-1 mt-5 flex items-center justify-between px-2">
        <span class="text-xs font-semibold uppercase tracking-wide text-slate-400">Personas</span>
        <button @click="showPersonas = true" class="text-slate-400 hover:text-white" title="Manage personas">⚙</button>
      </div>
      <div
        v-for="p in state.personas"
        :key="p.id"
        class="flex items-center gap-1 px-2 py-0.5 text-sm text-slate-400"
      >
        <span class="text-violet-400">◆</span>
        <span class="truncate">{{ p.displayName }}</span>
      </div>
    </div>

    <!-- User footer -->
    <footer class="flex items-center justify-between gap-2 border-t border-black/30 bg-slate-900/60 px-3 py-2">
      <div class="flex min-w-0 items-center gap-2">
        <span class="inline-block h-2 w-2 rounded-full" :class="state.connected ? 'bg-emerald-400' : 'bg-slate-500'" :title="state.connected ? 'connected' : 'offline'"></span>
        <span class="truncate text-sm">{{ state.me?.displayName }}</span>
      </div>
      <button @click="actions.logout()" class="text-xs text-slate-400 hover:text-white">Log out</button>
    </footer>

    <MyPersonas v-if="showPersonas" @close="showPersonas = false" />
  </div>
</template>
