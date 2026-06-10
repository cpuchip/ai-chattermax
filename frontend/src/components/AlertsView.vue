<script setup lang="ts">
import { computed } from 'vue'
import { state, actions } from '../store'

const alerts = computed(() => state.notifications)
const hasUnread = computed(() => actions.unreadCount() > 0)
const timeOf = (ts: string) => new Date(ts).toLocaleString([], { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
</script>

<template>
  <section class="cm-main">
    <header class="cm-roomhead">
      <span class="h">🔔 Alerts</span>
      <span class="topic">mentions of you</span>
      <button v-if="hasUnread" class="cm-btn xs" style="margin-left:auto" @click="actions.markAllAlertsRead()">Mark all read</button>
    </header>

    <div class="cm-msgs">
      <p v-if="!alerts.length" class="cm-empty">No mentions yet. When someone @mentions you, it lands here.</p>
      <button v-for="n in alerts" :key="n.id" class="cm-alert" :class="{ unread: !n.readAt }"
              @click="actions.openNotification(n)">
        <span class="cm-alert-top">
          <span class="who">{{ n.from }}</span>
          <span class="where" v-if="n.roomName"># {{ n.roomName }}</span>
          <span class="when">{{ timeOf(n.createdAt) }}</span>
        </span>
        <span class="cm-alert-body">{{ n.snippet }}</span>
      </button>
    </div>
  </section>
</template>
