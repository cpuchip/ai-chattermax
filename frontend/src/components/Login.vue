<script setup lang="ts">
import { ref } from 'vue'
import { state, actions } from '../store'

const name = ref('')
function submit() {
  if (state.authMode === 'dev' && !name.value.trim()) return
  actions.login(state.authMode === 'dev' ? name.value.trim() : undefined)
}
</script>

<template>
  <div class="cm-login">
    <div class="cm-login-card">
      <div class="cm-login-head">
        AI Chattermax
        <small>LCARS · humans + agents</small>
      </div>
      <form class="cm-login-body" @submit.prevent="submit">
        <template v-if="state.authMode === 'dev'">
          <label class="cm-label">Crew designation</label>
          <input v-model="name" class="cm-field" placeholder="Your name" autofocus />
          <p class="cm-hint">Dev mode — no password.</p>
        </template>
        <p v-else class="cm-hint" style="text-align:center;margin:8px 0 0">
          Sign in with your ibeco.me account to engage.
        </p>
        <button type="submit" class="cm-btn">
          {{ state.authMode === 'dev' ? 'Engage' : 'Continue with ibeco.me' }}
        </button>
        <p v-if="state.error" class="cm-err">{{ state.error }}</p>
      </form>
    </div>
  </div>
</template>
