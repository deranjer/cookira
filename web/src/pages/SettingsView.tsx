import { useState } from 'react'
import {
  ActionIcon,
  Badge,
  Button,
  Code,
  CopyButton,
  Alert,
  Group,
  Modal,
  PasswordInput,
  Select,
  Stack,
  Table,
  Tabs,
  Text,
  TextInput,
  Title,
} from '@mantine/core'
import { IconCopy, IconCopyCheck, IconTrash } from '@tabler/icons-react'
import {
  useCreateInvite,
  useCreateUser,
  useDisconnectOAuth,
  useInvites,
  useMe,
  useOAuthConnections,
  useRemoveUser,
  useRevokeInvite,
  useUpdateMe,
  useUpdateUser,
  useUsers,
} from '../lib/api/hooks'
import { ApiError } from '../lib/api/client'

function formatDate(iso: string | null) {
  if (!iso) return 'Never'
  return new Date(iso).toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' })
}

function CopyCode({ value }: { value: string }) {
  return (
    <Group gap="xs" wrap="nowrap" align="flex-start">
      <Code block flex={1} style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-all' }}>
        {value}
      </Code>
      <CopyButton value={value}>
        {({ copied, copy }) => (
          <ActionIcon variant="light" color={copied ? 'teal' : 'orange'} onClick={copy} aria-label="Copy">
            {copied ? <IconCopyCheck size={16} /> : <IconCopy size={16} />}
          </ActionIcon>
        )}
      </CopyButton>
    </Group>
  )
}

function ConnectedAppsSection() {
  const { data: connections } = useOAuthConnections()
  const disconnect = useDisconnectOAuth()

  return (
    <Stack gap="md">
      <div>
        <Title order={3}>Connected apps</Title>
        <Text size="sm" c="dimmed">
          Apps like Claude and ChatGPT that you approved through the sign-in page. Disconnecting revokes their access immediately.
        </Text>
      </div>
      {connections && connections.length > 0 ? (
        <Table verticalSpacing="sm">
          <Table.Thead>
            <Table.Tr>
              <Table.Th>App</Table.Th>
              <Table.Th>Access</Table.Th>
              <Table.Th>Connected</Table.Th>
              <Table.Th>Last used</Table.Th>
              <Table.Th />
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {connections.map((c) => (
              <Table.Tr key={c.id}>
                <Table.Td>{c.clientName}</Table.Td>
                <Table.Td>
                  <Badge size="sm" variant="light" color={c.scopes.includes('write') ? 'orange' : 'gray'}>
                    {c.scopes.includes('write') ? 'Read-write' : 'Read-only'}
                  </Badge>
                </Table.Td>
                <Table.Td>{formatDate(c.createdAt)}</Table.Td>
                <Table.Td>{formatDate(c.lastUsedAt)}</Table.Td>
                <Table.Td>
                  <ActionIcon
                    variant="subtle"
                    color="red"
                    aria-label="Disconnect app"
                    onClick={() => disconnect.mutate(c.id)}
                    loading={disconnect.isPending && disconnect.variables === c.id}
                  >
                    <IconTrash size={16} />
                  </ActionIcon>
                </Table.Td>
              </Table.Tr>
            ))}
          </Table.Tbody>
        </Table>
      ) : (
        <Text size="sm" c="dimmed">
          No connected apps yet.
        </Text>
      )}
    </Stack>
  )
}

