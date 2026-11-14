import test from 'node:test'
import assert from 'node:assert/strict'
import { groceryList } from '../src/lib/grocery.ts'

const group = (entries, name = '') => ({ name, items: [], entries })
const ingredient = (id, quantity, extra = {}) => ({ ingredientId: id, name: id, quantity, ...extra })

test('shared identity merges preparations and compatible units, with source amounts retained', () => {
  const result = groceryList([
    group([ingredient('green onion', 2, { preparation: 'sliced' }), ingredient('olive oil', 1, { unit: 'tbsp' })], 'Sauce'),
    group([ingredient('green onion', 3, { preparation: 'diced' }), ingredient('olive oil', 3, { unit: 'tsp' })], 'Rice'),
  ])
  assert.deepEqual(result.map(r => r.text), ['5 green onion', '2 tbsp olive oil'])
  assert.equal(result[0].sources.length, 2)
  assert.match(result[0].sources[0], /Sauce:/)
})

test('ranges, sizes, optional entries, notes and incompatible units remain separate', () => {
  const entries = [
    ingredient('onion', 1, { size: 'small' }), ingredient('onion', 2, { size: 'large' }),
    ingredient('onion', 1, { optional: true }), ingredient('onion', 2),
    ingredient('onion', 2, { quantityMax: 3 }), ingredient('onion', undefined, { notes: 'as needed' }),
    ingredient('flour', 1, { unit: 'cup' }), ingredient('flour', 100, { unit: 'g' }),
    ingredient('garlic', 2), ingredient('garlic powder', 1, { unit: 'tsp' }),
  ]
  const result = groceryList([group(entries)])
  assert.equal(result.length, entries.length)
  assert.equal(result[2].optional, true)
  assert.equal(result[4].text, '2–3 onion')
  assert.equal(result[5].text, 'onion, as needed')
})

test('legacy lines are retained and never merged with a guessed structured identity', () => {
  const result = groceryList([group([ingredient('onion', 2), { text: '1 onion' }]), { name: '', items: ['2 onion'] }])
  assert.deepEqual(result.map(r => r.text), ['2 onion', '3 onion'])
})
