<script setup lang="ts">
import { ref } from 'vue'

const emit = defineEmits<{
  join: [{ displayName: string; room: string }]
}>()

const displayName = ref('')
const room = ref('lobby')

function submit() {
  const name = displayName.value.trim()
  const roomName = room.value.trim() || 'lobby'
  if (!name) return
  emit('join', { displayName: name, room: roomName })
}
</script>

<template>
  <div class="flex min-h-screen items-center justify-center p-4">
    <div class="w-full max-w-sm rounded-xl bg-white p-6 shadow-lg">
      <h1 class="mb-4 text-center text-2xl font-bold">Join Chat</h1>
      <form @submit.prevent="submit" class="space-y-4">
        <div>
          <label class="mb-1 block text-sm font-medium">Display Name</label>
          <input
            v-model="displayName"
            type="text"
            required
            class="w-full rounded-lg border border-gray-300 px-3 py-2 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
            placeholder="Your name"
          />
        </div>
        <div>
          <label class="mb-1 block text-sm font-medium">Room</label>
          <input
            v-model="room"
            type="text"
            class="w-full rounded-lg border border-gray-300 px-3 py-2 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
            placeholder="lobby"
          />
        </div>
        <button
          type="submit"
          class="w-full rounded-lg bg-blue-600 py-2 font-medium text-white transition hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
        >
          Join
        </button>
      </form>
    </div>
  </div>
</template>
