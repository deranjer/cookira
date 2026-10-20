import { createTheme } from '@mantine/core'

// Mantine's built-in orange ramp is the accent; everything else stays on the
// default neutrals so light and dark both work without a custom palette.
export const theme = createTheme({
  primaryColor: 'orange',
  defaultRadius: 'md',
  fontFamily: "-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif",
  headings: { fontFamily: "Georgia, 'Times New Roman', serif", fontWeight: '600' },
})

export function formatMinutes(total: number): string {
  if (total <= 0) return ''
  const h = Math.floor(total / 60)
  const m = total % 60
  if (h === 0) return `${m} min`
  return m === 0 ? `${h} hr` : `${h} hr ${m} min`
}
