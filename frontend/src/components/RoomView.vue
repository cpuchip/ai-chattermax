<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'
import { state, actions } from '../store'
import { renderMarkdown } from '../lib/markdown'
import { useScripturePanel } from '../composables/useScripturePanel'

const { openDirect } = useScripturePanel()
// Intercept clicks on churchofjesuschrist.org links → open the in-app panel.
function onBodyClick(e: MouseEvent) {
  const a = (e.target as HTMLElement).closest('a.cjc-link') as HTMLAnchorElement | null
  if (!a) return
  e.preventDefault()
  openDirect({ reference: a.textContent?.trim() || 'Scripture', url: a.href })
}

const text = ref('')
const scroller = ref<HTMLDivElement | null>(null)
const dm = computed(() => actions.currentDM())
const room = computed(() => actions.currentRoom())
const messages = computed(() => state.messages[state.currentDMId || state.currentRoomId] ?? [])
const placeholder = computed(() => dm.value ? `Message ${dm.value.otherName}` : `Message #${room.value?.name ?? ''}`)

const isMine = (id: string) => state.me && id === state.me.id
const timeOf = (ts: string) => new Date(ts).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
const initial = (s: string) => s[0]?.toUpperCase() ?? '?'

function submit() {
  if (!text.value.trim()) return
  actions.send(text.value)
  text.value = ''
}
// Enter sends; Shift+Enter inserts a newline (the textarea's default, so we
// only intercept the bare Enter). Keeps the chat-native "type and hit enter"
// while allowing multi-line messages (code snippets, paragraphs).
function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter' && !e.shiftKey && !e.isComposing) {
    e.preventDefault()
    submit()
  }
}
async function deleteDM() {
  if (!dm.value) return
  if (!confirm(`Delete this conversation with ${dm.value.otherName}? Messages are removed for both of you.`)) return
  await actions.deleteDM(dm.value.id)
}
watch(() => messages.value.length, async () => {
  await nextTick()
  if (scroller.value) scroller.value.scrollTop = scroller.value.scrollHeight
})
</script>

<template>
  <section class="cm-main">
    <header class="cm-roomhead">
      <template v-if="dm">
        <span class="h">{{ dm.otherKind === 'persona' ? '◆' : '@' }} {{ dm.otherName }}</span>
        <span class="topic">direct message</span>
        <button class="cm-btn xs danger" style="margin-left:auto" @click="deleteDM">Delete</button>
      </template>
      <template v-else>
        <span class="h">{{ room?.visibility === 'private' ? '🔒' : '#' }} {{ room?.name }}</span>
        <span v-if="room?.topic" class="topic">{{ room.topic }}</span>
      </template>
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
          <div class="cm-bubble cm-md" :class="{ persona: m.senderKind === 'persona' && !isMine(m.senderId) }" @click="onBodyClick" v-html="renderMarkdown(m.body)" />
        </div>
      </div>
    </div>

    <form class="cm-composer" @submit.prevent="submit">
      <textarea v-model="text" class="cm-input" rows="1" :placeholder="placeholder"
                @keydown="onKeydown" />
      <button type="submit" class="cm-send">Send</button>
    </form>
  </section>
</template>
