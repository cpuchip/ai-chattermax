<script setup lang="ts">
import { ref } from 'vue'
import { state, actions } from '../store'
import { api } from '../api'

defineEmits<{ close: [] }>()

const newName = ref('')
const newHostRef = ref('dm-assistant')
const mintedKey = ref('')
const grantMsg = ref('')
const busy = ref(false)

async function create() {
  if (!newName.value.trim()) return
  busy.value = true
  try {
    await actions.createPersona(newName.value.trim(), newHostRef.value.trim())
    newName.value = ''
  } finally { busy.value = false }
}

async function mint(personaId: string) {
  mintedKey.value = ''
  const r = await actions.mintKey(personaId)
  mintedKey.value = r.key
}

async function grant(personaId: string) {
  grantMsg.value = ''
  if (!state.currentRoomId) { grantMsg.value = 'open a channel first'; return }
  await api.grantPersona(personaId, state.currentRoomId)
  grantMsg.value = `granted to #${actions.currentRoom()?.name}`
}

function mine(ownerUserId: string) {
  return state.me && ownerUserId === state.me.id
}
</script>

<template>
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4" @click.self="$emit('close')">
    <div class="w-full max-w-lg rounded-2xl bg-slate-800 p-6 shadow-2xl ring-1 ring-white/10">
      <div class="mb-4 flex items-center justify-between">
        <h2 class="text-lg font-semibold">Personas</h2>
        <button @click="$emit('close')" class="text-slate-400 hover:text-white">✕</button>
      </div>

      <!-- key reveal -->
      <div v-if="mintedKey" class="mb-4 rounded-lg bg-amber-500/10 p-3 ring-1 ring-amber-500/30">
        <p class="mb-1 text-xs font-semibold text-amber-300">Copy this key now — it is shown only once.</p>
        <code class="block break-all rounded bg-slate-950 px-2 py-1 text-xs text-amber-200">{{ mintedKey }}</code>
        <p class="mt-1 text-[11px] text-slate-400">Give it to your persona-host (pg-ai-stewards) so it can connect this persona.</p>
      </div>

      <!-- list -->
      <div class="mb-5 max-h-56 space-y-2 overflow-y-auto">
        <div v-for="p in state.personas" :key="p.id" class="flex items-center justify-between rounded-lg bg-slate-900/60 px-3 py-2">
          <div class="flex items-center gap-2">
            <span class="text-violet-400">◆</span>
            <div>
              <div class="text-sm">{{ p.displayName }}</div>
              <div class="text-[11px] text-slate-500">{{ p.hostRef || '—' }} · {{ p.hostKind }}</div>
            </div>
          </div>
          <div v-if="mine(p.ownerUserId)" class="flex gap-2">
            <button @click="grant(p.id)" class="rounded bg-slate-700 px-2 py-1 text-xs hover:bg-slate-600">Grant here</button>
            <button @click="mint(p.id)" class="rounded bg-indigo-600 px-2 py-1 text-xs hover:bg-indigo-500">Mint key</button>
          </div>
          <span v-else class="text-[11px] text-slate-600">owned by another</span>
        </div>
        <p v-if="!state.personas.length" class="text-sm text-slate-500">No personas yet.</p>
      </div>
      <p v-if="grantMsg" class="mb-3 text-xs text-emerald-400">{{ grantMsg }}</p>

      <!-- create -->
      <div class="border-t border-white/10 pt-4">
        <h3 class="mb-2 text-sm font-medium text-slate-300">New persona</h3>
        <div class="flex flex-wrap items-end gap-2">
          <div class="flex-1">
            <label class="mb-1 block text-[11px] uppercase text-slate-500">Display name</label>
            <input v-model="newName" class="w-full rounded border border-slate-600 bg-slate-900 px-2 py-1 text-sm focus:border-indigo-400 focus:outline-none" placeholder="Gandalf" />
          </div>
          <div>
            <label class="mb-1 block text-[11px] uppercase text-slate-500">Host ref</label>
            <input v-model="newHostRef" class="w-32 rounded border border-slate-600 bg-slate-900 px-2 py-1 text-sm focus:border-indigo-400 focus:outline-none" placeholder="dm-assistant" />
          </div>
          <button @click="create" :disabled="busy" class="rounded bg-emerald-600 px-3 py-1.5 text-sm font-medium hover:bg-emerald-500 disabled:opacity-50">Create</button>
        </div>
        <p class="mt-1 text-[11px] text-slate-500">Host ref = the pg-ai-stewards persona that supplies this one's mind.</p>
      </div>
    </div>
  </div>
</template>
