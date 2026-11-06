import type { CSSProperties } from 'react'

/** How a recipe photo is framed: a focal point (0..1 from left/top) and a zoom (1..4). */
export interface Framing {
  imageFocusX: number
  imageFocusY: number
  imageZoom: number
}

export const DEFAULT_FRAMING: Framing = { imageFocusX: 0.5, imageFocusY: 0.5, imageZoom: 1 }
export const MAX_ZOOM = 4

/** Header and list-card frame shapes (width / height), shared by the views and the editor previews. */
export const HEADER_ASPECT = 16 / 7
export const CARD_ASPECT = 300 / 160

export const isDefaultFraming = (f: Framing) =>
  f.imageFocusX === DEFAULT_FRAMING.imageFocusX &&
  f.imageFocusY === DEFAULT_FRAMING.imageFocusY &&
  f.imageZoom === DEFAULT_FRAMING.imageZoom

/** Older servers send no framing at all, so fall back to centred. */
export function framingOf(r: Partial<Framing>): Framing {
  return {
    imageFocusX: r.imageFocusX ?? DEFAULT_FRAMING.imageFocusX,
    imageFocusY: r.imageFocusY ?? DEFAULT_FRAMING.imageFocusY,
    imageZoom: r.imageZoom || DEFAULT_FRAMING.imageZoom,
  }
}

interface Props {
  src: string
  framing: Framing
  /** Frame shape as width / height. Omit when `height` is set. */
  aspect?: number
  height?: number
  maxHeight?: number
  radius?: string
  loading?: 'lazy' | 'eager'
  style?: CSSProperties
}

/**
 * A photo cropped to its frame with `object-fit: cover`, positioned on the
 * focal point and zoomed around it. The stored image is never cut, so the
 * framing can be changed at any time.
 */
export function FramedImage({ src, framing, aspect, height, maxHeight, radius, loading, style }: Props) {
  const origin = `${framing.imageFocusX * 100}% ${framing.imageFocusY * 100}%`
  return (
    <div
      style={{
        width: '100%',
        height,
        maxHeight,
        aspectRatio: height ? undefined : aspect,
        overflow: 'hidden',
        borderRadius: radius,
        background: 'var(--mantine-color-orange-light)',
        ...style,
      }}
    >
      <img
        src={src}
        alt=""
        loading={loading}
        style={{
          display: 'block',
          width: '100%',
          height: '100%',
          objectFit: 'cover',
          objectPosition: origin,
          transformOrigin: origin,
          transform: framing.imageZoom > 1 ? `scale(${framing.imageZoom})` : undefined,
        }}
      />
    </div>
  )
}
