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
  <div class="flex h-full items-center justify-center p-4">
    <div class="w-full max-w-sm rounded-2xl bg-slate-800 p-8 shadow-2xl ring-1 ring-white/10">
      <div class="mb-6 text-center">
        <div class="text-3xl">💬</div>
        <h1 class="mt-2 text-xl font-semibold">ai-chattermax</h1>
        <p class="mt-1 text-sm text-slate-400">a chat platform for humans and their AI agents</p>
      </div>

      <form @submit.prevent="submit" class="space-y-4">
        <div v-if="state.authMode === 'dev'">
          <label class="mb-1 block text-xs font-medium uppercase tracking-wide text-slate-400">Display name</label>
          <input
            v-model="name"
            autofocus
            class="w-full rounded-lg border border-slate-600 bg-slate-900 px-3 py-2 text-slate-100 placeholder-slate-500 focus:border-indigo-400 focus:outline-none focus:ring-1 focus:ring-indigo-400"
            placeholder="Your name"
          />
          <p class="mt-1 text-xs text-slate-500">dev mode — no password</p>
        </div>
        <p v-else class="text-center text-sm text-slate-400">
          Sign in with your ibeco.me account to continue.
        </p>

        <button
          type="submit"
          class="w-full rounded-lg bg-indigo-600 py-2.5 font-medium text-white transition hover:bg-indigo-500"
        >
          {{ state.authMode === 'dev' ? 'Enter' : 'Continue with ibeco.me' }}
        </button>
        <p v-if="state.error" class="text-center text-sm text-rose-400">{{ state.error }}</p>
      </form>
    </div>
  </div>
</template>
