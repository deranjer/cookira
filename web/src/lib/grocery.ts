import type { IngredientGroup } from './api/types'

export interface GroceryItem {
  text: string
  sources: string[]
}

// Keep in sync with Android's GroceryList.kt. Only total matching names and units;
// free-text ranges, alternatives, package sizes and different preparations stay intact.
const fractions: Record<string, number> = { '½': 1 / 2, '¼': 1 / 4, '¾': 3 / 4, '⅓': 1 / 3, '⅔': 2 / 3, '⅛': 1 / 8, '⅜': 3 / 8, '⅝': 5 / 8, '⅞': 7 / 8 }
const quantity = /^(\d+\s+\d+\/\d+|\d+\/\d+|\d*\s*[½¼¾⅓⅔⅛⅜⅝⅞]|\d+(?:\.\d+)?)\s+(.+)$/u
const units: Record<string, string> = {}
for (const [unit, aliases] of Object.entries({
  tsp: 'tsp teaspoon teaspoons', tbsp: 'tbsp tablespoon tablespoons', cup: 'cup cups',
  lb: 'lb lbs pound pounds', oz: 'oz ounce ounces', g: 'g gram grams', kg: 'kg kilogram kilograms',
  ml: 'ml milliliter milliliters millilitre millilitres', l: 'l liter liters litre litres',
})) for (const alias of aliases.split(' ')) units[alias] = unit

function number(text: string): number {
  const fraction = fractions[text.at(-1)!]
  if (fraction !== undefined) return Number(text.slice(0, -1).trim()) + fraction
  if (text.includes('/')) {
    const parts = text.split(/[\s/]+/).map(Number)
    return parts.length === 3 ? parts[0] + parts[1] / parts[2] : parts[0] / parts[1]
  }
  return Number(text)
}

function format(value: number): string {
  const whole = Math.floor(value)
  for (const denominator of [2, 3, 4, 8, 16]) {
    const numerator = Math.round((value - whole) * denominator)
    if (numerator > 0 && numerator < denominator && Math.abs(value - whole - numerator / denominator) < 1e-8)
      return `${whole ? `${whole} ` : ''}${numerator}/${denominator}`
  }
  return String(Number(value.toPrecision(12)))
}

export function groceryList(groups: IngredientGroup[]): GroceryItem[] {
  const rows: (GroceryItem & { amount?: number; suffix?: string })[] = []
  const merged = new Map<string, number>()
  for (const group of groups) for (const raw of group.items) {
    const text = raw.trim().replace(/\s+/g, ' ')
    if (!text) continue
    const source = group.name.trim() ? `${group.name.trim()}: ${text}` : text
    const match = quantity.exec(text)
    const amount = match ? number(match[1]) : NaN
    let suffix = match?.[2] ?? ''
    const unitMatch = /^(\p{L}+)\.?\s+(.+)$/u.exec(suffix)
    if (unitMatch && units[unitMatch[1].toLowerCase()]) suffix = `${units[unitMatch[1].toLowerCase()]} ${unitMatch[2]}`
    // Do not infer an amount for a range, package notation or alternative.
    if (!Number.isFinite(amount) || amount <= 0 || !/^\p{L}/u.test(suffix) || /^to\b|\b(or|plus|divided)\b|[()+]/i.test(suffix)) {
      rows.push({ text, sources: [source] })
      continue
    }
    const key = suffix.toLowerCase()
    const index = merged.get(key)
    if (index === undefined) {
      merged.set(key, rows.length)
      rows.push({ text, amount, suffix, sources: [source] })
    } else {
      const row = rows[index]
      row.amount! += amount
      row.text = `${format(row.amount!)} ${row.suffix}`
      row.sources.push(source)
    }
  }
  return rows.map(({ text, sources }) => ({ text, sources }))
}

/** One ingredient per line, ready to paste into a shopping list. */
export function groceryText(items: GroceryItem[], selected?: ReadonlySet<number>): string {
  return items.filter((_, index) => !selected || selected.has(index)).map((item) => item.text).join('\n')
}
