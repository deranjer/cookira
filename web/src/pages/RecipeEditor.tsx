import { useState } from 'react'
import {
  ActionIcon,
  Alert,
  Button,
  Center,
  Group,
  Loader,
  NumberInput,
  Paper,
  SegmentedControl,
  SimpleGrid,
  Stack,
  TagsInput,
  Text,
  Textarea,
  TextInput,
  Title,
} from '@mantine/core'
import { IconArrowDown, IconArrowUp, IconPlus, IconTrash } from '@tabler/icons-react'
import { useNavigate, useParams } from 'react-router-dom'
import { PhotoField } from '../components/PhotoField'
import { DEFAULT_FRAMING, framingOf, isDefaultFraming, type Framing } from '../components/FramedImage'
import { ApiError, recipesApi } from '../lib/api/client'
import { useCreateRecipe, useRecipe, useTags, useUpdateRecipe } from '../lib/api/hooks'
import type { Recipe, RecipeInput } from '../lib/api/types'

interface GroupDraft {
  name: string
  items: string // one ingredient per line
}

interface StepDraft {
  text: string
  substeps: string // one sub-step per line
}

const lines = (s: string) =>
  s
    .split('\n')
    .map((l) => l.trim())
    .filter(Boolean)

function move<T>(list: T[], from: number, to: number): T[] {
  if (to < 0 || to >= list.length) return list
  const next = [...list]
  const [item] = next.splice(from, 1)
  next.splice(to, 0, item)
  return next
}

