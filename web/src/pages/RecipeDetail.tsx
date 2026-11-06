import { Alert, Button, Center, Group, Loader, Menu } from '@mantine/core'
import { useDisclosure } from '@mantine/hooks'
import {
  IconDots,
  IconEdit,
  IconEye,
  IconEyeOff,
  IconHistory,
  IconPrinter,
  IconShare,
  IconTrash,
} from '@tabler/icons-react'
import { useNavigate, useParams } from 'react-router-dom'
import { HistoryDrawer } from '../components/HistoryDrawer'
import { RecipeView } from '../components/RecipeView'
import { ShareModal } from '../components/ShareModal'
import { recipesApi } from '../lib/api/client'
import { useDeleteRecipe, useRecipe, useUpdateRecipe } from '../lib/api/hooks'

export function RecipeDetail() {
  const { ref = '' } = useParams()
  const navigate = useNavigate()
  const { data: recipe, isLoading, isError } = useRecipe(ref)
  const update = useUpdateRecipe(ref)
  const del = useDeleteRecipe()
  const [shareOpen, share] = useDisclosure(false)
  const [historyOpen, history] = useDisclosure(false)

  if (isLoading) {
    return (
      <Center h={300}>
        <Loader color="orange" />
      </Center>
    )
  }
  if (isError || !recipe) {
    return <Alert color="red">Recipe not found.</Alert>
  }

  const published = recipe.status === 'published'

  return (
    <>
      <Group justify="flex-end" mb="md" maw={1080} mx="auto" className="no-print">
        <Button
          variant="light"
          leftSection={published ? <IconEyeOff size={16} /> : <IconEye size={16} />}
          loading={update.isPending}
          onClick={() => update.mutate({ status: published ? 'draft' : 'published' })}
        >
          {published ? 'Unpublish' : 'Publish'}
        </Button>
        <Button variant="default" leftSection={<IconEdit size={16} />} onClick={() => navigate(`/recipes/${recipe.slug}/edit`)}>
          Edit
        </Button>
        <Button variant="default" leftSection={<IconShare size={16} />} onClick={share.open}>
          Share
        </Button>
        <Menu position="bottom-end" withArrow>
          <Menu.Target>
            <Button variant="default" px={8} aria-label="More">
              <IconDots size={16} />
            </Button>
          </Menu.Target>
          <Menu.Dropdown>
            <Menu.Item leftSection={<IconPrinter size={14} />} onClick={() => window.print()}>
              Print
            </Menu.Item>
            <Menu.Item leftSection={<IconHistory size={14} />} onClick={history.open}>
              History
            </Menu.Item>
            <Menu.Divider />
            <Menu.Item
              color="red"
              leftSection={<IconTrash size={14} />}
              onClick={() => {
                if (window.confirm(`Delete "${recipe.title}"? You can restore it from history via its id.`)) {
                  del.mutate(recipe.id, { onSuccess: () => navigate('/') })
                }
              }}
            >
              Delete
            </Menu.Item>
          </Menu.Dropdown>
        </Menu>
      </Group>
      <RecipeView recipe={recipe} imageUrl={recipe.hasImage ? recipesApi.imageUrl(recipe.id, recipe.updatedAt) : undefined} />
      <ShareModal recipeRef={recipe.id} opened={shareOpen} onClose={share.close} />
      <HistoryDrawer recipeRef={recipe.id} opened={historyOpen} onClose={history.close} />
    </>
  )
}
