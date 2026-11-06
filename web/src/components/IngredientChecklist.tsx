import { Checkbox, Stack, Text } from '@mantine/core'
import type { Recipe } from '../lib/api/types'

/** The recipe's ingredients as tick boxes. Keys are "group:item", shared with the step views. */
export function IngredientChecklist({
  recipe,
  checked,
  onToggle,
  size = 'md',
}: {
  recipe: Recipe
  checked: Set<string>
  onToggle: (key: string) => void
  size?: 'md' | 'lg'
}) {
  if (recipe.ingredients.length === 0) return <Text c="dimmed">No ingredients yet.</Text>
  return (
    <Stack gap="md">
      {recipe.ingredients.map((group, gi) => (
        <div key={gi}>
          {group.name && (
            <Text fw={700} tt="uppercase" size="xs" c="dimmed" mb={6} style={{ letterSpacing: '0.04em' }}>
              {group.name}
            </Text>
          )}
          <Stack gap={size === 'lg' ? 10 : 6}>
            {group.items.map((item, ii) => (
              <IngredientCheck key={ii} text={item} checked={checked.has(`${gi}:${ii}`)} onChange={() => onToggle(`${gi}:${ii}`)} size={size} />
            ))}
          </Stack>
        </div>
      ))}
    </Stack>
  )
}

export function IngredientCheck({
  text,
  checked,
  onChange,
  size = 'md',
}: {
  text: string
  checked: boolean
  onChange: () => void
  size?: 'md' | 'lg'
}) {
  return (
    <Checkbox
      size={size}
      checked={checked}
      onChange={onChange}
      color="orange"
      label={text}
      styles={{
        label: {
          textDecoration: checked ? 'line-through' : undefined,
          opacity: checked ? 0.55 : 1,
          fontSize: size === 'lg' ? '1.2rem' : undefined,
          lineHeight: 1.4,
          cursor: 'pointer',
        },
        input: { cursor: 'pointer' },
      }}
    />
  )
}
