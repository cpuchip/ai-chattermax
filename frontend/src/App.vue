<script setup lang="ts">
import { onMounted } from 'vue'
import { state, actions } from './store'
import Login from './components/Login.vue'
import Shell from './components/Shell.vue'

onMounted(() => actions.init())
</script>

<template>
  <div v-if="state.loading" class="cm-login">
    <span style="color: var(--lcars-text-muted); font-family: var(--font-lcars); letter-spacing: 2px">INITIALIZING…</span>
  </div>
  <Login v-else-if="!state.me" />
  <Shell v-else />
</template>
