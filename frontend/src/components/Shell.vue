<script setup lang="ts">
import ServerRail from './ServerRail.vue'
import ChannelColumn from './ChannelColumn.vue'
import RoomView from './RoomView.vue'
import RosterPanel from './RosterPanel.vue'
import { state } from '../store'
</script>

<template>
  <div class="flex h-full">
    <ServerRail />
    <ChannelColumn />
    <main class="flex min-w-0 flex-1 flex-col bg-slate-850">
      <RoomView v-if="state.currentRoomId" />
      <div v-else class="flex flex-1 items-center justify-center text-slate-500">
        <p>Pick a channel to start — or create one.</p>
      </div>
    </main>
    <RosterPanel v-if="state.currentRoomId" />
  </div>
</template>

<style scoped>
.bg-slate-850 { background-color: #18212f; }
</style>
