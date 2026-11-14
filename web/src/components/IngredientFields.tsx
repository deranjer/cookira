import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Alert, Autocomplete, Button, Checkbox, Group, Modal, NumberInput, Paper, Select, SimpleGrid, Stack, TagsInput, Text, TextInput } from '@mantine/core'
import type { CatalogIngredient, Ingredient } from '../lib/api/types'
import { ingredientsApi } from '../lib/api/client'

const units = ['', 'tsp', 'tbsp', 'cup', 'g', 'kg', 'oz', 'lb', 'ml', 'l', 'clove', 'can', 'bunch']

export function IngredientFields({ entries, onChange }: { entries: Ingredient[]; onChange: (entries: Ingredient[]) => void }) {
  const { data: catalog = [], error } = useQuery({ queryKey: ['ingredient-catalog'], queryFn: ingredientsApi.list })
  const [manage, setManage] = useState(false)
  const update = (index: number, change: Partial<Ingredient>) => onChange(entries.map((e, i) => i === index ? { ...e, ...change } : e))
  const choices = [...new Set(catalog.flatMap(c => [c.name, ...c.aliases]))]
  return <Stack gap="sm">
    {error && <Alert color="red">Ingredient catalog could not load: {error.message}</Alert>}
    {entries.map((entry, i) => <Paper key={i} withBorder p="xs">
      <Stack gap="xs">
        {entry.text !== undefined ? <>
          <TextInput label="Original ingredient" value={entry.text} onChange={e => update(i, { text: e.currentTarget.value })} />
          <Button size="compact-xs" variant="light" onClick={() => onChange(entries.map((e, j) => j === i ? { name: '', originalText: e.text } : e))}>Structure this ingredient</Button>
        </> : <>
          {entry.originalText && <>
            <Text size="xs" c="dimmed">Original: {entry.originalText}</Text>
            <Button size="compact-xs" variant="subtle" onClick={() => onChange(entries.map((e, j) => j === i ? { text: e.originalText } : e))}>Keep original text</Button>
          </>}
          <Autocomplete label="Ingredient" placeholder="Choose an ingredient or type a new one" data={choices} value={entry.name ?? ''} onChange={name => {
            const match = catalog.find(c => [c.name, ...c.aliases].some(n => n.toLowerCase() === name.trim().toLowerCase()))
            update(i, { name, ingredientId: match?.id, category: match?.category })
          }} />
          <SimpleGrid cols={{ base: 1, sm: 3 }}>
            <NumberInput label="Quantity" placeholder="Unmeasured" min={0.000001} max={1000000} value={entry.quantity ?? ''} onChange={v => update(i, { quantity: typeof v === 'number' ? v : undefined })} />
            <NumberInput label="Up to (optional)" placeholder="For a range" min={0.000001} max={1000000} value={entry.quantityMax ?? ''} onChange={v => update(i, { quantityMax: typeof v === 'number' ? v : undefined })} />
            <Autocomplete label="Unit" placeholder="Count / no unit" data={units.filter(Boolean)} value={entry.unit ?? ''} onChange={unit => update(i, { unit })} />
          </SimpleGrid>
          <SimpleGrid cols={{ base: 1, sm: 2 }}>
            <TextInput label="Size / form" placeholder="medium, large…" value={entry.size ?? ''} onChange={e => update(i, { size: e.currentTarget.value })} />
            <TextInput label="Preparation" placeholder="finely diced, sliced…" value={entry.preparation ?? ''} onChange={e => update(i, { preparation: e.currentTarget.value })} />
          </SimpleGrid>
          <TextInput label="Notes" placeholder="to taste, as needed, or an alternative…" value={entry.notes ?? ''} onChange={e => update(i, { notes: e.currentTarget.value })} />
          <Checkbox label="Optional ingredient" checked={entry.optional ?? false} onChange={e => update(i, { optional: e.currentTarget.checked })} />
        </>}
        <Group justify="space-between">
          <Group gap="xs">
            <Button size="compact-xs" variant="subtle" disabled={i === 0} onClick={() => { const next = [...entries]; [next[i - 1], next[i]] = [next[i], next[i - 1]]; onChange(next) }}>Move up</Button>
            <Button size="compact-xs" variant="subtle" disabled={i === entries.length - 1} onClick={() => { const next = [...entries]; [next[i], next[i + 1]] = [next[i + 1], next[i]]; onChange(next) }}>Move down</Button>
          </Group>
          <Button size="compact-xs" color="red" variant="subtle" onClick={() => onChange(entries.filter((_, j) => j !== i))}>Remove ingredient</Button>
        </Group>
      </Stack>
    </Paper>)}
    <Group>
      <Button size="xs" variant="light" onClick={() => onChange([...entries, { name: '' }])}>Add ingredient</Button>
      <Button size="xs" variant="subtle" onClick={() => setManage(true)}>Manage ingredient catalog</Button>
    </Group>
    <Modal opened={manage} onClose={() => setManage(false)} title="Shared ingredient catalog" size="lg">
      {manage && <CatalogEditor catalog={catalog} />}
    </Modal>
  </Stack>
}

function CatalogEditor({ catalog }: { catalog: CatalogIngredient[] }) {
  const client = useQueryClient()
  const [id, setId] = useState<string | null>(null)
  const [name, setName] = useState('')
  const [category, setCategory] = useState('Other')
  const [aliases, setAliases] = useState<string[]>([])
  const [error, setError] = useState('')
  const [status, setStatus] = useState('')
  const [saving, setSaving] = useState(false)
  const select = (value: string | null) => {
    const c = catalog.find(c => c.id === value)
    setId(value); setName(c?.name ?? ''); setCategory(c?.category ?? 'Other'); setAliases(c?.aliases ?? []); setError(''); setStatus('')
  }
  return <Stack>
    <Text size="sm">Aliases identify the same ingredient. Keep different products, such as garlic and garlic powder, separate.</Text>
    <Select label="Edit an ingredient" placeholder="Choose an ingredient, or create one below" clearable searchable data={catalog.map(c => ({ value: c.id, label: c.name }))} value={id} onChange={select} />
    <TextInput label="Canonical name" value={name} onChange={e => setName(e.currentTarget.value)} />
    <Autocomplete label="Grocery category" data={['Produce', 'Meat', 'Seafood', 'Dairy', 'Pantry', 'Spices', 'Frozen', 'Other']} value={category} onChange={setCategory} />
    <TagsInput label="Aliases" description="Type an alternative name and press Enter" value={aliases} onChange={setAliases} />
    {error && <Alert color="red">{error}</Alert>}
    {status && <Text role="status" c="green">{status}</Text>}
    <Button disabled={!name.trim()} loading={saving} onClick={async () => {
      setSaving(true); setError(''); setStatus('')
      try {
        const c = await ingredientsApi.save({ ...(id ? { id } : {}), name, category, aliases })
        await client.invalidateQueries({ queryKey: ['ingredient-catalog'] })
        setId(c.id); setName(c.name); setAliases(c.aliases); setStatus('Ingredient saved.')
      } catch (e) { setError(e instanceof Error ? e.message : 'Could not save ingredient') }
      finally { setSaving(false) }
    }}>Save ingredient</Button>
    {id && <Button variant="subtle" onClick={() => select(null)}>Create another ingredient</Button>}
  </Stack>
}
