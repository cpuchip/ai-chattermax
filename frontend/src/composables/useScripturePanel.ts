import { reactive, readonly } from 'vue'

// Borrowed from cpuchip.net. A movable, tabbed panel that iframes a passage from
// churchofjesuschrist.org. Scripture text is never hosted here (copyright) — the
// panel pulls it live from the source. In ai-chattermax it opens when a chat
// message links a churchofjesuschrist.org URL (the Library "Computer" emits these).
// Opening more references stacks them as switchable tabs.

export interface ScriptureRef {
  reference: string // display label, e.g. "Alma 22:18"
  url: string // churchofjesuschrist.org study URL for the passage
}

interface ScriptureTab extends ScriptureRef {
  id: number
}

/** Beyond this, the oldest tab drops off so the panel never sprawls. */
const MAX_TABS = 8

let nextId = 1

const state = reactive({
  visible: false,
  minimized: false,
  tabs: [] as ScriptureTab[],
  activeId: 0,
})

function openTab(ref: ScriptureRef) {
  const existing = state.tabs.find((t) => t.url === ref.url)
  if (existing) {
    state.activeId = existing.id
  } else {
    const tab: ScriptureTab = { ...ref, id: nextId++ }
    state.tabs.push(tab)
    if (state.tabs.length > MAX_TABS) state.tabs.shift()
    state.activeId = tab.id
  }
  state.visible = true
  state.minimized = false
}

export function useScripturePanel() {
  return {
    state: readonly(state),

    /** Open a tab directly with a reference object. */
    openDirect(ref: ScriptureRef) {
      openTab(ref)
    },

    /** Bring an open tab to the front. */
    selectTab(id: number) {
      if (state.tabs.some((t) => t.id === id)) state.activeId = id
    },

    /** Close one tab; closing the last one closes the panel. */
    closeTab(id: number) {
      const i = state.tabs.findIndex((t) => t.id === id)
      if (i === -1) return
      state.tabs.splice(i, 1)
      if (state.tabs.length === 0) {
        state.visible = false
        state.activeId = 0
      } else if (state.activeId === id) {
        state.activeId = state.tabs[Math.min(i, state.tabs.length - 1)].id
      }
    },

    /** Close the whole panel and clear every tab. */
    close() {
      state.visible = false
      state.minimized = false
      state.tabs = []
      state.activeId = 0
    },

    /** Shrink to a draggable pill, keeping tabs and iframes alive. */
    minimize() {
      state.minimized = true
    },

    /** Restore the full panel from the pill. */
    expand() {
      state.minimized = false
    },
  }
}
