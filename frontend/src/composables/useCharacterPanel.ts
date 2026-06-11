import { reactive, readonly } from 'vue'
import { api, type DndCharacter } from '../api'

// DH-4: the /char panel — a movable sheet viewer/editor in the ScripturePanel
// mold. Opens on /char (your character) or /char <name>; edits PATCH through
// the chattermax proxy (the sheet's player or a room admin).

const state = reactive({
  visible: false,
  loading: false,
  editing: false,
  error: '',
  roomId: '',
  character: null as DndCharacter | null,
})

async function open(roomId: string, name?: string) {
  state.visible = true
  state.loading = true
  state.editing = false
  state.error = ''
  state.roomId = roomId
  try {
    state.character = name
      ? await api.dndCharacter(roomId, name)
      : await api.dndMyCharacter(roomId)
  } catch (e) {
    state.character = null
    state.error = e instanceof Error ? e.message : String(e)
  } finally {
    state.loading = false
  }
}

async function save(patch: Partial<DndCharacter>) {
  if (!state.character) return
  state.error = ''
  try {
    state.character = await api.dndPatchCharacter(state.roomId, state.character.name, patch)
    state.editing = false
  } catch (e) {
    state.error = e instanceof Error ? e.message : String(e)
  }
}

function close() {
  state.visible = false
}

function setEditing(on: boolean) {
  state.editing = on
  state.error = ''
}

export function useCharacterPanel() {
  return { state: readonly(state), open, close, save, setEditing }
}
