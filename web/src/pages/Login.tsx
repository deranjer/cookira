import { Button, Paper, PasswordInput, Stack, Text, TextInput, Title } from '@mantine/core'
import { useForm } from '@mantine/form'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { ApiError } from '../lib/api/client'
import { useLogin } from '../lib/api/hooks'
import { AuthFrame } from '../components/AuthFrame'

export function Login() {
  const navigate = useNavigate()
  const [params] = useSearchParams()
  // Only same-site paths, so a crafted link cannot bounce the user elsewhere.
  const next = params.get('next')
  const target = next && next.startsWith('/') && !next.startsWith('//') ? next : '/'
  const login = useLogin()
  const form = useForm({ initialValues: { email: '', password: '' } })

  const error = login.error instanceof ApiError ? login.error.message : login.isError ? 'Login failed' : null

  return (
    <AuthFrame>
      <Paper w={420} maw="100%" p="xl" withBorder>
        <form onSubmit={form.onSubmit((v) => login.mutate(v, { onSuccess: () => navigate(target) }))}>
          <Stack gap="md">
            <Title order={2}>Sign in</Title>
            <TextInput label="Email" required {...form.getInputProps('email')} />
            <PasswordInput label="Password" required {...form.getInputProps('password')} />
            {error && (
              <Text c="red" size="sm">
                {error}
              </Text>
            )}
            <Button type="submit" fullWidth loading={login.isPending}>
              Sign in
            </Button>
          </Stack>
        </form>
      </Paper>
    </AuthFrame>
  )
}
