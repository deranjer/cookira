import {
  ActionIcon,
  AppShell as MantineShell,
  Button,
  Group,
  Menu,
  Text,
  useComputedColorScheme,
  useMantineColorScheme,
} from '@mantine/core'
import { IconLogout, IconMoon, IconPlus, IconSettings, IconSun } from '@tabler/icons-react'
import { Link, Outlet, useNavigate } from 'react-router-dom'
import { useLogout, useMe } from '../lib/api/hooks'

export function AppShell() {
  const navigate = useNavigate()
  const { data: me } = useMe()
  const logout = useLogout()
  const { setColorScheme } = useMantineColorScheme()
  const scheme = useComputedColorScheme('light')

  return (
    <MantineShell header={{ height: 58 }} padding="md">
      <MantineShell.Header className="no-print">
        <Group h="100%" px="md" justify="space-between" wrap="nowrap">
          <Link to="/" style={{ textDecoration: 'none', color: 'inherit' }}>
            <Group gap="xs">
              <img src="/favicon.svg" width={28} height={28} alt="" />
              <Text fw={700} size="lg" ff="Georgia, serif">
                Cookira
              </Text>
            </Group>
          </Link>
          <Group gap="xs" wrap="nowrap">
            <Button component={Link} to="/recipes/new" leftSection={<IconPlus size={16} />} size="sm">
              New recipe
            </Button>
            <ActionIcon
              variant="subtle"
              color="gray"
              aria-label="Toggle color scheme"
              onClick={() => setColorScheme(scheme === 'dark' ? 'light' : 'dark')}
            >
              {scheme === 'dark' ? <IconSun size={18} /> : <IconMoon size={18} />}
            </ActionIcon>
            <Menu position="bottom-end" withArrow>
              <Menu.Target>
                <Button variant="subtle" color="gray" size="sm">
                  {me?.name}
                </Button>
              </Menu.Target>
              <Menu.Dropdown>
                <Menu.Item leftSection={<IconSettings size={14} />} component={Link} to="/settings">
                  Settings
                </Menu.Item>
                <Menu.Item
                  leftSection={<IconLogout size={14} />}
                  onClick={() => logout.mutate(undefined, { onSuccess: () => navigate('/login') })}
                >
                  Sign out
                </Menu.Item>
              </Menu.Dropdown>
            </Menu>
          </Group>
        </Group>
      </MantineShell.Header>
      <MantineShell.Main className="print-only-wrap">
        <Outlet />
      </MantineShell.Main>
    </MantineShell>
  )
}