function ConnectSection() {
  const base = window.location.origin
  const isHttps = base.startsWith('https://')
  const publicBase = isHttps ? base : 'https://<your-public-host>'
  const claudeCode = `claude mcp add --transport http cookira ${base}/mcp`
  const codex = `# ~/.codex/config.toml\n[mcp_servers.cookira]\nurl = "${base}/mcp"`

  const cloudNote = !isHttps && (
    <Alert color="yellow" title="Needs a public HTTPS address">
      Hosted apps (claude.ai, ChatGPT) cannot reach <Code>localhost</Code>. Set <Code>PUBLIC_URL</Code> to the HTTPS
      address Cookira is reachable at from the internet, then open Settings from that address.
    </Alert>
  )

  return (
    <Stack gap="md">
      <div>
        <Title order={3}>Connect an agent</Title>
        <Text size="sm" c="dimmed">
          Agents connect with OAuth, so there are no keys to copy around. Add Cookira&apos;s MCP address to your client,
          sign in here when it asks, and choose whether the app may only read recipes or also create and edit them.
          Connected apps are listed above, where you can disconnect them.
        </Text>
      </div>
      <Tabs defaultValue="claude" keepMounted={false}>
        <Tabs.List>
          <Tabs.Tab value="claude">Claude</Tabs.Tab>
          <Tabs.Tab value="code">Claude Code</Tabs.Tab>
          <Tabs.Tab value="codex">Codex</Tabs.Tab>
          <Tabs.Tab value="chatgpt">ChatGPT</Tabs.Tab>
        </Tabs.List>
        <Tabs.Panel value="claude" pt="sm">
          <Stack gap="sm">
            {cloudNote}
            <Stack gap={4} component="ol" pl="md" m={0}>
              <li>
                <Text size="sm">In claude.ai or Claude desktop, open Settings → Connectors and add a custom connector with the URL:</Text>
                <CopyCode value={`${publicBase}/mcp`} />
              </li>
              <li>
                <Text size="sm">Choose OAuth / sign-in and let it register automatically (leave client ID and secret empty).</Text>
              </li>
              <li>
                <Text size="sm">Cookira opens a sign-in and consent page. Approve it, choosing read-only or read-write.</Text>
              </li>
            </Stack>
          </Stack>
        </Tabs.Panel>
        <Tabs.Panel value="code" pt="sm">
          <Stack gap="xs">
            <CopyCode value={claudeCode} />
            <Text size="sm">
              Then run <Code>/mcp</Code> inside Claude Code and choose <b>cookira</b> to sign in. Your browser opens
              Cookira&apos;s consent page.
            </Text>
          </Stack>
        </Tabs.Panel>
        <Tabs.Panel value="codex" pt="sm">
          <Stack gap="xs">
            <CopyCode value={codex} />
            <Text size="sm">
              Then run <Code>codex mcp login cookira</Code> to sign in through your browser.
            </Text>
          </Stack>
        </Tabs.Panel>
        <Tabs.Panel value="chatgpt" pt="sm">
          <Stack gap="sm">
            {cloudNote}
            <Stack gap={4} component="ol" pl="md" m={0}>
              <li>
                <Text size="sm">
                  You need a ChatGPT plan with custom MCP support (currently Business, Enterprise or Edu; an admin may
                  need to enable developer mode).
                </Text>
              </li>
              <li>
                <Text size="sm">
                  In ChatGPT, add a custom MCP server (Plugins, or Settings → Apps in developer mode) with the URL:
                </Text>
                <CopyCode value={`${publicBase}/mcp`} />
              </li>
              <li>
                <Text size="sm">Choose <b>OAuth</b> as the authentication. Leave client ID and secret empty; ChatGPT registers itself.</Text>
              </li>
              <li>
                <Text size="sm">ChatGPT opens a Cookira sign-in and consent page. Approve it, choosing read-only or read-write.</Text>
              </li>
            </Stack>
            <Text size="xs" c="dimmed">
              See OpenAI&apos;s{' '}
              <a href="https://help.openai.com/en/articles/12584461-developer-mode-and-mcp-apps-in-chatgpt" target="_blank" rel="noreferrer noopener">
                custom MCP help article
              </a>
              . The ChatGPT desktop app uses the same connectors as the web app.
            </Text>
          </Stack>
        </Tabs.Panel>
      </Tabs>
    </Stack>
  )
}

function InviteModal({ opened, onClose }: { opened: boolean; onClose: () => void }) {
  const createInvite = useCreateInvite()
  const [email, setEmail] = useState('')
  const [role, setRole] = useState('member')
  const [error, setError] = useState<string | null>(null)
  const link = createInvite.data?.link

  function close() {
    createInvite.reset()
    setEmail('')
    setRole('member')
    setError(null)
    onClose()
  }

  return (
    <Modal opened={opened} onClose={close} title="Invite someone">
      {link ? (
        <Stack gap="sm">
          <Text size="sm">
            Send this link to them. It works once and expires in 7 days. You won&apos;t be able to see it again.
          </Text>
          <CopyCode value={link} />
          <Button onClick={close}>Done</Button>
        </Stack>
      ) : (
        <Stack gap="sm">
          <TextInput
            label="Email (optional)"
            description="Lock the invite to one address. Leave blank to let them enter their own."
            value={email}
            onChange={(e) => setEmail(e.currentTarget.value)}
          />
          <Select label="Role" data={['member', 'admin']} value={role} onChange={(v) => setRole(v ?? 'member')} allowDeselect={false} />
          {error && (
            <Text c="red" size="sm">
              {error}
            </Text>
          )}
          <Button
            loading={createInvite.isPending}
            onClick={() =>
              createInvite.mutate(
                { email: email.trim() || undefined, role },
                { onError: (e) => setError(e instanceof ApiError ? e.message : 'Failed to create invite') },
              )
            }
          >
            Create invite link
          </Button>
        </Stack>
      )}
    </Modal>
  )
}

