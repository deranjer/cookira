import { Button, Paper, PasswordInput, Stack, Text, TextInput, Title } from '@mantine/core'
import { useForm } from '@mantine/form'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { ApiError } from '../lib/api/client'
import { useAcceptInvite, useInviteInfo } from '../lib/api/hooks'
import { AuthFrame } from '../components/AuthFrame'

export function AcceptInvite() {
  const { token = '' } = useParams()
  const navigate = useNavigate()
  const info = useInviteInfo(token)
  const accept = useAcceptInvite(token)
  const form = useForm({
    initialValues: { name: '', email: '', password: '', confirmPassword: '' },
    validate: {
      password: (v) => (v.length < 8 ? 'Password must be at least 8 characters' : null),
      confirmPassword: (v, values) => (v !== values.password ? 'Passwords do not match' : null),
    },
  })

  const lockedEmail = info.data?.email
  const error =
    accept.error instanceof ApiError ? accept.error.message : accept.isError ? 'Could not create account' : null

  if (info.isLoading) return <AuthFrame>{null}</AuthFrame>

  if (info.isError || !info.data) {
    return (
      <AuthFrame>
        <Paper w={420} maw="100%" p="xl" withBorder>
          <Stack gap="sm">
            <Title order={2}>Invite not valid</Title>
            <Text size="sm" c="dimmed">
              This invite link has expired, was already used, or was revoked. Ask whoever invited you for a new one.
            </Text>
            <Button component={Link} to="/login" variant="light">
              Go to sign in
            </Button>
          </Stack>
        </Paper>
      </AuthFrame>
    )
  }

  return (
    <AuthFrame>
      <Paper w={420} maw="100%" p="xl" withBorder>
        <form
          onSubmit={form.onSubmit((v) =>
            accept.mutate(
              { name: v.name, email: lockedEmail ? undefined : v.email, password: v.password },
              { onSuccess: () => navigate('/') },
            ),
          )}
        >
          <Stack gap="md">
            <div>
              <Title order={2}>Join Cookira</Title>
              <Text size="sm" c="dimmed" mt={4}>
                You&apos;ve been invited. Choose your details to create your account.
              </Text>
            </div>
            <TextInput label="Name" required {...form.getInputProps('name')} />
            {lockedEmail ? (
              <TextInput label="Email" value={lockedEmail} readOnly />
            ) : (
              <TextInput label="Email" required {...form.getInputProps('email')} />
            )}
            <PasswordInput label="Password" required {...form.getInputProps('password')} />
            <PasswordInput label="Confirm password" required {...form.getInputProps('confirmPassword')} />
            {error && (
              <Text c="red" size="sm">
                {error}
              </Text>
            )}
            <Button type="submit" fullWidth loading={accept.isPending}>
              Create account
            </Button>
          </Stack>
        </form>
      </Paper>
    </AuthFrame>
  )
}
