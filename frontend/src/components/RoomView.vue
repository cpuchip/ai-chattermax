<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted, onUnmounted } from 'vue'
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

// Initiative strip (DH-1/D8): visible while a round runs in this room.
const initiative = computed(() => state.initiative[state.currentRoomId])
const currentTurnId = computed(() => initiative.value?.currentEntryId ?? '')
// The round's starter and server owner/admins get clickable controls — the
// buttons just send the same /init commands the server already gates.
const canRunInit = computed(() => {
  const r = initiative.value
  if (!r || !state.me) return false
  if (r.starterId === state.me.id) return true
  const me = state.registry.find((m) => m.userId === state.me!.id)
  return me?.role === 'owner' || me?.role === 'admin'
})
function initNext() { actions.send('/init next') }
function initEnd() { if (confirm('End this initiative round?')) actions.send('/init end') }

const isMine = (id: string) => state.me && id === state.me.id
const timeOf = (ts: string) => new Date(ts).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
const initial = (s: string) => s[0]?.toUpperCase() ?? '?'

// Reactions: hover a message → ☺+ → fixed palette; chips group by emoji.
// Optimistic own messages (id 'local-…') have no server id yet, so no reactions.
const PALETTE = ['👍', '❤️', '😂', '🎉', '👀', '🤔']
const pickerFor = ref('')
function togglePicker(messageId: string) { pickerFor.value = pickerFor.value === messageId ? '' : messageId }
function pick(messageId: string, emoji: string) {
  actions.toggleReaction(messageId, emoji)
  pickerFor.value = ''
}
function chips(m: { reactions?: { emoji: string; reactorId: string; reactor: string }[] }) {
  const by = new Map<string, { emoji: string; count: number; mine: boolean; names: string[] }>()
  for (const r of m.reactions ?? []) {
    const c = by.get(r.emoji) ?? { emoji: r.emoji, count: 0, mine: false, names: [] }
    c.count++
    c.names.push(r.reactor)
    if (state.me && r.reactorId === state.me.id) c.mine = true
    by.set(r.emoji, c)
  }
  return [...by.values()]
}
function onDocClick(e: MouseEvent) {
  if (!(e.target as HTMLElement).closest('.cm-reactwrap')) pickerFor.value = ''
}
onMounted(() => document.addEventListener('click', onDocClick))
onUnmounted(() => document.removeEventListener('click', onDocClick))

// "X is typing…" — a 1s tick makes expiry reactive; entries past their expiry
// drop out (the persona-host refreshes every 3s while a turn runs).
const nowTick = ref(Date.now())
let tickTimer: number | undefined
onMounted(() => { tickTimer = window.setInterval(() => { nowTick.value = Date.now() }, 1000) })
onUnmounted(() => { if (tickTimer) clearInterval(tickTimer) })
const typingNames = computed(() => {
  const ch = state.currentDMId || state.currentRoomId
  const m = state.typing[ch]
  if (!m) return [] as string[]
  return Object.entries(m).filter(([, exp]) => exp > nowTick.value).map(([who]) => who)
})
const typingLabel = computed(() => {
  const n = typingNames.value
  if (n.length === 0) return ''
  if (n.length === 1) return `${n[0]} is typing…`
  if (n.length === 2) return `${n[0]} and ${n[1]} are typing…`
  return `${n.length} people are typing…`
})

function submit() {
  if (!text.value.trim()) return
  actions.send(text.value)
  text.value = ''
  acToken.value = null
}

// Autocomplete (DH-1/D3): one popup, two triggers — "/" at the start of the
// message for commands (registry-driven), "@" anywhere for mentions (room
// roster + server members). ↑↓ select, Enter/Tab complete, Esc dismiss.
const taRef = ref<HTMLTextAreaElement | null>(null)
const acIndex = ref(0)
const acToken = ref<{ mode: 'cmd' | 'cmd-inline' | 'mention'; start: number; text: string } | null>(null)

function updateToken() {
  const el = taRef.value
  if (!el) { acToken.value = null; return }
  const pos = el.selectionStart ?? text.value.length
  const upto = text.value.slice(0, pos)
  if (/^\/[a-z0-9]*$/i.test(upto)) {
    acToken.value = { mode: 'cmd', start: 0, text: upto.slice(1) }
    acIndex.value = 0
    return
  }
  // Mid-message: only the inline-executable commands (/roll, /init) complete.
  const ws = Math.max(upto.lastIndexOf(' '), upto.lastIndexOf('\n'))
  const word = upto.slice(ws + 1)
  if (ws >= 0 && /^\/[a-z0-9]*$/i.test(word)) {
    acToken.value = { mode: 'cmd-inline', start: ws + 1, text: word.slice(1) }
    acIndex.value = 0
    return
  }
  const at = upto.lastIndexOf('@')
  if (at >= 0 && (at === 0 || /\s/.test(upto[at - 1])) && /^[\w-]*$/.test(upto.slice(at + 1))) {
    acToken.value = { mode: 'mention', start: at, text: upto.slice(at + 1) }
    acIndex.value = 0
    return
  }
  acToken.value = null
}

