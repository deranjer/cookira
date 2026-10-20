import { useState } from 'react'
import { Alert, Button, Group, Paper, Radio, Stack, Text, Title } from '@mantine/core'
import { useSearchParams } from 'react-router-dom'
import { AuthFrame } from '../components/AuthFrame'
import { ApiError } from '../lib/api/client'
import { useMe, useOAuthAuthorizeInfo, useOAuthDecision } from '../lib/api/hooks'

// Consent screen for OAuth clients (ChatGPT, claude.ai, ...). The client sends
// the browser here with the standard authorization-request query; we reuse the
// cookie session, ask read-only vs read-write, and bounce back to the client's
// redirect URI with the result.
export function OAuthAuthorize() {
  const [params] = useSearchParams()
  const { data: me } = useMe()
  const query = params.toString()
  const { data: info, error, isLoading } = useOAuthAuthorizeInfo(query)
  const decide = useOAuthDecision()
  const [access, setAccess] = useState<'read' | 'write'>('write')

  const effective = info?.canWrite === false ? 'read' : access

  function submit(approve: boolean) {
    decide.mutate(
      {
        clientId: params.get('client_id'),
        redirectUri: params.get('redirect_uri'),
        responseType: params.get('response_type'),
        codeChallenge: params.get('code_challenge'),
        codeChallengeMethod: params.get('code_challenge_method'),
        scope: params.get('scope') ?? '',
        state: params.get('state') ?? '',
        resource: params.get('resource') ?? '',
        approve,
        write: approve && effective === 'write',
      },
      { onSuccess: (r) => window.location.assign(r.redirectUrl) },
    )
  }

  const decideError =
    decide.error instanceof ApiError ? decide.error.message : decide.isError ? 'Something went wrong' : null

  return (
    <AuthFrame>
      <Paper w={460} maw="100%" p="xl" withBorder>
        {isLoading ? (
          <Text c="dimmed">Loading…</Text>
        ) : error || !info ? (
          <Stack gap="sm">
            <Title order={3}>Can&apos;t connect</Title>
            <Alert color="red">{error instanceof ApiError ? error.message : 'Invalid authorization request.'}</Alert>
            <Text size="sm" c="dimmed">
              Start the connection again from the app you were connecting.
            </Text>
          </Stack>
        ) : (
          <Stack gap="md">
            <Title order={3}>Connect {info.clientName} to Cookira?</Title>
            <Text size="sm">
              <b>{info.clientName}</b> wants to access the recipes of <b>{me?.name}</b> ({me?.email}). After approving
              you will be sent back to <b>{info.redirectHost}</b>.
            </Text>
            <Radio.Group value={effective} onChange={(v) => setAccess(v as 'read' | 'write')} label="What may it do?">
              <Stack gap="xs" mt="xs">
                <Radio
                  value="write"
                  disabled={!info.canWrite}
                  label="Read and write"
                  description="Search and view recipes, and create or edit them (changes are recorded in each recipe's history)."
                />
                <Radio value="read" label="Read-only" description="Search and view recipes, but change nothing." />
              </Stack>
            </Radio.Group>
            <Text size="xs" c="dimmed">
              You can disconnect it any time under Settings → Connected apps.
            </Text>
            {decideError && (
              <Text c="red" size="sm">
                {decideError}
              </Text>
            )}
            <Group justify="flex-end">
              <Button variant="subtle" color="gray" onClick={() => submit(false)} disabled={decide.isPending}>
                Deny
              </Button>
              <Button onClick={() => submit(true)} loading={decide.isPending}>
                Approve
              </Button>
            </Group>
          </Stack>
        )}
      </Paper>
    </AuthFrame>
  )
}
