<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useCharacterPanel } from '../composables/useCharacterPanel'
import type { DndCharacter } from '../api'

// DH-4: the /char sheet panel — ScripturePanel's movable-LCARS mold pointed at
// a dnd-tools character. View by default; Edit unlocks the fields the table
// actually touches mid-session (HP, AC, speed, abilities, conditions, notes).

const { state, close, save, setEditing } = useCharacterPanel()

const panel = ref<HTMLElement | null>(null)
const pos = ref<{ left: number; top: number } | null>(null)

watch(
  () => state.visible,
  (visible) => {
    if (!visible || pos.value) return
    pos.value = { left: Math.max(16, window.innerWidth - 440 - 36), top: 80 }
  },
)

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

const ABILITIES = ['str', 'dex', 'con', 'int', 'wis', 'cha']
const mod = (score: number) => {
  const m = Math.floor((score - 10) / 2)
  return m >= 0 ? `+${m}` : `${m}`
}

const c = computed(() => state.character)

// Edit buffer — only the fields we let the panel change.
const edit = reactive({
  hp_current: 0, hp_max: 0, ac: 10, speed: 30,
  alignment: '', notes: '', conditions: '',
  abilities: {} as Record<string, number>,
})
watch(
  () => state.editing,
  (on) => {
    if (!on || !c.value) return
    edit.hp_current = c.value.hp_current
    edit.hp_max = c.value.hp_max
    edit.ac = c.value.ac
    edit.speed = c.value.speed
    edit.alignment = c.value.alignment
    edit.notes = c.value.notes
    edit.conditions = (c.value.conditions ?? []).join(', ')
    edit.abilities = { ...c.value.abilities }
  },
)

function submit() {
  const patch: Partial<DndCharacter> = {
    hp_current: Number(edit.hp_current), hp_max: Number(edit.hp_max),
    ac: Number(edit.ac), speed: Number(edit.speed),
    alignment: edit.alignment, notes: edit.notes,
    conditions: edit.conditions.split(',').map((s) => s.trim()).filter(Boolean),
    abilities: Object.fromEntries(Object.entries(edit.abilities).map(([k, v]) => [k, Number(v)])),
  }
  save(patch)
}

const slotLevels = computed(() =>
  Object.entries(c.value?.spell_slots ?? {}).sort(([a], [b]) => Number(a) - Number(b)),
)
</script>