const acItems = computed(() => {
  const tk = acToken.value
  if (!tk) return []
  const q = tk.text.toLowerCase()
  if (tk.mode === 'cmd' || tk.mode === 'cmd-inline') {
    return state.commands
      .filter((c) => c.name.startsWith(q) && (tk.mode === 'cmd' || c.name === 'roll' || c.name === 'init'))
      .map((c) => ({ key: c.name, label: '/' + c.name, hint: c.args ?? '', help: c.help ?? '', insert: '/' + c.name + ' ' }))
  }
  const ch = state.currentDMId || state.currentRoomId
  const seen = new Set<string>()
  const people: { name: string; kind: string; hint: string }[] = []
  for (const p of state.roster[ch] ?? []) {
    if (!seen.has(p.name)) { seen.add(p.name); people.push({ name: p.name, kind: p.kind, hint: p.kind === 'persona' ? 'agent' : '' }) }
  }
  // Cast members (DH-2): @Grimble routes to the persona who voices him.
  for (const sp of state.cast[ch] ?? []) {
    if (!seen.has(sp.displayName)) { seen.add(sp.displayName); people.push({ name: sp.displayName, kind: 'cast', hint: '▹ ' + (sp.personaName ?? 'cast') }) }
  }
  for (const m of state.registry) {
    if (!seen.has(m.displayName)) { seen.add(m.displayName); people.push({ name: m.displayName, kind: 'human', hint: '' }) }
  }
  return people
    .filter((p) => p.name.toLowerCase().replace(/\s/g, '').startsWith(q))
    .slice(0, 8)
    .map((p) => ({ key: p.name, label: '@' + p.name, hint: p.hint, help: '', insert: '@' + p.name.replace(/\s/g, '') + ' ' }))
})

function acComplete(i = acIndex.value) {
  const tk = acToken.value
  const item = acItems.value[i]
  if (!tk || !item) return
  const el = taRef.value
  const pos = el?.selectionStart ?? text.value.length
  text.value = text.value.slice(0, tk.start) + item.insert + text.value.slice(pos)
  acToken.value = null
  const np = tk.start + item.insert.length
  nextTick(() => { el?.setSelectionRange(np, np); el?.focus() })
}

// Enter sends; Shift+Enter inserts a newline. While the autocomplete popup is
// open, Enter/Tab complete instead of sending.
function onKeydown(e: KeyboardEvent) {
  if (acToken.value && acItems.value.length) {
    if (e.key === 'ArrowDown') { e.preventDefault(); acIndex.value = (acIndex.value + 1) % acItems.value.length; return }
    if (e.key === 'ArrowUp') { e.preventDefault(); acIndex.value = (acIndex.value - 1 + acItems.value.length) % acItems.value.length; return }
    if (e.key === 'Enter' || e.key === 'Tab') { e.preventDefault(); acComplete(); return }
    if (e.key === 'Escape') { acToken.value = null; return }
  }
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

    <div v-if="initiative?.entries?.length" class="cm-initiative">
      <span class="cm-init-round">⚔️ ROUND {{ initiative.round }}</span>
      <span v-for="e in initiative.entries" :key="e.id" class="cm-init-entry" :class="{ now: e.id === currentTurnId }">
        {{ e.name }} <b>{{ e.total }}</b>
      </span>
      <span v-if="canRunInit" class="cm-init-ctl">
        <button class="cm-init-btn" title="Next turn (/init next)" @click="initNext">Next ▸</button>
        <button class="cm-init-btn danger" title="End initiative (/init end)" @click="initEnd">✕</button>
      </span>
    </div>

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
          <div v-if="chips(m).length || pickerFor === m.id" class="cm-reactions">
            <button v-for="c in chips(m)" :key="c.emoji" class="cm-chip" :class="{ mine: c.mine }"
                    :title="c.names.join(', ')" @click="actions.toggleReaction(m.id, c.emoji)">
              {{ c.emoji }} <span class="n">{{ c.count }}</span>
            </button>
          </div>
        </div>
        <div v-if="!m.id.startsWith('local-')" class="cm-reactwrap">
          <button class="cm-react-btn" title="Add reaction" @click.stop="togglePicker(m.id)">☺+</button>
          <div v-if="pickerFor === m.id" class="cm-react-pop">
            <button v-for="e in PALETTE" :key="e" class="cm-react-opt" @click.stop="pick(m.id, e)">{{ e }}</button>
          </div>
        </div>
      </div>
    </div>

    <div class="cm-typing" :class="{ on: !!typingLabel }">
      <span v-if="typingLabel"><span class="cm-typing-dots"><i></i><i></i><i></i></span>{{ typingLabel }}</span>
    </div>

    <form class="cm-composer" @submit.prevent="submit">
      <div v-if="acItems.length" class="cm-ac">
        <button v-for="(it, i) in acItems" :key="it.key" type="button" class="cm-ac-item" :class="{ sel: i === acIndex }"
                @mousedown.prevent="acComplete(i)">
          <span class="l">{{ it.label }}</span>
          <span v-if="it.hint" class="hint">{{ it.hint }}</span>
          <span v-if="it.help" class="help">{{ it.help }}</span>
        </button>
      </div>
      <textarea ref="taRef" v-model="text" class="cm-input" rows="1" :placeholder="placeholder"
                @keydown="onKeydown" @input="updateToken" @click="updateToken" @keyup.left="updateToken" @keyup.right="updateToken" />
      <button type="submit" class="cm-send">Send</button>
    </form>
  </section>
</template>
