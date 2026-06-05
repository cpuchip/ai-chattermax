<script setup lang="ts">
import { ref, reactive, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import { useScripturePanel } from '../composables/useScripturePanel'

// Borrowed from cpuchip.net (animejs trimmed). A movable LCARS panel that iframes
// a passage live from churchofjesuschrist.org — no scripture text is hosted here
// (copyright). Opens when a chat message's churchofjesuschrist.org link is clicked.
// Draggable by its header; multiple references stack as switchable tabs.

const { state, close, selectTab, closeTab, minimize, expand } = useScripturePanel()

const panel = ref<HTMLElement | null>(null)
const pos = ref<{ left: number; top: number } | null>(null)
const loaded = reactive<Record<number, boolean>>({})

const activeTab = computed(
  () => state.tabs.find((t) => t.id === state.activeId) ?? null,
)

watch(
  () => state.visible,
  (visible) => {
    if (!visible || pos.value) return
    pos.value = { left: Math.max(16, window.innerWidth - 480 - 36), top: 72 }
  },
)

// --- Drag by the header ---
let dragDX = 0
let dragDY = 0

function onDragMove(e: MouseEvent) {
  if (!pos.value) return
  pos.value = {
    left: Math.min(Math.max(8, e.clientX - dragDX), window.innerWidth - 90),
    top: Math.min(Math.max(8, e.clientY - dragDY), window.innerHeight - 60),
  }
}

function onDragEnd() {
  window.removeEventListener('mousemove', onDragMove)
  window.removeEventListener('mouseup', onDragEnd)
}

function startDrag(e: MouseEvent) {
  if (!pos.value) return
  dragDX = e.clientX - pos.value.left
  dragDY = e.clientY - pos.value.top
  window.addEventListener('mousemove', onDragMove)
  window.addEventListener('mouseup', onDragEnd)
}

function clampToViewport() {
  if (!pos.value) return
  pos.value = {
    left: Math.min(Math.max(8, pos.value.left), window.innerWidth - 90),
    top: Math.min(Math.max(8, pos.value.top), window.innerHeight - 60),
  }
}

onMounted(() => window.addEventListener('resize', clampToViewport))
onBeforeUnmount(() => {
  onDragEnd()
  window.removeEventListener('resize', clampToViewport)
})
</script>

<template>
  <!-- Pill: minimized, draggable, keeps the iframe + tabs alive. -->
  <div
    v-if="state.visible && state.minimized && activeTab"
    class="scripture-pill"
    :style="pos ? { left: pos.left + 'px', top: pos.top + 'px' } : undefined"
  >
    <div class="sp-pill-grip" @mousedown="startDrag" title="Drag to move"><span aria-hidden="true">≡</span></div>
    <button class="sp-pill-body" type="button" @click="expand" :title="`Expand — ${activeTab.reference}`">
      <span class="sp-pill-label">{{ activeTab.reference }}</span>
      <span v-if="state.tabs.length > 1" class="sp-pill-count">+{{ state.tabs.length - 1 }}</span>
    </button>
    <button class="sp-pill-close" type="button" @click="close" title="Close">✕</button>
  </div>

  <!-- Full panel. -->
  <div
    v-else-if="state.visible && activeTab"
    ref="panel"
    class="scripture-panel"
    :style="pos ? { left: pos.left + 'px', top: pos.top + 'px' } : undefined"
  >
    <div class="sp-header" @mousedown="startDrag">
      <span class="sp-ref">{{ activeTab.reference }}</span>
      <button class="sp-minimize" type="button" @click="minimize" title="Minimize to pill">–</button>
      <button class="sp-close" type="button" @click="close" aria-label="Close panel">✕</button>
    </div>

    <div v-if="state.tabs.length > 1" class="sp-tabs">
      <div
        v-for="tab in state.tabs" :key="tab.id"
        class="sp-tab" :class="{ active: tab.id === state.activeId }"
        @click="selectTab(tab.id)"
      >
        <span class="sp-tab-label">{{ tab.reference }}</span>
        <button class="sp-tab-x" type="button" :aria-label="`Close ${tab.reference}`" @click.stop="closeTab(tab.id)">✕</button>
      </div>
    </div>

    <div class="sp-body">
      <div v-for="tab in state.tabs" :key="tab.id" v-show="tab.id === state.activeId" class="sp-frame">
        <div v-if="!loaded[tab.id]" class="sp-loading">Loading from churchofjesuschrist.org…</div>
        <iframe
          :src="tab.url" class="sp-iframe" :class="{ 'sp-iframe-hidden': !loaded[tab.id] }"
          sandbox="allow-same-origin allow-scripts allow-popups" referrerpolicy="no-referrer"
          :title="tab.reference" @load="loaded[tab.id] = true"
        />
      </div>
    </div>

    <a class="sp-link" :href="activeTab.url" target="_blank" rel="noopener noreferrer">Open on churchofjesuschrist.org ↗</a>
  </div>
</template>

<style scoped>
.scripture-panel {
  position: fixed; z-index: 60; width: 480px; max-width: calc(100vw - 24px);
  height: 600px; max-height: 84vh; display: flex; flex-direction: column;
  background: #0d0d0d; border: 1px solid rgba(204, 153, 204, 0.4);
  border-radius: 4px 4px 4px 14px; box-shadow: 0 14px 44px rgba(0, 0, 0, 0.66); overflow: hidden;
}
.sp-header {
  display: flex; align-items: center; justify-content: space-between; gap: 8px;
  padding: 8px 10px 8px 14px; background: var(--lcars-lavender); cursor: grab; user-select: none; flex: none;
}
.sp-header:active { cursor: grabbing; }
.sp-ref {
  font-family: var(--font-lcars); font-size: 1rem; font-weight: 600; color: var(--lcars-text-dark);
  text-transform: uppercase; letter-spacing: 1px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; flex: 1;
}
.sp-minimize, .sp-close {
  border: none; background: rgba(0, 0, 0, 0.25); color: var(--lcars-text-dark); width: 22px; height: 22px;
  border-radius: 50%; cursor: pointer; font-size: 0.7rem; line-height: 1; flex: none;
}
.sp-minimize { font-size: 1rem; font-weight: 700; }
.sp-minimize:hover { background: var(--lcars-sky); }
.sp-close:hover { background: var(--lcars-orange); }
.scripture-pill {
  position: fixed; z-index: 60; display: flex; align-items: stretch; height: 36px; background: #0d0d0d;
  border: 1px solid rgba(204, 153, 204, 0.4); border-radius: 18px 18px 18px 4px;
  box-shadow: 0 8px 26px rgba(0, 0, 0, 0.55); overflow: hidden; font-family: var(--font-lcars); user-select: none;
}
.sp-pill-grip {
  display: flex; align-items: center; justify-content: center; width: 26px; background: var(--lcars-lavender);
  color: var(--lcars-text-dark); cursor: grab; font-size: 1.1rem; line-height: 1;
}
.sp-pill-grip:active { cursor: grabbing; }
.sp-pill-body {
  display: flex; align-items: center; gap: 7px; padding: 0 14px; border: none; background: transparent;
  color: var(--lcars-text); font: inherit; font-size: 0.78rem; text-transform: uppercase; letter-spacing: 1px;
  cursor: pointer; max-width: 220px;
}
.sp-pill-body:hover { background: rgba(204, 153, 204, 0.14); color: var(--lcars-gold); }
.sp-pill-label { white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.sp-pill-count {
  flex: none; padding: 2px 7px; border-radius: 9px; background: var(--lcars-orange);
  color: var(--lcars-text-dark); font-size: 0.66rem; letter-spacing: 0.5px;
}
.sp-pill-close {
  border: none; background: transparent; color: var(--lcars-text-muted); width: 32px; cursor: pointer;
  font-size: 0.78rem; line-height: 1; border-left: 1px solid rgba(204, 153, 204, 0.18);
}
.sp-pill-close:hover { background: var(--lcars-orange); color: var(--lcars-text-dark); }
.sp-tabs {
  display: flex; gap: 3px; padding: 5px 6px; background: #161616;
  border-bottom: 1px solid rgba(204, 153, 204, 0.22); overflow-x: auto; flex: none;
}
.sp-tab {
  display: flex; align-items: center; gap: 6px; padding: 4px 6px 4px 9px; border-radius: 3px 3px 3px 9px;
  background: #262626; color: var(--lcars-text-muted); font-family: var(--font-lcars); font-size: 0.72rem;
  text-transform: uppercase; letter-spacing: 0.5px; white-space: nowrap; cursor: pointer; flex: none; max-width: 168px;
}
.sp-tab:hover { background: #333; color: var(--lcars-read); }
.sp-tab.active { background: var(--lcars-lavender); color: var(--lcars-text-dark); }
.sp-tab-label { overflow: hidden; text-overflow: ellipsis; }
.sp-tab-x {
  border: none; background: rgba(0, 0, 0, 0.22); color: inherit; width: 15px; height: 15px; border-radius: 50%;
  cursor: pointer; font-size: 0.58rem; line-height: 1; padding: 0; flex: none;
}
.sp-tab-x:hover { background: var(--lcars-orange); color: var(--lcars-text-dark); }
.sp-body { flex: 1; min-height: 0; position: relative; background: #fff; }
.sp-frame { position: absolute; inset: 0; }
.sp-loading {
  position: absolute; inset: 0; display: flex; align-items: center; justify-content: center; background: #0d0d0d;
  color: var(--lcars-text-muted); font-family: var(--font-mono); font-size: 0.74rem; letter-spacing: 1px;
}
.sp-iframe { width: 100%; height: 100%; border: none; }
.sp-iframe-hidden { opacity: 0; }
.sp-link {
  display: block; padding: 9px 16px; border-top: 1px solid rgba(204, 153, 204, 0.25); font-family: var(--font-mono);
  font-size: 0.7rem; text-transform: uppercase; letter-spacing: 1px; color: var(--lcars-sky); flex: none;
}
.sp-link:hover { color: var(--lcars-gold); }
@media (max-width: 640px) {
  .scripture-panel { left: 12px !important; right: 12px; top: auto !important; bottom: 12px; width: auto; height: 70vh; }
  .sp-header { cursor: default; }
}
</style>