<template>
  <div
    v-if="state.visible"
    ref="panel"
    class="char-panel"
    :style="pos ? { left: pos.left + 'px', top: pos.top + 'px' } : undefined"
  >
    <div class="cp-head" @mousedown="startDrag">
      <span class="cp-grip" aria-hidden="true">≡</span>
      <span class="cp-title">
        {{ c ? c.name : 'Character' }}
        <span v-if="c" class="cp-sub">{{ [c.species, c.class && `${c.class} ${c.level}`].filter(Boolean).join(' · ') }}</span>
      </span>
      <button v-if="c && !state.editing" class="cp-btn" @click="setEditing(true)" title="Edit sheet">✎</button>
      <button v-if="state.editing" class="cp-btn" @click="setEditing(false)" title="Cancel">↩</button>
      <button class="cp-btn" @click="close" title="Close">✕</button>
    </div>

    <div class="cp-body">
      <p v-if="state.loading" class="cp-dim">loading sheet…</p>
      <p v-else-if="state.error" class="cp-error">{{ state.error }}</p>

      <template v-else-if="c && !state.editing">
        <div class="cp-vitals">
          <span class="cp-hp" :class="{ low: c.hp_current * 3 <= c.hp_max }">❤ {{ c.hp_current }}/{{ c.hp_max }}</span>
          <span>🛡 AC {{ c.ac }}</span>
          <span>👟 {{ c.speed }} ft</span>
          <span>✨ XP {{ c.xp }}</span>
        </div>
        <div v-if="c.conditions?.length" class="cp-conditions">
          <span v-for="cond in c.conditions" :key="cond" class="cp-chip">{{ cond }}</span>
        </div>

        <div class="cp-abilities">
          <div v-for="a in ABILITIES" :key="a" class="cp-ab">
            <div class="cp-ab-key">{{ a.toUpperCase() }}</div>
            <div class="cp-ab-score">{{ c.abilities?.[a] ?? 10 }}</div>
            <div class="cp-ab-mod">{{ mod(c.abilities?.[a] ?? 10) }}</div>
          </div>
        </div>

        <div v-if="c.skills?.length" class="cp-row"><b>Skills:</b> {{ c.skills.join(', ').replaceAll('_', ' ') }}</div>
        <div v-if="c.saves?.length" class="cp-row"><b>Saves:</b> {{ c.saves.map((s) => s.toUpperCase()).join(', ') }}</div>

        <template v-if="c.attacks?.length">
          <div class="cp-sect">Attacks</div>
          <div v-for="a in c.attacks" :key="a.name" class="cp-row">
            ⚔️ <b>{{ a.name }}</b> — {{ a.damage }}<template v-if="a.damage_type"> {{ a.damage_type }}</template>
            <span v-if="a.range" class="cp-dim"> (range {{ a.range }})</span>
          </div>
        </template>

        <template v-if="c.spells?.length || slotLevels.length">
          <div class="cp-sect">Spells</div>
          <div v-if="slotLevels.length" class="cp-row cp-dim">
            Slots: <span v-for="[lvl, n] in slotLevels" :key="lvl" class="cp-chip">L{{ lvl }} ×{{ n }}</span>
          </div>
          <div v-for="sp in c.spells" :key="sp.name" class="cp-row">
            ✨ {{ sp.name }} <span class="cp-dim">{{ sp.level > 0 ? `L${sp.level}` : 'cantrip' }}</span>
          </div>
        </template>

        <template v-if="c.inventory?.length">
          <div class="cp-sect">Inventory</div>
          <div v-for="it in c.inventory" :key="it.name" class="cp-row">
            {{ it.name }}<span v-if="it.qty > 1"> ×{{ it.qty }}</span>
            <span v-if="it.notes" class="cp-dim"> — {{ it.notes }}</span>
          </div>
        </template>

        <template v-if="c.features?.length">
          <div class="cp-sect">Features</div>
          <div class="cp-row">{{ c.features.join('; ') }}</div>
        </template>

        <template v-if="c.notes">
          <div class="cp-sect">Notes</div>
          <div class="cp-row cp-notes">{{ c.notes }}</div>
        </template>

        <div class="cp-row cp-dim cp-meta">{{ c.campaign }} · played by {{ c.player || '—' }}</div>
      </template>

      <template v-else-if="c && state.editing">
        <div class="cp-editgrid">
          <label>HP <input v-model.number="edit.hp_current" type="number" /></label>
          <label>Max <input v-model.number="edit.hp_max" type="number" /></label>
          <label>AC <input v-model.number="edit.ac" type="number" /></label>
          <label>Speed <input v-model.number="edit.speed" type="number" /></label>
        </div>
        <div class="cp-abilities">
          <div v-for="a in ABILITIES" :key="a" class="cp-ab">
            <div class="cp-ab-key">{{ a.toUpperCase() }}</div>
            <input v-model.number="edit.abilities[a]" type="number" class="cp-ab-edit" />
          </div>
        </div>
        <label class="cp-editrow">Alignment <input v-model="edit.alignment" /></label>
        <label class="cp-editrow">Conditions <input v-model="edit.conditions" placeholder="poisoned, prone" /></label>
        <label class="cp-editrow">Notes <textarea v-model="edit.notes" rows="4" /></label>
        <p v-if="state.error" class="cp-error">{{ state.error }}</p>
        <div class="cp-actions">
          <button class="cp-save" @click="submit">Save</button>
          <button class="cp-cancel" @click="setEditing(false)">Cancel</button>
        </div>
      </template>
    </div>
  </div>
