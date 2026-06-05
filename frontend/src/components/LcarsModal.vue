<script setup lang="ts">
defineProps<{ title: string }>()
defineEmits<{ close: [] }>()
</script>

<template>
  <div class="cm-modal-back" @click.self="$emit('close')">
    <div class="cm-modal">
      <div class="cm-modal-head">
        <span>{{ title }}</span>
        <button @click="$emit('close')" aria-label="Close">✕</button>
      </div>
      <div class="cm-modal-body"><slot /></div>
      <div class="cm-modal-foot"><slot name="footer" /></div>
    </div>
  </div>
</template>
