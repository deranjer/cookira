import assert from 'node:assert/strict'
import test from 'node:test'
import { groceryList, groceryText } from '../src/lib/grocery.ts'

const group = (items, name = '') => ({ name, items })
const texts = (groups) => groceryList(groups).map((item) => item.text)

test('totals repeated ingredients across groups and retains source amounts', () => {
  const items = groceryList([group(['3 tbsp olive oil', '2 tsp cumin'], 'Marinade'), group(['1 tablespoon olive oil', '½ tsp cumin'], 'Rice')])
  assert.deepEqual(items.map((item) => item.text), ['4 tbsp olive oil', '2 1/2 tsp cumin'])
  assert.deepEqual(items[0].sources, ['Marinade: 3 tbsp olive oil', 'Rice: 1 tablespoon olive oil'])
})

test('supports mixed, unicode and ASCII fractions, decimals and count amounts', () => {
  assert.deepEqual(texts([group(['1½ cups rice', '1 1/2 cup rice', '1/3 cup milk', '⅓ cup milk', '0.1 g spice', '0.2 g spice', '1 onion', '2 onion'])]),
    ['3 cup rice', '2/3 cup milk', '0.3 g spice', '3 onion'])
})

test('keeps units, names, preparations and ambiguous amounts separate', () => {
  const raw = ['1 tsp salt', '1 tbsp salt', '1 tsp kosher salt', '1 onion, diced', '1 onion, sliced',
    '2–3 tomatoes', '2 to 3 tomatoes', '2 to 3 tomatoes', '1 (14 oz) can tomatoes', '1 (14 oz) can tomatoes',
    'salt to taste', 'salt to taste', '1 tbsp butter or olive oil', '1 tbsp butter or olive oil', '1 cup milk, divided', '1 cup milk, divided']
  assert.deepEqual(texts([group(raw)]), raw)
})

test('normalizes whitespace, case and unit aliases without changing unmatched text', () => {
  assert.deepEqual(texts([group([' 1   tbsp. olive oil ', '2 tablespoons Olive Oil', '', '  '])]), ['3 tbsp olive oil'])
  assert.deepEqual(texts([group(['1/0 cup milk', '1/0 cup milk'])]), ['1/0 cup milk', '1/0 cup milk'])
})

test('copies all or only selected items in list order, one item per line', () => {
  const items = groceryList([group(['1 onion', '2 cups rice', '1 tsp salt'])])
  assert.equal(groceryText(items), '1 onion\n2 cups rice\n1 tsp salt')
  assert.equal(groceryText(items, new Set([2, 0])), '1 onion\n1 tsp salt')
  assert.equal(groceryText(items, new Set()), '')
  assert.deepEqual(groceryList([]), [])
})
