import { Alert, Button, Checkbox, Group, Modal, Stack, Text, Textarea } from '@mantine/core'
import { useState } from 'react'
import type { IngredientGroup } from '../lib/api/types'
import { groceryList, groceryText } from '../lib/grocery'

export function GroceryView({ ingredients }: { ingredients: IngredientGroup[] }) {
  const [opened, setOpened] = useState(false)
  return <>
    <Button variant="light" onClick={() => setOpened(true)} className="no-print">Grocery view</Button>
    <Modal opened={opened} onClose={() => setOpened(false)} title="Grocery list" size="lg">
      {opened && <GroceryContents ingredients={ingredients} />}
    </Modal>
  </>
}

function GroceryContents({ ingredients }: { ingredients: IngredientGroup[] }) {
  const [items] = useState(() => groceryList(ingredients))
  const [selected, setSelected] = useState(() => new Set(items.map((_, i) => i)))
  const [status, setStatus] = useState('')
  const [fallback, setFallback] = useState<string | null>(null)
  const copy = async (all: boolean) => {
    const text = groceryText(items, all ? undefined : selected)
    setStatus('')
    setFallback(null)
    try {
      await navigator.clipboard.writeText(text)
      setStatus(`Copied ${all ? items.length : selected.size} items.`)
    } catch {
      setFallback(text)
    }
  }
  return <Stack>
    <Text size="sm" c="dimmed">Matching ingredient names and units are totaled across groups. Other entries stay separate. Select what you need to buy.</Text>
    {items.length === 0 ? <Text c="dimmed">No ingredients yet.</Text> : <>
      <Group>
        <Button onClick={() => void copy(true)}>Copy all</Button>
        <Button variant="light" disabled={selected.size === 0} onClick={() => void copy(false)}>Copy selected ({selected.size})</Button>
      </Group>
      <Group>
        <Button size="compact-sm" variant="subtle" onClick={() => setSelected(new Set(items.map((_, i) => i)))}>Select all</Button>
        <Button size="compact-sm" variant="subtle" onClick={() => setSelected(new Set())}>Clear selection</Button>
      </Group>
      {items.map((item, i) => <Stack gap={4} key={i}>
        <Checkbox label={item.text} checked={selected.has(i)} onChange={() => setSelected((previous) => {
          const next = new Set(previous)
          if (!next.delete(i)) next.add(i)
          return next
        })} />
        {item.sources.length > 1 && <Text size="xs" c="dimmed" pl={30}>{item.sources.join(' + ')}</Text>}
      </Stack>)}
    </>}
    <Text size="sm" role="status" aria-live="polite">{status}</Text>
    {fallback !== null && <Alert color="orange" title="Copy manually">
      Clipboard access is unavailable. Select and copy the list below.
      <Textarea mt="xs" aria-label="Grocery list to copy" value={fallback} readOnly autosize onFocus={(event) => event.currentTarget.select()} />
    </Alert>}
  </Stack>
}
