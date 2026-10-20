import { useEffect, useMemo, useState } from 'react'
import { Alert, Button, Group, Loader, Stack, Text } from '@mantine/core'
import { Dropzone, type FileRejection } from '@mantine/dropzone'
import { IconPhoto, IconTrash, IconUpload, IconX } from '@tabler/icons-react'
import { recipesApi } from '../lib/api/client'
import { useRemoveImage, useUploadImage } from '../lib/api/hooks'
import type { Recipe } from '../lib/api/types'

const ACCEPT = ['image/jpeg', 'image/png', 'image/webp']
const MAX_BYTES = 10 * 1024 * 1024

interface Props {
  /** The saved recipe being edited; omit when creating a new one. */
  recipe?: Recipe
  /** New-recipe mode: the chosen file is held here and uploaded after the recipe is created. */
  pendingFile: File | null
  onPendingFile: (file: File | null) => void
}

/**
 * Photo picker for the recipe editor. For an existing recipe a dropped file
 * uploads straight away (the server resizes it); for a new recipe it is held
 * until the recipe has been created.
 */
export function PhotoField({ recipe, pendingFile, onPendingFile }: Props) {
  const upload = useUploadImage(recipe?.id ?? '')
  const remove = useRemoveImage(recipe?.id ?? '')
  const [error, setError] = useState<string | null>(null)

  const pendingUrl = useMemo(() => (pendingFile ? URL.createObjectURL(pendingFile) : null), [pendingFile])
  useEffect(() => () => { if (pendingUrl) URL.revokeObjectURL(pendingUrl) }, [pendingUrl])

  const savedUrl = recipe?.hasImage ? recipesApi.imageUrl(recipe.id, recipe.updatedAt) : null
  const preview = pendingUrl ?? savedUrl

  function onDrop(files: File[]) {
    setError(null)
    const file = files[0]
    if (!file) return
    if (recipe) {
      upload.mutate(file, { onError: (e) => setError(e.message) })
    } else {
      onPendingFile(file)
    }
  }

  function onReject(rejections: FileRejection[]) {
    const code = rejections[0]?.errors[0]?.code
    setError(
      code === 'file-too-large'
        ? 'That photo is larger than 10 MB.'
        : code === 'file-invalid-type'
          ? 'Photos must be JPEG, PNG or WebP.'
          : 'That file could not be used.',
    )
  }

  const busy = upload.isPending || remove.isPending

  return (
    <Stack gap="xs">
      <Text size="sm" fw={500}>
        Photo
      </Text>
      {preview && (
        <img
          src={preview}
          alt=""
          style={{ width: '100%', maxHeight: 260, objectFit: 'cover', borderRadius: 'var(--mantine-radius-md)' }}
        />
      )}
      <Dropzone
        onDrop={onDrop}
        onReject={onReject}
        accept={ACCEPT}
        maxSize={MAX_BYTES}
        maxFiles={1}
        multiple={false}
        loading={busy}
        loaderProps={{ color: 'orange' }}
      >
        <Group justify="center" gap="md" mih={preview ? 70 : 110} style={{ pointerEvents: 'none' }}>
          <Dropzone.Accept>
            <IconUpload size={32} color="var(--mantine-color-orange-6)" />
          </Dropzone.Accept>
          <Dropzone.Reject>
            <IconX size={32} color="var(--mantine-color-red-6)" />
          </Dropzone.Reject>
          <Dropzone.Idle>
            <IconPhoto size={32} color="var(--mantine-color-dimmed)" />
          </Dropzone.Idle>
          <div>
            <Text size="md">{preview ? 'Drop a new photo here, or click to replace it' : 'Drop a photo here, or click to choose one'}</Text>
            <Text size="xs" c="dimmed">
              JPEG, PNG or WebP up to 10 MB. It is resized automatically.
            </Text>
          </div>
        </Group>
      </Dropzone>
      {error && <Alert color="red">{error}</Alert>}
      {preview && (
        <Group>
          <Button
            variant="subtle"
            color="red"
            size="xs"
            leftSection={remove.isPending ? <Loader size={12} color="red" /> : <IconTrash size={14} />}
            disabled={busy}
            onClick={() => {
              setError(null)
              if (pendingFile) onPendingFile(null)
              else remove.mutate(undefined, { onError: (e) => setError(e.message) })
            }}
          >
            Remove photo
          </Button>
        </Group>
      )}
    </Stack>
  )
}
