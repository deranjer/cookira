import { useState } from 'react'
import { Badge, Card, Chip, Group, Loader, SegmentedControl, SimpleGrid, Stack, Text, TextInput, Title } from '@mantine/core'
import { useDebouncedValue } from '@mantine/hooks'
import { IconClock, IconSearch } from '@tabler/icons-react'
import { Link } from 'react-router-dom'
import { FramedImage, framingOf } from '../components/FramedImage'
import { recipesApi } from '../lib/api/client'
import { useRecipes, useTags } from '../lib/api/hooks'
import type { RecipeSummary } from '../lib/api/types'
import { formatMinutes } from '../theme'

function RecipeCard({ recipe }: { recipe: RecipeSummary }) {
  const time = formatMinutes(recipe.prepMinutes + recipe.cookMinutes)
  return (
    <Card withBorder padding={0} component={Link} to={`/recipes/${recipe.slug}`} style={{ textDecoration: 'none', color: 'inherit' }}>
      {recipe.hasImage ? (
        <FramedImage
          src={recipesApi.imageUrl(recipe.slug, recipe.updatedAt, true)}
          framing={framingOf(recipe)}
          height={160}
          loading="lazy"
        />
      ) : (
        <div style={{ height: 160, background: 'var(--mantine-color-orange-light)' }} />
      )}
      <Stack gap={6} p="md">
        <Group gap="xs" wrap="nowrap" justify="space-between" align="flex-start">
          <Text fw={600} size="lg" lineClamp={2} ff="Georgia, serif">
            {recipe.title}
          </Text>
          {recipe.status === 'draft' && (
            <Badge color="gray" variant="light">
              Draft
            </Badge>
          )}
        </Group>
        {recipe.tagline && (
          <Text size="sm" c="dimmed" lineClamp={2}>
            {recipe.tagline}
          </Text>
        )}
        <Group gap={6}>
          {time && (
            <Group gap={4}>
              <IconClock size={14} />
              <Text size="xs">{time}</Text>
            </Group>
          )}
          {recipe.tags.slice(0, 3).map((t) => (
            <Badge key={t} size="xs" variant="light" color="orange">
              {t}
            </Badge>
          ))}
        </Group>
      </Stack>
    </Card>
  )
}

export function RecipesView() {
  const [search, setSearch] = useState('')
  const [debounced] = useDebouncedValue(search, 250)
  const [tag, setTag] = useState<string | null>(null)
  const [status, setStatus] = useState('')
  const { data: tags } = useTags()
  const { data, isLoading } = useRecipes({ q: debounced, tag: tag ?? undefined, status: status || undefined })

  return (
    <Stack gap="md" maw={1200} mx="auto">
      <Group justify="space-between">
        <Title order={2}>Recipes</Title>
        <SegmentedControl
          value={status}
          onChange={setStatus}
          data={[
            { label: 'All', value: '' },
            { label: 'Published', value: 'published' },
            { label: 'Drafts', value: 'draft' },
          ]}
        />
      </Group>
      <TextInput
        placeholder="Search recipes, ingredients, tags…"
        leftSection={<IconSearch size={16} />}
        value={search}
        onChange={(e) => setSearch(e.currentTarget.value)}
      />
      {tags && tags.length > 0 && (
        <Chip.Group multiple={false} value={tag} onChange={(v) => setTag(v === tag ? null : (v as string))}>
          <Group gap={6}>
            {tags.map((t) => (
              <Chip key={t.tag} value={t.tag} size="xs" variant="outline" onClick={() => tag === t.tag && setTag(null)}>
                {t.tag} ({t.count})
              </Chip>
            ))}
          </Group>
        </Chip.Group>
      )}
      {isLoading ? (
        <Loader color="orange" />
      ) : data && data.items.length > 0 ? (
        <SimpleGrid cols={{ base: 1, xs: 2, md: 3, lg: 4 }}>
          {data.items.map((r) => (
            <RecipeCard key={r.id} recipe={r} />
          ))}
        </SimpleGrid>
      ) : (
        <Text c="dimmed">No recipes found. Add one with “New recipe”, or ask an agent connected over MCP.</Text>
      )}
    </Stack>
  )
}
