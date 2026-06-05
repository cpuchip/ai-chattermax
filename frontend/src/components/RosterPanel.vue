<script setup lang="ts">
import { computed } from 'vue'
import { state } from '../store'

const roster = computed(() => state.roster[state.currentRoomId] ?? [])
const humans = computed(() => roster.value.filter((p) => p.kind === 'human'))
const personas = computed(() => roster.value.filter((p) => p.kind === 'persona'))
</script>

<template>
  <aside class="flex w-56 flex-col bg-slate-800/60 px-3 py-4">
    <h3 class="mb-3 text-xs font-semibold uppercase tracking-wide text-slate-400">In this channel</h3>

    <div v-if="personas.length" class="mb-4">
      <div class="mb-1 text-[11px] font-semibold uppercase text-violet-400">Agents — {{ personas.length }}</div>
      <div v-for="p in personas" :key="p.id" class="flex items-center gap-2 py-1 text-sm text-slate-200">
        <span class="text-violet-400">◆</span>
        <span class="truncate">{{ p.name }}</span>
      </div>
    </div>

    <div>
      <div class="mb-1 text-[11px] font-semibold uppercase text-emerald-400">Online — {{ humans.length }}</div>
      <div v-for="p in humans" :key="p.id" class="flex items-center gap-2 py-1 text-sm text-slate-200">
        <span class="inline-block h-2 w-2 rounded-full bg-emerald-400"></span>
        <span class="truncate">{{ p.name }}</span>
      </div>
      <p v-if="!humans.length" class="text-xs text-slate-600">just you, so far</p>
    </div>
  </aside>
</template>
