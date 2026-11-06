import { Badge, Box, Button, Divider, Grid, Group, Paper, SimpleGrid, Stack, Text, Title, UnstyledButton } from '@mantine/core'
import { useDisclosure } from '@mantine/hooks'
import { IconCheck, IconChefHat, IconClock, IconFlame, IconSoup, IconToolsKitchen2 } from '@tabler/icons-react'
import { useMemo, useState } from 'react'
import ReactMarkdown from 'react-markdown'
import { CookMode } from './CookMode'
import { DurationButtons, TimerDock, useCookTimers } from './CookTimers'
import { FramedImage, HEADER_ASPECT, framingOf } from './FramedImage'
import { IngredientCheck, IngredientChecklist } from './IngredientChecklist'
import type { Recipe } from '../lib/api/types'
import { matchIngredients, splitPhase } from '../lib/cook'
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

/**
 * The read-only rendering of a recipe, shared by the app and the public share page.
 * Wide screens put the ingredients beside the directions (and keep them in view while you scroll);
 * narrow ones stack them. Ingredients and steps can be ticked off, each step shows the ingredients
 * it uses, times in the text become timers, and "Start cooking" opens a one-step-at-a-time view.
 */
export function RecipeView({ recipe, imageUrl }: { recipe: Recipe; imageUrl?: string }) {
  const total = recipe.prepMinutes + recipe.cookMinutes
  const [checked, setChecked] = useState<Set<string>>(new Set())
  const [doneSteps, setDoneSteps] = useState<Set<number>>(new Set())
  const [current, setCurrent] = useState<number | null>(null)
  const [cooking, cook] = useDisclosure(false)
  const timers = useCookTimers(recipe.title)
  const used = useMemo(() => matchIngredients(recipe), [recipe])

  const toggle = (key: string) =>
    setChecked((c) => {
      const n = new Set(c)
      if (!n.delete(key)) n.add(key)
      return n
    })
  const toggleStep = (i: number) =>
    setDoneSteps((c) => {
      const n = new Set(c)
      if (!n.delete(i)) n.add(i)
      return n
    })

  return (
    <Stack gap="lg" maw={1080} mx="auto">
      {imageUrl && (
        <FramedImage
          src={imageUrl}
          framing={framingOf(recipe)}
          aspect={HEADER_ASPECT}
          maxHeight={380}
          radius="var(--mantine-radius-md)"
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

      {recipe.steps.length > 0 && (
        <Button size="lg" leftSection={<IconChefHat size={22} />} onClick={cook.open} className="no-print" fullWidth>
          Start cooking
        </Button>
      )}

      <Grid gutter="xl">
        <Grid.Col span={{ base: 12, md: 5 }}>
          <Box className="recipe-sticky">
            <Title order={2} mb="sm">
              Ingredients
            </Title>
            <IngredientChecklist recipe={recipe} checked={checked} onToggle={toggle} />
          </Box>
        </Grid.Col>

        <Grid.Col span={{ base: 12, md: 7 }}>
          <Title order={2} mb="sm">
            Directions
          </Title>
          {recipe.steps.length === 0 && <Text c="dimmed">No steps yet.</Text>}
          <Stack gap="md">
            {recipe.steps.map((step, i) => {
              const [phase, text] = splitPhase(step.text)
              const isDone = doneSteps.has(i)
              const isCurrent = current === i
              const whole = [step.text, ...(step.substeps ?? [])].join(' ')
              return (
                <Paper
                  key={i}
                  withBorder
                  p="md"
                  radius="md"
                  onClick={() => setCurrent(isCurrent ? null : i)}
                  style={{
                    cursor: 'pointer',
                    borderColor: isCurrent ? 'var(--mantine-color-orange-6)' : undefined,
                    borderWidth: isCurrent ? 2 : 1,
                    opacity: isDone ? 0.55 : 1,
                  }}
                >
                  <Group align="flex-start" wrap="nowrap" gap="md">
                    <UnstyledButton
                      aria-label={isDone ? `Step ${i + 1} done, tap to undo` : `Mark step ${i + 1} done`}
                      onClick={(e) => {
                        e.stopPropagation()
                        toggleStep(i)
                      }}
                      style={{
                        width: 40,
                        height: 40,
                        flex: '0 0 40px',
                        borderRadius: '50%',
                        display: 'grid',
                        placeItems: 'center',
                        fontWeight: 700,
                        fontSize: 18,
                        color: isDone ? '#fff' : 'var(--mantine-color-orange-8)',
                        background: isDone ? 'var(--mantine-color-orange-6)' : 'var(--mantine-color-orange-light)',
                      }}
                    >
                      {isDone ? <IconCheck size={22} /> : i + 1}
                    </UnstyledButton>
                    <Stack gap="xs" style={{ flex: 1, minWidth: 0 }}>
                      {phase && (
                        <Text c="orange" fw={700} tt="uppercase" size="xs" style={{ letterSpacing: '0.06em' }}>
                          {phase}
                        </Text>
                      )}
                      <Text fz="1.15rem" lh={1.6} td={isDone ? 'line-through' : undefined}>
                        {text}
                      </Text>
                      {step.substeps && step.substeps.length > 0 && (
                        <Stack gap={4} pl="xs">
                          {step.substeps.map((s, j) => (
                            <Group key={j} gap="xs" align="flex-start" wrap="nowrap">
                              <Text c="dimmed" w={20} lh={1.6}>
                                {String.fromCharCode(97 + (j % 26))}.
                              </Text>
                              <Text lh={1.6}>{s}</Text>
                            </Group>
                          ))}
                        </Stack>
                      )}
                      <div onClick={(e) => e.stopPropagation()}>
                        <DurationButtons text={whole} stepLabel={`Step ${i + 1}`} api={timers} />
                      </div>
                      {isCurrent && used[i].length > 0 && (
                        <Stack gap={6} onClick={(e) => e.stopPropagation()} pt={4}>
                          <Text size="xs" c="dimmed" fw={700} tt="uppercase" style={{ letterSpacing: '0.04em' }}>
                            You'll need
                          </Text>
                          {used[i].map((ref) => (
                            <IngredientCheck key={ref.key} text={ref.text} checked={checked.has(ref.key)} onChange={() => toggle(ref.key)} />
                          ))}
                        </Stack>
                      )}
                    </Stack>
                  </Group>
                </Paper>
              )
            })}
          </Stack>
        </Grid.Col>
      </Grid>

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

      <CookMode recipe={recipe} opened={cooking} onClose={cook.close} checked={checked} onToggle={toggle} timers={timers} />
      <TimerDock api={timers} />
    </Stack>
  )
}
