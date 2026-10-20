import { Badge, Divider, Group, List, Paper, SimpleGrid, Stack, Text, Title } from '@mantine/core'
import { IconClock, IconFlame, IconSoup, IconToolsKitchen2 } from '@tabler/icons-react'
import ReactMarkdown from 'react-markdown'
import type { Recipe } from '../lib/api/types'
import { formatMinutes } from '../theme'

function Stat({ icon, label, value }: { icon: React.ReactNode; label: string; value: string }) {
  if (!value) return null
  return (
    <Group gap={8} wrap="nowrap">
      {icon}
      <div>
        <Text size="xs" c="dimmed" tt="uppercase" fw={600}>
          {label}
        </Text>
        <Text fw={500}>{value}</Text>
      </div>
    </Group>
  )
}

/** The read-only rendering of a recipe, shared by the app and the public share page. */
export function RecipeView({ recipe, imageUrl }: { recipe: Recipe; imageUrl?: string }) {
  const total = recipe.prepMinutes + recipe.cookMinutes
  return (
    <Stack gap="lg" maw={860} mx="auto">
      {imageUrl && (
        <img
          src={imageUrl}
          alt=""
          style={{ width: '100%', maxHeight: 380, objectFit: 'cover', borderRadius: 'var(--mantine-radius-md)' }}
        />
      )}
      <div>
        <Group gap="xs" mb={4}>
          <Title order={1}>{recipe.title}</Title>
          {recipe.status === 'draft' && (
            <Badge color="gray" variant="light" className="no-print">
              Draft
            </Badge>
          )}
        </Group>
        {recipe.tagline && (
          <Text c="dimmed" size="lg">
            {recipe.tagline}
          </Text>
        )}
        {(recipe.author || recipe.sourceUrl) && (
          <Text size="sm" c="dimmed" mt={4}>
            {recipe.author && <>By {recipe.author}</>}
            {recipe.author && recipe.sourceUrl && ' · '}
            {recipe.sourceUrl && (
              <a href={recipe.sourceUrl} target="_blank" rel="noreferrer noopener">
                Source
              </a>
            )}
          </Text>
        )}
        {recipe.tags.length > 0 && (
          <Group gap={6} mt="xs">
            {recipe.tags.map((t) => (
              <Badge key={t} variant="light" color="orange">
                {t}
              </Badge>
            ))}
          </Group>
        )}
      </div>

      <Paper withBorder p="md">
        <SimpleGrid cols={{ base: 2, sm: 4 }} spacing="md">
          <Stat icon={<IconToolsKitchen2 size={20} />} label="Servings" value={recipe.servings ? String(recipe.servings) : ''} />
          <Stat icon={<IconClock size={20} />} label="Prep" value={formatMinutes(recipe.prepMinutes)} />
          <Stat icon={<IconSoup size={20} />} label="Cook" value={formatMinutes(recipe.cookMinutes)} />
          <Stat icon={<IconFlame size={20} />} label="Calories" value={recipe.calories ? `${recipe.calories} kcal` : ''} />
        </SimpleGrid>
        {total > 0 && recipe.prepMinutes > 0 && recipe.cookMinutes > 0 && (
          <Text size="sm" c="dimmed" mt="sm">
            Total {formatMinutes(total)}
          </Text>
        )}
      </Paper>

      <div>
        <Title order={2} mb="sm">
          Ingredients
        </Title>
        <Stack gap="md">
          {recipe.ingredients.length === 0 && <Text c="dimmed">No ingredients yet.</Text>}
          {recipe.ingredients.map((group, i) => (
            <div key={i}>
              {group.name && (
                <Title order={4} mb={4}>
                  {group.name}
                </Title>
              )}
              <List spacing={4}>
                {group.items.map((item, j) => (
                  <List.Item key={j}>{item}</List.Item>
                ))}
              </List>
            </div>
          ))}
        </Stack>
      </div>

      <Divider />

      <div>
        <Title order={2} mb="sm">
          Directions
        </Title>
        {recipe.steps.length === 0 && <Text c="dimmed">No steps yet.</Text>}
        <List type="ordered" spacing="sm">
          {recipe.steps.map((step, i) => (
            <List.Item key={i}>
              {step.text}
              {step.substeps && step.substeps.length > 0 && (
                <List type="ordered" withPadding listStyleType="lower-alpha" spacing={4} mt={4}>
                  {step.substeps.map((s, j) => (
                    <List.Item key={j}>{s}</List.Item>
                  ))}
                </List>
              )}
            </List.Item>
          ))}
        </List>
      </div>

      {recipe.notes && (
        <>
          <Divider />
          <div>
            <Title order={2} mb="sm">
              Notes
            </Title>
            <ReactMarkdown>{recipe.notes}</ReactMarkdown>
          </div>
        </>
      )}
    </Stack>
  )
}
