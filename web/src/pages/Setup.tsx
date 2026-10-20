import { Button, Paper, PasswordInput, Stack, Text, TextInput, Title } from '@mantine/core'
import { useForm } from '@mantine/form'
import { useNavigate } from 'react-router-dom'
import { ApiError } from '../lib/api/client'
import { useCompleteSetup } from '../lib/api/hooks'
import { AuthFrame } from '../components/AuthFrame'

export function Setup() {
  const navigate = useNavigate()
  const complete = useCompleteSetup()
  const form = useForm({
    initialValues: { name: '', email: '', password: '', confirmPassword: '' },
    validate: {
      confirmPassword: (value, values) => (value !== values.password ? 'Passwords do not match' : null),
      password: (value) => (value.length < 8 ? 'Password must be at least 8 characters' : null),
    },
  })

  const error = complete.error instanceof ApiError ? complete.error.message : complete.isError ? 'Setup failed' : null

  return (
    <AuthFrame>
      <Paper w={420} maw="100%" p="xl" withBorder>
        <form
          onSubmit={form.onSubmit((v) =>
            complete.mutate({ name: v.name, email: v.email, password: v.password }, { onSuccess: () => navigate('/') }),
          )}
        >
          <Stack gap="md">
            <div>
              <Title order={2}>Welcome to Cookira</Title>
              <Text size="sm" c="dimmed" mt={4}>
                Create the admin account to finish setting up.
              </Text>
            </div>
            <TextInput label="Name" required {...form.getInputProps('name')} />
            <TextInput label="Email" required {...form.getInputProps('email')} />
            <PasswordInput label="Password" required {...form.getInputProps('password')} />
            <PasswordInput label="Confirm password" required {...form.getInputProps('confirmPassword')} />
            {error && (
              <Text c="red" size="sm">
                {error}
              </Text>
            )}
            <Button type="submit" fullWidth loading={complete.isPending}>
              Create admin account
            </Button>
          </Stack>
        </form>
      </Paper>
    </AuthFrame>
  )
}
