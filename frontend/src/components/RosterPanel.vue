<script setup lang="ts">
import { computed } from 'vue'
import { state, actions } from '../store'

const roster = computed(() => state.roster[state.currentRoomId] ?? [])
const humans = computed(() => roster.value.filter((p) => p.kind === 'human'))
const personas = computed(() => roster.value.filter((p) => p.kind === 'persona'))

// One-click DM from the roster. Personas must have DMs enabled by their owner.
const dmEnabled = (personaId: string) =>
  state.personas.find((p) => p.id === personaId)?.dmEnabled ?? false
const isMe = (id: string) => state.me?.id === id

// Mood (REM-3): pick an emoji status shown next to your name everywhere.
const MOODS = ['😀', '🤔', '😎', '🔥', '😴', '🎲']
function pickMood(m: string) { actions.setMood(state.me?.mood === m ? '' : m) }
</script>

<template>
  <aside class="cm-rosterpanel" :class="{ open: state.ui.rosterOpen }">
    <div class="cm-rosterhead">In Channel</div>

    <template v-if="personas.length">
      <div class="cm-rgroup">Agents — {{ personas.length }}</div>
      <div v-for="p in personas" :key="p.id" class="cm-rrow">
        <span class="ico">◆</span><span>{{ p.name }}</span>
        <button v-if="dmEnabled(p.id)" class="cm-rdm" title="Message" @click="actions.openDMWithPersona(p.id)">✉</button>
      </div>
    </template>

    <div class="cm-rgroup">Online — {{ humans.length }}</div>
    <div v-for="p in humans" :key="p.id" class="cm-rrow">
      <span class="on" /><span>{{ p.name }}</span>
      <span v-if="p.mood" class="cm-rmood">{{ p.mood }}</span>
      <button v-if="!isMe(p.id)" class="cm-rdm" title="Message" @click="actions.openDMWithUser(p.id)">✉</button>
    </div>
    <p v-if="!humans.length" class="cm-rempty">just you, so far</p>

    <div class="cm-rgroup" style="margin-top:auto">Your mood</div>
    <div class="cm-moodpick">
      <button v-for="m in MOODS" :key="m" class="cm-mood-opt" :class="{ on: state.me?.mood === m }"
              @click="pickMood(m)">{{ m }}</button>
    </div>
  </aside>
</template>