</template>

<style scoped>
.char-panel {
  position: fixed; z-index: 60; width: 420px; max-width: calc(100vw - 16px);
  max-height: min(72vh, 640px); display: flex; flex-direction: column;
  background: var(--panel, #0c0c14); border: 1px solid var(--accent, #f90);
  border-radius: 14px 14px 6px 6px; box-shadow: 0 12px 36px rgba(0, 0, 0, 0.5);
  font-size: 0.85rem;
}
.cp-head {
  display: flex; align-items: center; gap: 8px; padding: 8px 10px;
  background: var(--accent, #f90); color: #000; cursor: grab; user-select: none;
  border-radius: 12px 12px 0 0; font-weight: 700;
}
.cp-grip { opacity: 0.6 }
.cp-title { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap }
.cp-sub { font-weight: 400; opacity: 0.75; margin-left: 6px; font-size: 0.78rem }
.cp-btn { background: rgba(0,0,0,0.18); border: 0; border-radius: 6px; padding: 2px 8px; cursor: pointer; color: #000 }
.cp-btn:hover { background: rgba(0,0,0,0.32) }
.cp-body { overflow-y: auto; padding: 10px 12px 12px }
.cp-vitals { display: flex; gap: 12px; flex-wrap: wrap; font-weight: 600; margin-bottom: 6px }
.cp-hp.low { color: #f55 }
.cp-conditions { margin-bottom: 6px }
.cp-chip {
  display: inline-block; background: rgba(255, 153, 0, 0.16); border: 1px solid rgba(255, 153, 0, 0.4);
  border-radius: 999px; padding: 0 8px; margin: 0 4px 4px 0; font-size: 0.75rem;
}
.cp-abilities { display: grid; grid-template-columns: repeat(6, 1fr); gap: 6px; margin: 8px 0 }
.cp-ab { text-align: center; background: rgba(255,255,255,0.04); border-radius: 8px; padding: 4px 2px }
.cp-ab-key { font-size: 0.65rem; opacity: 0.7 }
.cp-ab-score { font-weight: 700; font-size: 1rem }
.cp-ab-mod { font-size: 0.75rem; opacity: 0.85 }
.cp-ab-edit { width: 100%; text-align: center; background: rgba(0,0,0,0.4); border: 1px solid #444; border-radius: 6px; color: inherit; padding: 2px 0 }
.cp-sect { margin: 10px 0 2px; font-size: 0.72rem; letter-spacing: 0.08em; text-transform: uppercase; opacity: 0.65 }
.cp-row { margin: 2px 0; line-height: 1.4 }
.cp-dim { opacity: 0.6 }
.cp-notes { white-space: pre-wrap }
.cp-meta { margin-top: 10px; font-size: 0.75rem }
.cp-error { color: #f66 }
.cp-editgrid { display: grid; grid-template-columns: repeat(4, 1fr); gap: 8px; margin-bottom: 4px }
.cp-editgrid label, .cp-editrow { display: flex; flex-direction: column; gap: 2px; font-size: 0.72rem; opacity: 0.9 }
.cp-editrow { margin: 6px 0 }
.cp-editgrid input, .cp-editrow input, .cp-editrow textarea {
  background: rgba(0,0,0,0.4); border: 1px solid #444; border-radius: 6px; color: inherit; padding: 4px 6px; font: inherit;
}
.cp-actions { display: flex; gap: 8px; margin-top: 8px }
.cp-save { background: var(--accent, #f90); color: #000; border: 0; border-radius: 8px; padding: 5px 14px; font-weight: 700; cursor: pointer }
.cp-cancel { background: transparent; color: inherit; border: 1px solid #555; border-radius: 8px; padding: 5px 12px; cursor: pointer }
@media (max-width: 640px) {
  .char-panel { left: 8px !important; right: 8px; width: auto }
}
</style>
