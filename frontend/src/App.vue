<script setup lang="ts">
import { ref } from 'vue'
import JoinScreen from './components/JoinScreen.vue'
import RoomView from './components/RoomView.vue'

interface JoinInfo {
  displayName: string
  room: string
}

const joined = ref<JoinInfo | null>(null)

function onJoin(info: JoinInfo) {
  joined.value = info
}

function onLeave() {
  joined.value = null
}
</script>

<template>
  <div class="min-h-screen bg-gray-50 text-gray-900">
    <JoinScreen v-if="!joined" @join="onJoin" />
    <RoomView
      v-else
      :display-name="joined.displayName"
      :room="joined.room"
      @leave="onLeave"
    />
  </div>
</template>
