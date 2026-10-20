import { Alert, Button, Center, Group, Loader, Text } from '@mantine/core'
import { IconPrinter } from '@tabler/icons-react'
import { useParams } from 'react-router-dom'
import { RecipeView } from '../components/RecipeView'
import { sharesApi } from '../lib/api/client'
import { useSharedRecipe } from '../lib/api/hooks'

/** Public, login-free, read-only page behind a share link. */
export function SharedRecipe() {
  const { token } = useParams()
  const { data: recipe, isLoading, isError } = useSharedRecipe(token)

  if (isLoading) {
    return (
      <Center h="100vh">
        <Loader color="orange" />
      </Center>
    )
  }
  if (isError || !recipe || !token) {
    return (
      <Center h="100vh" p="md">
        <Alert color="gray" title="This link is no longer available">
          The recipe was unshared, the link expired, or the address is wrong.
        </Alert>
      </Center>
    )
  }

  return (
    <div style={{ padding: 16 }}>
      <Group justify="space-between" maw={860} mx="auto" mb="md" className="no-print">
        <Group gap="xs">
          <img src="/favicon.svg" width={24} height={24} alt="" />
          <Text fw={700} ff="Georgia, serif">
            Cookira
          </Text>
        </Group>
        <Button variant="light" size="xs" leftSection={<IconPrinter size={14} />} onClick={() => window.print()}>
          Print
        </Button>
      </Group>
      <RecipeView recipe={recipe} imageUrl={recipe.hasImage ? sharesApi.publicImageUrl(token) : undefined} />
    </div>
  )
}
