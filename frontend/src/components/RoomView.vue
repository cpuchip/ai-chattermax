<script setup lang="ts">
import { ref, watch, nextTick } from 'vue'
import { useChat } from '../composables/useChat'
import RosterPanel from './RosterPanel.vue'

const props = defineProps<{
  displayName: string
  room: string
}>()

const emit = defineEmits<{
  leave: []
}>()

const text = ref('')
const transcriptRef = ref<HTMLDivElement | null>(null)

const { messages, roster, send, disconnect, connect } = useChat()

connect({
  displayName: props.displayName,
  room: props.room,
})

function handleSend() {
  const t = text.value.trim()
  if (!t) return
  send(t)
  text.value = ''
}

function handleLeave() {
  disconnect()
  emit('leave')
}

watch(
  () => messages.value.length,
  async () => {
    await nextTick()
    if (transcriptRef.value) {
      transcriptRef.value.scrollTop = transcriptRef.value.scrollHeight
    }
  }
)
</script>

<template>
  <div class="flex h-screen flex-col">
    <header class="flex items-center justify-between border-b border-gray-200 bg-white px-4 py-3">
      <div>
        <h2 class="text-lg font-semibold">{{ room }}</h2>
        <p class="text-sm text-gray-500">as {{ displayName }}</p>
      </div>
      <button
        @click="handleLeave"
        class="rounded-lg border border-gray-300 px-3 py-1.5 text-sm font-medium text-gray-700 hover:bg-gray-50"
      >
        Leave
      </button>
    </header>

    <div class="flex flex-1 overflow-hidden">
      <div class="flex flex-1 flex-col">
        <div ref="transcriptRef" class="flex-1 overflow-y-auto p-4 space-y-2">
          <div
            v-for="(msg, i) in messages"
            :key="i"
            :class="[
              'max-w-[80%] rounded-lg px-3 py-2 text-sm',
              msg.sender === displayName
                ? 'ml-auto bg-blue-600 text-white'
                : 'bg-gray-200 text-gray-900',
            ]"
          >
            <div v-if="msg.sender !== displayName" class="mb-0.5 text-xs font-semibold text-gray-600">
              {{ msg.sender }}
            </div>
            {{ msg.body }}
          </div>
        </div>

        <form
          @submit.prevent="handleSend"
          class="flex items-center gap-2 border-t border-gray-200 bg-white px-4 py-3"
        >
          <input
            v-model="text"
            type="text"
            placeholder="Type a message…"
            class="flex-1 rounded-lg border border-gray-300 px-3 py-2 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
          />
          <button
            type="submit"
            class="rounded-lg bg-blue-600 px-4 py-2 font-medium text-white hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500"
          >
            Send
          </button>
        </form>
      </div>

      <aside class="w-48 border-l border-gray-200 bg-gray-50 p-3">
        <RosterPanel :roster="roster" />
      </aside>
    </div>
  </div>
</template>
