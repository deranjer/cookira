import { useState } from 'react'
import { ActionIcon, Button, CopyButton, Group, Modal, Select, Stack, Text, TextInput } from '@mantine/core'
import { IconCopy, IconCopyCheck, IconTrash } from '@tabler/icons-react'
import { useCreateShare, useRevokeShare, useShares } from '../lib/api/hooks'

const EXPIRY = [
  { value: '0', label: 'Never expires' },
  { value: '1', label: '1 day' },
  { value: '7', label: '7 days' },
  { value: '30', label: '30 days' },
]

export function ShareModal({ recipeRef, opened, onClose }: { recipeRef: string; opened: boolean; onClose: () => void }) {
  const [expiry, setExpiry] = useState('0')
  const { data: shares } = useShares(recipeRef, opened)
  const create = useCreateShare(recipeRef)
  const revoke = useRevokeShare(recipeRef)

  return (
    <Modal opened={opened} onClose={onClose} title="Share this recipe" size="lg">
      <Stack gap="md">
        <Text size="sm" c="dimmed">
          Anyone with a link can view this recipe without signing in. They cannot edit it or see anything else.
          Revoke a link at any time.
        </Text>
        <Group align="flex-end">
          <Select label="Link lifetime" data={EXPIRY} value={expiry} onChange={(v) => setExpiry(v ?? '0')} allowDeselect={false} w={180} />
          <Button onClick={() => create.mutate(Number(expiry))} loading={create.isPending}>
            Create link
          </Button>
        </Group>
        {shares && shares.length > 0 ? (
          <Stack gap="xs">
            {shares.map((sh) => (
              <Group key={sh.id} gap="xs" wrap="nowrap" align="flex-end">
                <TextInput
                  flex={1}
                  readOnly
                  value={sh.url}
                  description={sh.expiresAt ? `Expires ${new Date(sh.expiresAt).toLocaleDateString()}` : 'Never expires'}
                  styles={{ input: { fontFamily: 'monospace', fontSize: 12 } }}
                />
                <CopyButton value={sh.url}>
                  {({ copied, copy }) => (
                    <ActionIcon size="lg" variant="light" color={copied ? 'teal' : 'orange'} onClick={copy} aria-label="Copy link">
                      {copied ? <IconCopyCheck size={16} /> : <IconCopy size={16} />}
                    </ActionIcon>
                  )}
                </CopyButton>
                <ActionIcon
                  size="lg"
                  variant="subtle"
                  color="red"
                  aria-label="Revoke link"
                  loading={revoke.isPending && revoke.variables === sh.id}
                  onClick={() => revoke.mutate(sh.id)}
                >
                  <IconTrash size={16} />
                </ActionIcon>
              </Group>
            ))}
          </Stack>
        ) : (
          <Text size="sm" c="dimmed">
            No active share links.
          </Text>
        )}
      </Stack>
    </Modal>
  )
}
