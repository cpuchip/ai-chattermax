<script setup lang="ts">
interface Participant {
  id: string
  kind?: string
  online?: boolean
}

defineProps<{
  roster: Participant[]
}>()
</script>

<template>
  <div>
    <h3 class="mb-2 text-sm font-semibold uppercase tracking-wide text-gray-500">
      Participants
    </h3>
    <ul class="space-y-1">
      <li
        v-for="p in roster"
        :key="p.id"
        class="flex items-center gap-2 rounded-md px-2 py-1 text-sm"
      >
        <span
          class="h-2 w-2 rounded-full"
          :class="p.online !== false ? 'bg-green-500' : 'bg-gray-400'"
        />
        <span class="truncate">{{ p.id }}</span>
      </li>
    </ul>
    <p v-if="roster.length === 0" class="text-sm text-gray-400">No one here yet.</p>
  </div>
</template>
