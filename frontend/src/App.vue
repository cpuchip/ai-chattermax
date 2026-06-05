<script setup lang="ts">
import { onMounted } from 'vue'
import { state, actions } from './store'
import Login from './components/Login.vue'
import Shell from './components/Shell.vue'

onMounted(() => actions.init())
</script>

<template>
  <div class="h-screen w-screen overflow-hidden bg-slate-900 text-slate-100">
    <div v-if="state.loading" class="flex h-full items-center justify-center text-slate-400">
      <span class="animate-pulse">Loading…</span>
    </div>
    <Login v-else-if="!state.me" />
    <Shell v-else />
  </div>
</template>
