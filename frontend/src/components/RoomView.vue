<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'
import { state, actions } from '../store'

const text = ref('')
const scroller = ref<HTMLDivElement | null>(null)
const room = computed(() => actions.currentRoom())
const messages = computed(() => state.messages[state.currentRoomId] ?? [])

const isMine = (id: string) => state.me && id === state.me.id
const timeOf = (ts: string) => new Date(ts).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
const initial = (s: string) => s[0]?.toUpperCase() ?? '?'

function submit() {
  if (!text.value.trim()) return
  actions.send(text.value)
  text.value = ''
}
watch(() => messages.value.length, async () => {
  await nextTick()
  if (scroller.value) scroller.value.scrollTop = scroller.value.scrollHeight
})
</script>

<template>
  <section class="cm-main">
    <header class="cm-roomhead">
      <span class="h">{{ room?.visibility === 'private' ? '🔒' : '#' }} {{ room?.name }}</span>
      <span v-if="room?.topic" class="topic">{{ room.topic }}</span>
    </header>

    <div ref="scroller" class="cm-msgs">
      <p v-if="!messages.length" class="cm-empty">No transmissions yet. Say hello.</p>
      <div v-for="m in messages" :key="m.id" class="cm-msg" :class="{ mine: isMine(m.senderId) }">
        <div class="cm-ava" :class="{ persona: m.senderKind === 'persona' }">
          {{ m.senderKind === 'persona' ? '◆' : initial(m.sender) }}
        </div>
        <div class="cm-body">
          <div class="cm-meta">
            <span class="cm-who" :class="{ persona: m.senderKind === 'persona' }">{{ m.sender }}</span>
            <span v-if="m.senderKind === 'persona'" class="cm-badge">agent</span>
            <span class="cm-time">{{ timeOf(m.ts) }}</span>
          </div>
          <div class="cm-bubble" :class="{ persona: m.senderKind === 'persona' && !isMine(m.senderId) }">{{ m.body }}</div>
        </div>
      </div>
    </div>

    <form class="cm-composer" @submit.prevent="submit">
      <input v-model="text" class="cm-input" :placeholder="`Message #${room?.name ?? ''}`" />
      <button type="submit" class="cm-send">Send</button>
    </form>
  </section>
</template>
