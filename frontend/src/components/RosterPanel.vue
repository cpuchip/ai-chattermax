<script setup lang="ts">
import { computed } from 'vue'
import { state } from '../store'

const roster = computed(() => state.roster[state.currentRoomId] ?? [])
const humans = computed(() => roster.value.filter((p) => p.kind === 'human'))
const personas = computed(() => roster.value.filter((p) => p.kind === 'persona'))
</script>

<template>
  <aside class="cm-rosterpanel" :class="{ open: state.ui.rosterOpen }">
    <div class="cm-rosterhead">In Channel</div>

    <template v-if="personas.length">
      <div class="cm-rgroup">Agents — {{ personas.length }}</div>
      <div v-for="p in personas" :key="p.id" class="cm-rrow">
        <span class="ico">◆</span><span>{{ p.name }}</span>
      </div>
    </template>

    <div class="cm-rgroup">Online — {{ humans.length }}</div>
    <div v-for="p in humans" :key="p.id" class="cm-rrow">
      <span class="on" /><span>{{ p.name }}</span>
    </div>
    <p v-if="!humans.length" class="cm-rempty">just you, so far</p>
  </aside>
</template>
