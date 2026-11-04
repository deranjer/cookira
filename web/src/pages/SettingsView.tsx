import { useState } from 'react'
import {
  ActionIcon,
  Badge,
  Button,
  Checkbox,
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
  useApiKeys,
  useCreateApiKey,
  useCreateInvite,
  useCreateUser,
  useDeleteApiKey,
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

function NewApiKeyModal({ opened, onClose }: { opened: boolean; onClose: () => void }) {
  const [name, setName] = useState('')
  const [readOnly, setReadOnly] = useState(false)
  const [revealedKey, setRevealedKey] = useState<string | null>(null)
  const createKey = useCreateApiKey()

  function handleClose() {
    setName('')
    setReadOnly(false)
    setRevealedKey(null)
    onClose()
  }

  return (
    <Modal opened={opened} onClose={handleClose} title={revealedKey ? 'API key created' : 'New API key'} size="md">
      {revealedKey ? (
        <Stack gap="sm">
          <Text size="sm">Copy this key now. It will not be shown again.</Text>
          <CopyCode value={revealedKey} />
          <Button onClick={handleClose}>Done</Button>
        </Stack>
      ) : (
        <Stack gap="sm">
          <TextInput
            label="Name"
            placeholder="e.g. Claude desktop"
            value={name}
            onChange={(e) => setName(e.currentTarget.value)}
            autoFocus
            required
          />
          <Checkbox
            label="Read-only"
            description="Can search and view recipes, but not create or change anything."
            checked={readOnly}
            onChange={(e) => setReadOnly(e.currentTarget.checked)}
          />
          <Group justify="flex-end">
            <Button variant="subtle" color="gray" onClick={handleClose}>
              Cancel
            </Button>
            <Button
              onClick={() => createKey.mutate({ name, readOnly }, { onSuccess: (k) => setRevealedKey(k.key) })}
              loading={createKey.isPending}
              disabled={!name.trim()}
            >
              Create key
            </Button>
          </Group>
        </Stack>
      )}
    </Modal>
  )
}

function ApiKeysSection() {
  const { data: keys } = useApiKeys()
  const deleteKey = useDeleteApiKey()
  const [open, setOpen] = useState(false)

  return (
    <Stack gap="md">
      <Group justify="space-between">
        <div>
          <Title order={3}>API keys</Title>
          <Text size="sm" c="dimmed">
            For agents (Claude, Codex) and scripts. Agents with a read-write key can create and edit recipes.
          </Text>
        </div>
        <Button onClick={() => setOpen(true)}>New API key</Button>
      </Group>
      {keys && keys.length > 0 ? (
        <Table verticalSpacing="sm">
          <Table.Thead>
            <Table.Tr>
              <Table.Th>Name</Table.Th>
              <Table.Th>Scope</Table.Th>
              <Table.Th>Created</Table.Th>
              <Table.Th>Last used</Table.Th>
              <Table.Th />
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {keys.map((key) => (
              <Table.Tr key={key.id}>
                <Table.Td>{key.name}</Table.Td>
                <Table.Td>
                  <Badge size="sm" variant="light" color={key.scopes.includes('write') ? 'orange' : 'gray'}>
                    {key.scopes.includes('write') ? 'Read-write' : 'Read-only'}
                  </Badge>
                </Table.Td>
                <Table.Td>{formatDate(key.createdAt)}</Table.Td>
                <Table.Td>{formatDate(key.lastUsedAt)}</Table.Td>
                <Table.Td>
                  <ActionIcon
                    variant="subtle"
                    color="red"
                    aria-label="Revoke key"
                    onClick={() => deleteKey.mutate(key.id)}
                    loading={deleteKey.isPending && deleteKey.variables === key.id}
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
          No API keys yet.
        </Text>
      )}
      <NewApiKeyModal opened={open} onClose={() => setOpen(false)} />
    </Stack>
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
          Apps like ChatGPT that you approved through the sign-in page. Disconnecting revokes their access immediately.
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
  const publicBase = base.startsWith('https://') ? base : 'https://<your-public-host>'
  const claudeCode = `claude mcp add --transport http cookira ${base}/mcp \\\n  --header "Authorization: Bearer <your key>"`
  const codex = `# ~/.codex/config.toml\n[mcp_servers.cookira]\nurl = "${base}/mcp"\nbearer_token_env_var = "COOKIRA_API_KEY"\n\n# then set COOKIRA_API_KEY=<your key> in the environment Codex runs in`
  const claudeDesktop = `{\n  "mcpServers": {\n    "cookira": {\n      "command": "npx",\n      "args": [\n        "-y", "mcp-remote", "${base}/mcp",\n        "--header", "Authorization:Bearer \${COOKIRA_API_KEY}"\n      ],\n      "env": { "COOKIRA_API_KEY": "<your key>" }\n    }\n  }\n}`

  return (
    <Stack gap="md">
      <div>
        <Title order={3}>Connect an agent</Title>
        <Text size="sm" c="dimmed">
          Create an API key, then point your client at <Code>{base}/mcp</Code> with the key as a Bearer token.
        </Text>
      </div>
      <Tabs defaultValue="desktop" keepMounted={false}>
        <Tabs.List>
          <Tabs.Tab value="desktop">Claude desktop</Tabs.Tab>
          <Tabs.Tab value="code">Claude Code</Tabs.Tab>
          <Tabs.Tab value="codex">Codex</Tabs.Tab>
          <Tabs.Tab value="chatgpt">ChatGPT</Tabs.Tab>
        </Tabs.List>
        <Tabs.Panel value="desktop" pt="sm">
          <Stack gap="xs">
            <Text size="sm">
              Add to <Code>claude_desktop_config.json</Code> (Settings → Developer → Edit Config). This uses the
              <Code>mcp-remote</Code> bridge so the Bearer header can be sent.
            </Text>
            <CopyCode value={claudeDesktop} />
          </Stack>
        </Tabs.Panel>
        <Tabs.Panel value="code" pt="sm">
          <CopyCode value={claudeCode} />
        </Tabs.Panel>
        <Tabs.Panel value="codex" pt="sm">
          <CopyCode value={codex} />
        </Tabs.Panel>
        <Tabs.Panel value="chatgpt" pt="sm">
          <Stack gap="sm">
            <Text size="sm">
              ChatGPT connects with OAuth instead of an API key: you sign in to Cookira and approve the connection, then
              choose whether it may only read recipes or also create and edit them. Connected apps appear under{' '}
              <b>Connected apps</b> above, where you can disconnect them.
            </Text>
            {publicBase.startsWith('https://<') && (
              <Alert color="yellow" title="Needs a public HTTPS address">
                ChatGPT cannot reach <Code>localhost</Code>. Set <Code>PUBLIC_URL</Code> to the HTTPS address Cookira is
                reachable at from the internet (for a private server, OpenAI offers a Secure MCP Tunnel), then open
                Settings from that address.
              </Alert>
            )}
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
              . The ChatGPT desktop app uses the same connectors as the web app. claude.ai custom connectors work the
              same way.
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
      <ApiKeysSection />
      <ConnectedAppsSection />
      <ConnectSection />
      <MembersSection />
      <AccountSection />
    </Stack>
  )
}
