import { Box, Button, Divider, Drawer, Group, Modal, Progress, Stack, Text, Title } from '@mantine/core'
import { useDisclosure, useHotkeys } from '@mantine/hooks'
import { IconArrowLeft, IconArrowRight, IconCheck, IconListDetails } from '@tabler/icons-react'
import { useEffect, useMemo, useState } from 'react'
import type { Recipe } from '../lib/api/types'
import { matchIngredients, splitPhase } from '../lib/cook'
import { DurationButtons, type CookTimersApi } from './CookTimers'
import { IngredientCheck, IngredientChecklist } from './IngredientChecklist'

/**
 * Full-screen, one-step-at-a-time view: big type, only the ingredients the step uses, a sheet with
 * the whole list, timers, and ←/→ keys. Keeps the screen awake while open where the browser allows.
 */
export function CookMode({
  recipe,
  opened,
  onClose,
  checked,
  onToggle,
  timers,
}: {
  recipe: Recipe
  opened: boolean
  onClose: () => void
  checked: Set<string>
  onToggle: (key: string) => void
  timers: CookTimersApi
}) {
  const steps = recipe.steps
  const used = useMemo(() => matchIngredients(recipe), [recipe])
  const [page, setPage] = useState(0)
  const [listOpen, list] = useDisclosure(false)
  const done = page >= steps.length

  const next = () => (done ? onClose() : setPage((p) => Math.min(steps.length, p + 1)))
  const back = () => setPage((p) => Math.max(0, p - 1))
  useHotkeys([
    ['ArrowRight', () => opened && next()],
    ['ArrowLeft', () => opened && back()],
  ])

  useEffect(() => {
    if (!opened) return
    setPage(0)
    let lock: WakeLockSentinel | null = null
    navigator.wakeLock
      ?.request('screen')
      .then((l) => (lock = l))
      .catch(() => {})
    return () => {
      void lock?.release().catch(() => {})
    }
  }, [opened])

  const step = steps[page]
  const [phase, text] = step ? splitPhase(step.text) : [null, '']

  return (
    <Modal
      opened={opened}
      onClose={onClose}
      fullScreen
      withinPortal
      title={<Text fw={600}>{done ? 'Done' : `Step ${page + 1} of ${steps.length}`}</Text>}
      transitionProps={{ transition: 'fade', duration: 120 }}
      styles={{ body: { paddingBottom: 'calc(env(safe-area-inset-bottom) + 24px)' } }}
    >
      <Box maw={860} mx="auto">
        <Progress value={steps.length ? (Math.min(page, steps.length) / steps.length) * 100 : 100} color="orange" mb="xl" />
        {done || !step ? (
          <Stack mt="xl" gap="md">
            <Title order={1}>Enjoy!</Title>
            {recipe.notes && <Text c="dimmed" size="lg">{recipe.notes}</Text>}
          </Stack>
        ) : (
          <Stack gap="lg" mih="50vh">
            {phase && (
              <Text c="orange" fw={700} tt="uppercase" size="sm" style={{ letterSpacing: '0.06em' }}>
                {phase}
              </Text>
            )}
            <Text fz={{ base: 26, sm: 34 }} lh={1.4} fw={500}>
              {text}
            </Text>
            {step.substeps?.map((s, j) => (
              <Group key={j} gap="sm" align="flex-start" wrap="nowrap">
                <Text fz={22} lh={1.4} w={28}>
                  {String.fromCharCode(97 + (j % 26))}.
                </Text>
                <Text fz={22} lh={1.4}>
                  {s}
                </Text>
              </Group>
            ))}
            <DurationButtons text={[step.text, ...(step.substeps ?? [])].join(' ')} stepLabel={`Step ${page + 1}`} api={timers} size="lg" />
            {used[page].length > 0 && (
              <>
                <Divider />
                <Text c="dimmed" fw={600}>
                  You'll need
                </Text>
                <Stack gap={12}>
                  {used[page].map((ref) => (
                    <IngredientCheck key={ref.key} text={ref.text} checked={checked.has(ref.key)} onChange={() => onToggle(ref.key)} size="lg" />
                  ))}
                </Stack>
              </>
            )}
          </Stack>
        )}

        <Group mt="xl" grow preventGrowOverflow={false} wrap="nowrap">
          <Button size="xl" variant="default" onClick={back} disabled={page === 0} leftSection={<IconArrowLeft />}>
            Back
          </Button>
          <Button size="xl" variant="default" onClick={list.open} leftSection={<IconListDetails />}>
            Ingredients
          </Button>
          <Button size="xl" onClick={next} rightSection={done ? <IconCheck /> : <IconArrowRight />}>
            {done ? 'Finish' : page === steps.length - 1 ? 'Last step done' : 'Next step'}
          </Button>
        </Group>
      </Box>

      <Drawer opened={listOpen} onClose={list.close} position="right" title={<Text fw={600}>Ingredients</Text>} zIndex={400} size="md">
        <IngredientChecklist recipe={recipe} checked={checked} onToggle={onToggle} size="lg" />
      </Drawer>
    </Modal>
  )
}