function EditorForm({ recipe }: { recipe?: Recipe }) {
  const navigate = useNavigate()
  const create = useCreateRecipe()
  const update = useUpdateRecipe(recipe?.id ?? '')
  const { data: knownTags } = useTags()

  const [title, setTitle] = useState(recipe?.title ?? '')
  const [tagline, setTagline] = useState(recipe?.tagline ?? '')
  const [author, setAuthor] = useState(recipe?.author ?? '')
  const [servings, setServings] = useState<number | string>(recipe?.servings || '')
  const [prep, setPrep] = useState<number | string>(recipe?.prepMinutes || '')
  const [cook, setCook] = useState<number | string>(recipe?.cookMinutes || '')
  const [calories, setCalories] = useState<number | string>(recipe?.calories || '')
  const [sourceUrl, setSourceUrl] = useState(recipe?.sourceUrl ?? '')
  const [tags, setTags] = useState<string[]>(recipe?.tags ?? [])
  const [status, setStatus] = useState<string>(recipe?.status ?? 'draft')
  const [notes, setNotes] = useState(recipe?.notes ?? '')
  const [groups, setGroups] = useState<GroupDraft[]>(
    recipe && recipe.ingredients.length > 0
      ? recipe.ingredients.map((g) => ({ name: g.name, items: g.items.join('\n') }))
      : [{ name: '', items: '' }],
  )
  const [steps, setSteps] = useState<StepDraft[]>(
    recipe && recipe.steps.length > 0
      ? recipe.steps.map((s) => ({ text: s.text, substeps: (s.substeps ?? []).join('\n') }))
      : [{ text: '', substeps: '' }],
  )
  const [photo, setPhoto] = useState<File | null>(null)
  const [framing, setFraming] = useState<Framing>(recipe ? framingOf(recipe) : DEFAULT_FRAMING)
  const [error, setError] = useState<string | null>(null)

  const num = (v: number | string) => (typeof v === 'number' ? v : 0)
  const saving = create.isPending || update.isPending

  function save() {
    setError(null)
    const input: RecipeInput = {
      title,
      tagline,
      author,
      servings: num(servings),
      prepMinutes: num(prep),
      cookMinutes: num(cook),
      calories: num(calories),
      sourceUrl,
      tags,
      status: status as Recipe['status'],
      notes,
      ingredients: groups.map((g) => ({ name: g.name.trim(), items: lines(g.items) })),
      steps: steps.map((s) => ({ text: s.text.trim(), substeps: lines(s.substeps) })),
      // For a new recipe the photo (and so its framing) is uploaded after creation.
      ...(recipe ? framing : {}),
    }
    const onError = (e: Error) => setError(e instanceof ApiError ? e.message : 'Save failed')
    if (recipe) {
      update.mutate(input, { onSuccess: (r) => navigate(`/recipes/${r.slug}`), onError })
    } else {
      create.mutate(input, {
        onSuccess: async (r) => {
          if (photo) {
            try {
              await recipesApi.uploadImage(r.id, photo)
              if (!isDefaultFraming(framing)) await recipesApi.update(r.id, framing)
            } catch (e) {
              // The recipe exists now, so keep the user on its edit page to retry the photo.
              setError(`Recipe saved, but the photo upload failed: ${e instanceof Error ? e.message : 'unknown error'}`)
              navigate(`/recipes/${r.slug}/edit`)
              return
            }
          }
          navigate(`/recipes/${r.slug}`)
        },
        onError,
      })
    }
  }

  return (
    <Stack gap="lg" maw={860} mx="auto">
      <Title order={2}>{recipe ? 'Edit recipe' : 'New recipe'}</Title>

      <PhotoField recipe={recipe} pendingFile={photo} onPendingFile={setPhoto} framing={framing} onFramingChange={setFraming} />

      <TextInput label="Title" required value={title} onChange={(e) => setTitle(e.currentTarget.value)} />
      <TextInput label="Tagline" value={tagline} onChange={(e) => setTagline(e.currentTarget.value)} />
      <SimpleGrid cols={{ base: 1, sm: 2 }}>
        <TextInput label="Author" value={author} onChange={(e) => setAuthor(e.currentTarget.value)} />
        <TextInput label="Source URL" placeholder="https://" value={sourceUrl} onChange={(e) => setSourceUrl(e.currentTarget.value)} />
      </SimpleGrid>
      <SimpleGrid cols={{ base: 2, sm: 4 }}>
        <NumberInput label="Servings" min={0} value={servings} onChange={setServings} />
        <NumberInput label="Prep (min)" min={0} value={prep} onChange={setPrep} />
        <NumberInput label="Cook (min)" min={0} value={cook} onChange={setCook} />
        <NumberInput label="Calories" min={0} value={calories} onChange={setCalories} />
      </SimpleGrid>
      <TagsInput
        label="Tags"
        placeholder="Type and press Enter"
        data={knownTags?.map((t) => t.tag) ?? []}
        value={tags}
        onChange={setTags}
      />

      <div>
        <Title order={3} mb="xs">
          Ingredients
        </Title>
        <Stack gap="sm">
          {groups.map((g, i) => (
            <Paper key={i} withBorder p="sm">
              <Group align="flex-start" wrap="nowrap">
                <Stack gap="xs" flex={1}>
                  <TextInput
                    size="xs"
                    placeholder="Group name (optional), e.g. Broth"
                    value={g.name}
                    onChange={(e) => setGroups(groups.map((x, j) => (j === i ? { ...x, name: e.currentTarget.value } : x)))}
                  />
                  <Textarea
                    autosize
                    minRows={3}
                    placeholder="One ingredient per line"
                    value={g.items}
                    onChange={(e) => setGroups(groups.map((x, j) => (j === i ? { ...x, items: e.currentTarget.value } : x)))}
                  />
                </Stack>
                <Stack gap={2}>
                  <ActionIcon variant="subtle" color="gray" aria-label="Move up" onClick={() => setGroups(move(groups, i, i - 1))}>
                    <IconArrowUp size={16} />
                  </ActionIcon>
                  <ActionIcon variant="subtle" color="gray" aria-label="Move down" onClick={() => setGroups(move(groups, i, i + 1))}>
                    <IconArrowDown size={16} />
                  </ActionIcon>
                  <ActionIcon variant="subtle" color="red" aria-label="Remove group" onClick={() => setGroups(groups.filter((_, j) => j !== i))}>
                    <IconTrash size={16} />
                  </ActionIcon>
                </Stack>
              </Group>
            </Paper>
          ))}
          <Button variant="light" size="xs" leftSection={<IconPlus size={14} />} onClick={() => setGroups([...groups, { name: '', items: '' }])} w="fit-content">
            Add ingredient group
          </Button>
        </Stack>
      </div>

      <div>
        <Title order={3} mb="xs">
          Directions
        </Title>
        <Stack gap="sm">
          {steps.map((s, i) => (
            <Paper key={i} withBorder p="sm">
              <Group align="flex-start" wrap="nowrap">
                <Text fw={600} mt={6} w={22}>
                  {i + 1}.
                </Text>
                <Stack gap="xs" flex={1}>
                  <Textarea
                    autosize
                    minRows={2}
                    value={s.text}
                    onChange={(e) => setSteps(steps.map((x, j) => (j === i ? { ...x, text: e.currentTarget.value } : x)))}
                  />
                  <Textarea
                    size="xs"
                    autosize
                    minRows={1}
                    placeholder="Sub-steps, one per line (optional)"
                    value={s.substeps}
                    onChange={(e) => setSteps(steps.map((x, j) => (j === i ? { ...x, substeps: e.currentTarget.value } : x)))}
                  />
                </Stack>
                <Stack gap={2}>
                  <ActionIcon variant="subtle" color="gray" aria-label="Move up" onClick={() => setSteps(move(steps, i, i - 1))}>
                    <IconArrowUp size={16} />
                  </ActionIcon>
                  <ActionIcon variant="subtle" color="gray" aria-label="Move down" onClick={() => setSteps(move(steps, i, i + 1))}>
                    <IconArrowDown size={16} />
                  </ActionIcon>
                  <ActionIcon variant="subtle" color="red" aria-label="Remove step" onClick={() => setSteps(steps.filter((_, j) => j !== i))}>
                    <IconTrash size={16} />
                  </ActionIcon>
                </Stack>
              </Group>
            </Paper>
          ))}
          <Button variant="light" size="xs" leftSection={<IconPlus size={14} />} onClick={() => setSteps([...steps, { text: '', substeps: '' }])} w="fit-content">
            Add step
          </Button>
        </Stack>
      </div>

      <Textarea label="Notes (markdown)" autosize minRows={3} value={notes} onChange={(e) => setNotes(e.currentTarget.value)} />

      <div>
        <Text size="sm" fw={500} mb={4}>
          Status
        </Text>
        <SegmentedControl
          value={status}
          onChange={setStatus}
          data={[
            { label: 'Draft', value: 'draft' },
            { label: 'Published', value: 'published' },
          ]}
        />
      </div>

      {error && <Alert color="red">{error}</Alert>}
      <Group>
        <Button onClick={save} loading={saving} disabled={!title.trim()}>
          {recipe ? 'Save changes' : 'Create recipe'}
        </Button>
        <Button variant="subtle" color="gray" onClick={() => navigate(recipe ? `/recipes/${recipe.slug}` : '/')}>
          Cancel
        </Button>
      </Group>
    </Stack>
  )
}

export function RecipeEditor() {
  const { ref } = useParams()
  const { data: recipe, isLoading, isError } = useRecipe(ref)

  if (!ref) return <EditorForm />
  if (isLoading) {
    return (
      <Center h={300}>
        <Loader color="orange" />
      </Center>
    )
  }
  if (isError || !recipe) return <Alert color="red">Recipe not found.</Alert>
  // Keyed by id so moving between recipes resets the form, but a photo upload
  // (which refetches the recipe) does not wipe unsaved edits.
  return <EditorForm key={recipe.id} recipe={recipe} />
}
