import type { Recipe } from './api/types'

// Cooking helpers shared by the recipe page and cook mode. The matcher and duration parser are
// ports of android/.../IngredientMatcher.kt and DurationParser.kt: keep the two in step.

/** "Prep: Marinate the chicken." -> ["Prep", "Marinate the chicken."]. Only a one-word label is split off. */
export function splitPhase(text: string): [string | null, string] {
  const m = /^(\p{Lu}\p{L}{1,11}):\s+([\s\S]+)$/u.exec(text.trim())
  return m ? [m[1], m[2]] : [null, text]
}

// ---------------------------------------------------------------- durations

export interface FoundDuration {
  text: string
  /** The low end of a range ("4–5 hours" starts at 4 hours). */
  seconds: number
  isRange: boolean
}

const NUM = '(?:\\d+(?:\\.\\d+)?[½¼¾⅓⅔]?|[½¼¾⅓⅔])'
const DURATION = new RegExp(
  `(?<![\\d.])(${NUM})(?:\\s*(?:-|–|—|to)\\s*(${NUM}))?\\s*(hours?|hrs?|minutes?|mins?|seconds?|secs?)\\b`,
  'gi',
)
const FRACTIONS: Record<string, number> = { '½': 0.5, '¼': 0.25, '¾': 0.75, '⅓': 1 / 3, '⅔': 2 / 3 }

function parseNumber(s: string): number | null {
  const last = s[s.length - 1]
  const frac = FRACTIONS[last]
  const whole = frac === undefined ? s : s.slice(0, -1)
  const base = whole === '' ? 0 : Number(whole)
  return Number.isNaN(base) ? null : base + (frac ?? 0)
}

export function findDurations(text: string): FoundDuration[] {
  const seen = new Set<string>()
  const out: FoundDuration[] = []
  for (const m of text.matchAll(DURATION)) {
    const low = parseNumber(m[1])
    if (low === null) continue
    const unit = { h: 3600, m: 60, s: 1 }[m[3][0].toLowerCase() as 'h' | 'm' | 's']
    const seconds = Math.trunc(low * unit)
    const label = m[0].trim()
    if (seconds <= 0 || seen.has(label.toLowerCase())) continue
    seen.add(label.toLowerCase())
    out.push({ text: label, seconds, isRange: !!m[2] })
  }
  return out
}

/** 3725 -> "1:02:05", 205 -> "3:25". */
export function clock(totalSeconds: number): string {
  const s = Math.max(0, Math.floor(totalSeconds))
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const sec = String(s % 60).padStart(2, '0')
  return h > 0 ? `${h}:${String(m).padStart(2, '0')}:${sec}` : `${m}:${sec}`
}

// ---------------------------------------------------------------- ingredient matching

export interface IngredientRef {
  /** "group:item", the same key the recipe page ticks ingredients with. */
  key: string
  text: string
}

const FILLERS = new Set(
  (
    'tsp teaspoon tbsp tablespoon cup lb pound oz ounce gram kg ml liter litre pinch dash clove can ' +
    'slice stick bunch package pkg piece handful ' +
    'a an the of and or to for with in on at into plus about more taste optional ' +
    'diced minced chopped sliced grated shredded crushed mashed melted softened cubed halved peeled ' +
    'thinly thickly finely roughly freshly fresh uncooked cooked raw large small medium extra ' +
    'boneless skinless kosher plain whole ground'
  )
    .split(' ')
    .map(singular),
)

function singular(w: string): string {
  if (w.length > 4 && w.endsWith('ies')) return w.slice(0, -3) + 'y'
  if (w.length > 4 && w.endsWith('oes')) return w.slice(0, -2)
  if (w.length > 3 && w.endsWith('s') && !w.endsWith('ss')) return w.slice(0, -1)
  return w
}

function tokens(text: string, dropFillers = true): Set<string> {
  const words = text
    .replace(/\([^)]*\)/g, ' ')
    .toLowerCase()
    .split(/[^\p{L}]+/u)
    .map(singular)
    .filter((w) => w.length >= 2 && (!dropFillers || !FILLERS.has(w)))
  return new Set(words)
}

const containsAll = (big: Set<string>, small: Set<string>) => [...small].every((t) => big.has(t))
const overlaps = (a: Set<string>, b: Set<string>) => [...a].some((t) => b.has(t))

interface Item {
  key: string
  text: string
  tokens: Set<string>
}
interface Group {
  nameTokens: Set<string>
  items: Item[]
  /** A name like "Chicken" or "Rice" is also an ingredient word; "Marinade" is only a label. */
  isLabel: boolean
}
interface Candidate {
  item: Item
  group: number
  full: boolean
  viaGroup: boolean
}

/**
 * Which ingredients each step uses, from the text alone. An ingredient matches when its
 * meaningful words show up in the step; a step that names a group ("whisk the marinade") brings
 * that group in and drops other groups' copies of the same thing (every group has its own salt).
 */
export function matchIngredients(recipe: Recipe): IngredientRef[][] {
  const groups: Group[] = recipe.ingredients.map((g, gi) => {
    const nameTokens = tokens(g.name)
    const items = g.items.map((text, ii) => ({ key: `${gi}:${ii}`, text, tokens: tokens(text.split(',')[0]) }))
    return {
      nameTokens,
      items,
      isLabel: nameTokens.size > 0 && !items.some((it) => containsAll(it.tokens, nameTokens)),
    }
  })
  return recipe.steps.map((step) => matchStep(groups, tokens([step.text, ...(step.substeps ?? [])].join(' '), false)))
}

function matchStep(groups: Group[], step: Set<string>): IngredientRef[] {
  const focus = new Set(groups.flatMap((g, i) => (g.nameTokens.size > 0 && containsAll(step, g.nameTokens) ? [i] : [])))
  const whole = new Set([...focus].filter((i) => groups[i].isLabel))

  const found: Candidate[] = []
  groups.forEach((g, gi) => {
    for (const item of g.items) {
      if (item.tokens.size === 0) continue
      if (whole.has(gi)) found.push({ item, group: gi, full: true, viaGroup: true })
      else if ([...item.tokens].some((t) => step.has(t))) {
        found.push({ item, group: gi, full: containsAll(step, item.tokens), viaGroup: false })
      }
    }
  })

  // "onion" vs "red onion": the more specific line wins when the step is that specific,
  // and the plain line wins when the step only says "onion".
  const specific = found.filter(
    (c) =>
      c.viaGroup ||
      !found.some((o) => {
        if (o === c || o.item.tokens.size === c.item.tokens.size) return false
        return o.item.tokens.size > c.item.tokens.size
          ? o.full && containsAll(o.item.tokens, c.item.tokens)
          : o.full && !c.full && containsAll(c.item.tokens, o.item.tokens)
      }),
  )

  // Another group's copy of something the focused group already lists is a duplicate, not a second use.
  const focusTokens = specific.filter((c) => focus.has(c.group)).map((c) => c.item.tokens)
  return specific
    .filter((c) => c.viaGroup || focus.has(c.group) || !focusTokens.some((f) => overlaps(f, c.item.tokens)))
    .map((c) => ({ key: c.item.key, text: c.item.text }))
}
