import { Badge, Button, Drawer, Group, Loader, Stack, Text } from '@mantine/core'
import { useRestoreRevision, useRevisions } from '../lib/api/hooks'

export function HistoryDrawer({ recipeRef, opened, onClose }: { recipeRef: string; opened: boolean; onClose: () => void }) {
  const { data: revisions, isLoading } = useRevisions(recipeRef, opened)
  const restore = useRestoreRevision(recipeRef)

  return (
    <Drawer opened={opened} onClose={onClose} title="History" position="right" size="md">
      {isLoading ? (
        <Loader color="orange" />
      ) : (
        <Stack gap="sm">
          <Text size="sm" c="dimmed">
            Every change, by you or by an agent, is recorded. Restoring rewrites the recipe from that revision (and is itself recorded).
          </Text>
          {revisions?.map((r, i) => (
            <Group key={r.revision} justify="space-between" wrap="nowrap">
              <div>
                <Group gap={6}>
                  <Text fw={500}>#{r.revision}</Text>
                  <Badge size="xs" variant="light">
                    {r.action}
                  </Badge>
                  {r.actor !== 'web' && (
                    <Badge size="xs" variant="light" color="grape">
                      {r.actor}
                    </Badge>
                  )}
                </Group>
                <Text size="xs" c="dimmed">
                  {new Date(r.createdAt).toLocaleString()}
                </Text>
              </div>
              {i > 0 && (
                <Button
                  size="xs"
                  variant="light"
                  loading={restore.isPending && restore.variables === r.revision}
                  onClick={() => restore.mutate(r.revision)}
                >
                  Restore
                </Button>
              )}
            </Group>
          ))}
        </Stack>
      )}
    </Drawer>
  )
}