function MembersSection() {
  const { data: me } = useMe()
  const isAdmin = me?.role === 'admin'
  const { data: users } = useUsers()
  const { data: invites } = useInvites(isAdmin)
  const createUser = useCreateUser()
  const updateUser = useUpdateUser()
  const removeUser = useRemoveUser()
  const revokeInvite = useRevokeInvite()
  const [inviteOpen, setInviteOpen] = useState(false)
  const [open, setOpen] = useState(false)
  const [form, setForm] = useState({ name: '', email: '', password: '', role: 'member' })
  const [error, setError] = useState<string | null>(null)
  const [actionError, setActionError] = useState<string | null>(null)
  const fail = (e: unknown) => setActionError(e instanceof ApiError ? e.message : 'Something went wrong')

  return (
    <Stack gap="md">
      <Group justify="space-between">
        <div>
          <Title order={3}>Household</Title>
          <Text size="sm" c="dimmed">
            People who can sign in. Recipes are private to signed-in members unless you share a link.
          </Text>
        </div>
        {isAdmin && (
          <Group gap="xs">
            <Button variant="default" onClick={() => setOpen(true)}>
              Add with password
            </Button>
            <Button onClick={() => setInviteOpen(true)}>Invite</Button>
          </Group>
        )}
      </Group>
      {actionError && (
        <Text c="red" size="sm">
          {actionError}
        </Text>
      )}
      <Table verticalSpacing="sm">
        <Table.Tbody>
          {users?.map((u) => (
            <Table.Tr key={u.id}>
              <Table.Td>{u.name}</Table.Td>
              <Table.Td>{u.email}</Table.Td>
              <Table.Td>
                {isAdmin ? (
                  <Select
                    size="xs"
                    w={110}
                    data={['member', 'admin']}
                    value={u.role}
                    allowDeselect={false}
                    onChange={(v) => {
                      setActionError(null)
                      if (v) updateUser.mutate({ id: u.id, role: v }, { onError: fail })
                    }}
                  />
                ) : (
                  <Badge size="sm" variant="light" color={u.role === 'admin' ? 'orange' : 'gray'}>
                    {u.role}
                  </Badge>
                )}
              </Table.Td>
              {isAdmin && (
                <Table.Td w={40}>
                  {u.id !== me?.id && (
                    <ActionIcon
                      variant="subtle"
                      color="red"
                      aria-label={`Remove ${u.name}`}
                      onClick={() => {
                        setActionError(null)
                        if (window.confirm(`Remove ${u.name}? They will be signed out and lose access.`)) {
                          removeUser.mutate(u.id, { onError: fail })
                        }
                      }}
                    >
                      <IconTrash size={16} />
                    </ActionIcon>
                  )}
                </Table.Td>
              )}
            </Table.Tr>
          ))}
        </Table.Tbody>
      </Table>

      {isAdmin && !!invites?.length && (
        <Stack gap="xs">
          <Title order={4}>Pending invites</Title>
          <Table verticalSpacing="xs">
            <Table.Tbody>
              {invites.map((inv) => (
                <Table.Tr key={inv.id}>
                  <Table.Td>{inv.email ?? <Text c="dimmed">Any email</Text>}</Table.Td>
                  <Table.Td>
                    <Badge size="sm" variant="light" color={inv.role === 'admin' ? 'orange' : 'gray'}>
                      {inv.role}
                    </Badge>
                  </Table.Td>
                  <Table.Td>
                    {inv.expired ? (
                      <Text size="sm" c="red">
                        Expired
                      </Text>
                    ) : (
                      <Text size="sm" c="dimmed">
                        Expires {formatDate(inv.expiresAt)}
                      </Text>
                    )}
                  </Table.Td>
                  <Table.Td w={40}>
                    <ActionIcon
                      variant="subtle"
                      color="red"
                      aria-label="Revoke invite"
                      onClick={() => revokeInvite.mutate(inv.id, { onError: fail })}
                    >
                      <IconTrash size={16} />
                    </ActionIcon>
                  </Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        </Stack>
      )}

      <InviteModal opened={inviteOpen} onClose={() => setInviteOpen(false)} />
      <Modal opened={open} onClose={() => setOpen(false)} title="Add with password">
        <Stack gap="sm">
          <TextInput label="Name" value={form.name} onChange={(e) => setForm({ ...form, name: e.currentTarget.value })} required />
          <TextInput label="Email" value={form.email} onChange={(e) => setForm({ ...form, email: e.currentTarget.value })} required />
          <PasswordInput label="Password (8+ characters)" value={form.password} onChange={(e) => setForm({ ...form, password: e.currentTarget.value })} required />
          <Select label="Role" data={['member', 'admin']} value={form.role} onChange={(v) => setForm({ ...form, role: v ?? 'member' })} allowDeselect={false} />
          {error && (
            <Text c="red" size="sm">
              {error}
            </Text>
          )}
          <Button
            loading={createUser.isPending}
            onClick={() =>
              createUser.mutate(form, {
                onSuccess: () => {
                  setOpen(false)
                  setForm({ name: '', email: '', password: '', role: 'member' })
                  setError(null)
                },
                onError: (e) => setError(e instanceof ApiError ? e.message : 'Failed to add member'),
              })
            }
          >
            Add member
          </Button>
        </Stack>
      </Modal>
    </Stack>
  )
}

function AccountSection() {
  const { data: me } = useMe()
  const update = useUpdateMe()
  const [name, setName] = useState<string | null>(null)
  const [pw, setPw] = useState({ current: '', next: '', confirm: '' })
  const [message, setMessage] = useState<{ ok: boolean; text: string } | null>(null)
  const err = (e: unknown) => setMessage({ ok: false, text: e instanceof ApiError ? e.message : 'Update failed' })

  return (
    <Stack gap="md">
      <div>
        <Title order={3}>Your account</Title>
        <Text size="sm" c="dimmed">
          Changing your password signs you out of every other device.
        </Text>
      </div>
      <Group align="flex-end">
        <TextInput label="Name" value={name ?? me?.name ?? ''} onChange={(e) => setName(e.currentTarget.value)} />
        <Button
          variant="default"
          disabled={name === null || name.trim() === '' || name === me?.name}
          loading={update.isPending}
          onClick={() =>
            update.mutate(
              { name: name ?? undefined },
              { onSuccess: () => setMessage({ ok: true, text: 'Name updated' }), onError: err },
            )
          }
        >
          Save name
        </Button>
      </Group>
      <Stack gap="xs" maw={320}>
        <PasswordInput label="Current password" value={pw.current} onChange={(e) => setPw({ ...pw, current: e.currentTarget.value })} />
        <PasswordInput label="New password (8+ characters)" value={pw.next} onChange={(e) => setPw({ ...pw, next: e.currentTarget.value })} />
        <PasswordInput label="Confirm new password" value={pw.confirm} onChange={(e) => setPw({ ...pw, confirm: e.currentTarget.value })} />
        <Button
          disabled={!pw.current || pw.next.length < 8}
          loading={update.isPending}
          onClick={() => {
            if (pw.next !== pw.confirm) return setMessage({ ok: false, text: 'New passwords do not match' })
            update.mutate(
              { currentPassword: pw.current, password: pw.next },
              {
                onSuccess: () => {
                  setPw({ current: '', next: '', confirm: '' })
                  setMessage({ ok: true, text: 'Password changed; other devices were signed out' })
                },
                onError: err,
              },
            )
          }}
        >
          Change password
        </Button>
      </Stack>
      {message && (
        <Text size="sm" c={message.ok ? 'teal' : 'red'}>
          {message.text}
        </Text>
      )}
    </Stack>
  )
}

export function SettingsView() {
  return (
    <Stack gap="xl" maw={860} mx="auto">
      <Title order={2}>Settings</Title>
      <ConnectedAppsSection />
      <ConnectSection />
      <MembersSection />
      <AccountSection />
    </Stack>
  )
}
