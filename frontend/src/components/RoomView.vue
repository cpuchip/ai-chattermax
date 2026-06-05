<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'
import { state, actions } from '../store'

const text = ref('')
const scroller = ref<HTMLDivElement | null>(null)

const room = computed(() => actions.currentRoom())
const messages = computed(() => state.messages[state.currentRoomId] ?? [])

function isMine(senderId: string) {
  return state.me && senderId === state.me.id
}
function timeOf(ts: string) {
  return new Date(ts).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}
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
  <header class="flex h-12 items-center gap-2 border-b border-black/30 px-4 shadow-sm">
    <span class="text-slate-500">{{ room?.visibility === 'private' ? '🔒' : '#' }}</span>
    <span class="font-semibold">{{ room?.name }}</span>
    <span v-if="room?.topic" class="ml-2 truncate text-sm text-slate-400">{{ room.topic }}</span>
  </header>

  <div ref="scroller" class="flex-1 space-y-3 overflow-y-auto px-4 py-4">
    <p v-if="!messages.length" class="text-center text-sm text-slate-600">
      No messages yet. Say hello.
    </p>
    <div
      v-for="m in messages"
      :key="m.id"
      class="flex gap-3"
      :class="isMine(m.senderId) ? 'flex-row-reverse' : ''"
    >
      <div
        class="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-sm font-semibold"
        :class="m.senderKind === 'persona' ? 'bg-violet-600/30 text-violet-200' : 'bg-slate-600 text-slate-100'"
      >
        {{ m.senderKind === 'persona' ? '◆' : (m.sender[0]?.toUpperCase() ?? '?') }}
      </div>
      <div class="max-w-[70%]" :class="isMine(m.senderId) ? 'text-right' : ''">
        <div class="mb-0.5 flex items-center gap-2" :class="isMine(m.senderId) ? 'flex-row-reverse' : ''">
          <span class="text-sm font-medium" :class="m.senderKind === 'persona' ? 'text-violet-300' : 'text-slate-200'">{{ m.sender }}</span>
          <span v-if="m.senderKind === 'persona'" class="rounded bg-violet-600/30 px-1 text-[10px] font-semibold uppercase text-violet-300">agent</span>
          <span class="text-[10px] text-slate-500">{{ timeOf(m.ts) }}</span>
        </div>
        <div
          class="inline-block whitespace-pre-wrap rounded-2xl px-3 py-2 text-sm"
          :class="isMine(m.senderId) ? 'bg-indigo-600 text-white' : m.senderKind === 'persona' ? 'bg-violet-950/60 text-violet-50 ring-1 ring-violet-500/20' : 'bg-slate-700 text-slate-100'"
        >{{ m.body }}</div>
      </div>
    </div>
  </div>

  <form @submit.prevent="submit" class="flex items-center gap-2 border-t border-black/30 px-4 py-3">
    <input
      v-model="text"
      :placeholder="`Message #${room?.name ?? ''}`"
      class="flex-1 rounded-lg border border-slate-600 bg-slate-900 px-3 py-2 text-slate-100 placeholder-slate-500 focus:border-indigo-400 focus:outline-none focus:ring-1 focus:ring-indigo-400"
    />
    <button type="submit" class="rounded-lg bg-indigo-600 px-4 py-2 font-medium text-white transition hover:bg-indigo-500">Send</button>
  </form>
</template>
