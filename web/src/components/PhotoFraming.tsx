import { useState, type KeyboardEvent, type PointerEvent } from 'react'
import { Button, Group, SimpleGrid, Slider, Stack, Text } from '@mantine/core'
import { IconFocus2, IconRestore } from '@tabler/icons-react'
import {
  CARD_ASPECT,
  DEFAULT_FRAMING,
  FramedImage,
  HEADER_ASPECT,
  MAX_ZOOM,
  isDefaultFraming,
  type Framing,
} from './FramedImage'

interface Props {
  /** Full photo, used for the original and for the header preview. */
  src: string
  /** Thumbnail, used for the browse-card preview (what the list really loads). */
  thumbSrc: string
  framing: Framing
  onChange: (f: Framing) => void
}

const clamp01 = (v: number) => Math.min(1, Math.max(0, v))

/** The part of the photo (as fractions of its width/height) that a frame of shape `frame` shows. */
function visibleRegion(f: Framing, imageAspect: number, frame: number) {
  const wide = imageAspect > frame
  const vw = (wide ? frame / imageAspect : 1) / f.imageZoom
  const vh = (wide ? 1 : imageAspect / frame) / f.imageZoom
  return { left: f.imageFocusX * (1 - vw), top: f.imageFocusY * (1 - vh), width: vw, height: vh }
}

/**
 * Photo framing editor: click or drag on the original to pick the focal point,
 * zoom with the slider, and watch both real frames (recipe header and browse
 * card) update live. The outlines on the original show what each frame keeps.
 */
export function PhotoFraming({ src, thumbSrc, framing, onChange }: Props) {
  const [imageAspect, setImageAspect] = useState(HEADER_ASPECT)
  const [dragging, setDragging] = useState(false)

  function pick(e: PointerEvent<HTMLDivElement>) {
    const rect = e.currentTarget.getBoundingClientRect()
    onChange({
      ...framing,
      imageFocusX: clamp01((e.clientX - rect.left) / rect.width),
      imageFocusY: clamp01((e.clientY - rect.top) / rect.height),
    })
  }

  function onKeyDown(e: KeyboardEvent<HTMLDivElement>) {
    const step = e.shiftKey ? 0.1 : 0.02
    const d: Record<string, [number, number]> = {
      ArrowLeft: [-step, 0],
      ArrowRight: [step, 0],
      ArrowUp: [0, -step],
      ArrowDown: [0, step],
    }
    const move = d[e.key]
    if (!move) return
    e.preventDefault()
    onChange({
      ...framing,
      imageFocusX: clamp01(framing.imageFocusX + move[0]),
      imageFocusY: clamp01(framing.imageFocusY + move[1]),
    })
  }

  const outline = (frame: number, color: string, dashed: boolean) => {
    const r = visibleRegion(framing, imageAspect, frame)
    return (
      <div
        style={{
          position: 'absolute',
          left: `${r.left * 100}%`,
          top: `${r.top * 100}%`,
          width: `${r.width * 100}%`,
          height: `${r.height * 100}%`,
          border: `2px ${dashed ? 'dashed' : 'solid'} ${color}`,
          boxShadow: dashed ? undefined : '0 0 0 9999px rgba(0, 0, 0, 0.35)',
          boxSizing: 'border-box',
          pointerEvents: 'none',
        }}
      />
    )
  }

  return (
    <Stack gap="sm">
      <Group justify="space-between">
        <Text size="sm" fw={500}>
          Framing
        </Text>
        <Button
          variant="subtle"
          size="xs"
          leftSection={<IconRestore size={14} />}
          disabled={isDefaultFraming(framing)}
          onClick={() => onChange(DEFAULT_FRAMING)}
        >
          Reset
        </Button>
      </Group>

      <div>
        <Text size="xs" c="dimmed" mb={4}>
          Original photo — click or drag to choose what stays in view. Bright area: recipe header; dashed: browse card.
        </Text>
        <div
          role="slider"
          tabIndex={0}
          aria-label="Photo focal point"
          aria-valuetext={`${Math.round(framing.imageFocusX * 100)}% across, ${Math.round(framing.imageFocusY * 100)}% down`}
          onKeyDown={onKeyDown}
          onPointerDown={(e) => {
            e.currentTarget.setPointerCapture(e.pointerId)
            setDragging(true)
            pick(e)
          }}
          onPointerMove={(e) => dragging && pick(e)}
          onPointerUp={() => setDragging(false)}
          onPointerCancel={() => setDragging(false)}
          style={{
            position: 'relative',
            overflow: 'hidden',
            width: 'fit-content',
            maxWidth: '100%',
            margin: '0 auto',
            cursor: 'crosshair',
            touchAction: 'none',
            userSelect: 'none',
            borderRadius: 'var(--mantine-radius-md)',
          }}
        >
          <img
            src={src}
            alt=""
            draggable={false}
            onLoad={(e) => setImageAspect(e.currentTarget.naturalWidth / e.currentTarget.naturalHeight || HEADER_ASPECT)}
            style={{ display: 'block', maxWidth: '100%', maxHeight: 320 }}
          />
          {outline(HEADER_ASPECT, 'var(--mantine-color-orange-4)', false)}
          {outline(CARD_ASPECT, 'white', true)}
          <IconFocus2
            size={28}
            color="white"
            style={{
              position: 'absolute',
              left: `${framing.imageFocusX * 100}%`,
              top: `${framing.imageFocusY * 100}%`,
              transform: 'translate(-50%, -50%)',
              filter: 'drop-shadow(0 0 2px rgba(0,0,0,0.8))',
              pointerEvents: 'none',
            }}
          />
        </div>
      </div>

      <div>
        <Text size="xs" c="dimmed">
          Zoom {framing.imageZoom.toFixed(1)}×
        </Text>
        <Slider
          min={1}
          max={MAX_ZOOM}
          step={0.05}
          value={framing.imageZoom}
          onChange={(v) => onChange({ ...framing, imageZoom: v })}
          label={(v) => `${v.toFixed(1)}×`}
          color="orange"
        />
      </div>

      <SimpleGrid cols={{ base: 1, sm: 5 }} spacing="md">
        <div style={{ gridColumn: 'span 3' }}>
          <Text size="xs" c="dimmed" mb={4}>
            Recipe page header
          </Text>
          <FramedImage src={src} framing={framing} aspect={HEADER_ASPECT} radius="var(--mantine-radius-md)" />
        </div>
        <div style={{ gridColumn: 'span 2' }}>
          <Text size="xs" c="dimmed" mb={4}>
            Browse card
          </Text>
          <FramedImage src={thumbSrc} framing={framing} aspect={CARD_ASPECT} radius="var(--mantine-radius-md)" />
        </div>
      </SimpleGrid>
    </Stack>
  )
}
