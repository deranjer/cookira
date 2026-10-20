import type { ReactNode } from 'react'
import { Group, Text } from '@mantine/core'

export function AuthFrame({ children }: { children: ReactNode }) {
  return (
    <div
      style={{
        minHeight: '100vh',
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        gap: 20,
        padding: 16,
      }}
    >
      <Group gap="xs">
        <img src="/favicon.svg" width={32} height={32} alt="" />
        <Text fw={700} size="xl" ff="Georgia, serif">
          Cookira
        </Text>
      </Group>
      {children}
    </div>
  )
}
