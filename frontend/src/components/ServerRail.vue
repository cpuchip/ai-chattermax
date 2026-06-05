<script setup lang="ts">
import { state, actions } from '../store'

function initials(name: string) {
  return name.split(/\s+/).slice(0, 2).map((w) => w[0]?.toUpperCase() ?? '').join('')
}
async function createServer() {
  const name = window.prompt('New server name')
  if (name && name.trim()) await actions.createServer(name.trim())
}
</script>

<template>
  <nav class="flex w-16 flex-col items-center gap-2 bg-slate-950 py-3">
    <button
      v-for="s in state.servers"
      :key="s.id"
      :title="s.name"
      @click="actions.selectServer(s.id)"
      class="flex h-11 w-11 items-center justify-center rounded-2xl text-sm font-semibold transition"
      :class="s.id === state.currentServerId
        ? 'bg-indigo-600 text-white rounded-xl'
        : 'bg-slate-800 text-slate-300 hover:bg-indigo-600 hover:text-white hover:rounded-xl'"
    >
      {{ initials(s.name) }}
    </button>
    <button
      @click="createServer"
      title="Create a server"
      class="flex h-11 w-11 items-center justify-center rounded-2xl bg-slate-800 text-2xl text-emerald-400 transition hover:rounded-xl hover:bg-emerald-600 hover:text-white"
    >+</button>
  </nav>
</template>
