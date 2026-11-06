import { ActionIcon, Button, Group, Paper, Stack, Text } from '@mantine/core'
import { IconStopwatch, IconX } from '@tabler/icons-react'
import { useCallback, useEffect, useRef, useState } from 'react'
import { clock, findDurations } from '../lib/cook'

export interface CookTimer {
  id: number
  label: string
  endAt: number
  fired: boolean
}

/** Three short beeps, plus a notification and a buzz where the browser allows them. */
function ring(label: string, ctx: AudioContext | null) {
  try {
    if (ctx) {
      for (let i = 0; i < 3; i++) {
        const osc = ctx.createOscillator()
        const gain = ctx.createGain()
        osc.frequency.value = 880
        gain.gain.value = 0.2
        osc.connect(gain).connect(ctx.destination)
        osc.start(ctx.currentTime + i * 0.5)
        osc.stop(ctx.currentTime + i * 0.5 + 0.25)
      }
    }
    navigator.vibrate?.([300, 150, 300])
    if ('Notification' in window && Notification.permission === 'granted') {
      new Notification('Timer done', { body: label })
    }
  } catch {
    // Sound and notifications are best effort; the dock still shows "Time's up".
  }
}

/** Kitchen timers for one recipe page: in-page countdowns that beep when they finish. */
export function useCookTimers(recipeTitle: string) {
  const [timers, setTimers] = useState<CookTimer[]>([])
  const [now, setNow] = useState(() => Date.now())
  const nextId = useRef(1)
  const audio = useRef<AudioContext | null>(null)

  const start = useCallback((label: string, seconds: number) => {
    // Created inside the click so the browser lets it make sound later.
    try {
      audio.current ??= new AudioContext()
      void audio.current.resume()
      if ('Notification' in window && Notification.permission === 'default') void Notification.requestPermission()
    } catch {
      // No audio: the timer still counts down.
    }
    setNow(Date.now())
    setTimers((ts) => [...ts, { id: nextId.current++, label, endAt: Date.now() + seconds * 1000, fired: false }])
  }, [])

  const cancel = useCallback((id: number) => setTimers((ts) => ts.filter((t) => t.id !== id)), [])

  useEffect(() => {
    if (timers.length === 0) return
    const t = setInterval(() => setNow(Date.now()), 500)
    return () => clearInterval(t)
  }, [timers.length])

  useEffect(() => {
    const due = timers.filter((t) => !t.fired && t.endAt <= now)
    if (due.length === 0) return
    setTimers((ts) => ts.map((t) => (due.some((d) => d.id === t.id) ? { ...t, fired: true } : t)))
    due.forEach((t) => ring(t.label, audio.current))
  }, [now, timers])

  // The soonest running timer in the tab title, so it can be watched from another tab.
  useEffect(() => {
    const running = timers.filter((t) => !t.fired).sort((a, b) => a.endAt - b.endAt)[0]
    if (!running) return
    const original = document.title
    document.title = `${clock((running.endAt - now) / 1000)} · ${recipeTitle}`
    return () => {
      document.title = original
    }
  }, [timers, now, recipeTitle])

  return { timers, now, start, cancel }
}

export type CookTimersApi = ReturnType<typeof useCookTimers>

/** Floating list of running timers; sits above the cook-mode overlay. */
export function TimerDock({ api }: { api: CookTimersApi }) {
  if (api.timers.length === 0) return null
  return (
    <Stack gap="xs" className="no-print" style={{ position: 'fixed', right: 16, bottom: 16, zIndex: 1000, width: 280 }}>
      {api.timers.map((t) => {
        const left = (t.endAt - api.now) / 1000
        return (
          <Paper key={t.id} withBorder shadow="md" p="sm" bg={t.fired ? 'red.1' : undefined} style={t.fired ? { borderColor: 'var(--mantine-color-red-6)' } : undefined}>
            <Group justify="space-between" wrap="nowrap" align="flex-start">
              <div>
                <Text fz={28} fw={700} lh={1.1} c={t.fired ? 'red.9' : undefined}>
                  {t.fired ? "Time's up" : clock(Math.ceil(left))}
                </Text>
                <Text size="sm" c={t.fired ? 'red.9' : 'dimmed'}>
                  {t.label}
                </Text>
              </div>
              <ActionIcon variant="subtle" color="gray" aria-label={t.fired ? 'Dismiss timer' : 'Cancel timer'} onClick={() => api.cancel(t.id)}>
                <IconX size={18} />
              </ActionIcon>
            </Group>
          </Paper>
        )
      })}
    </Stack>
  )
}

/** One "start timer" button per time mentioned in [text]. */
export function DurationButtons({
  text,
  stepLabel,
  api,
  size = 'sm',
}: {
  text: string
  stepLabel: string
  api: CookTimersApi
  size?: 'sm' | 'md' | 'lg'
}) {
  const found = findDurations(text)
  if (found.length === 0) return null
  return (
    <Group gap="xs" className="no-print">
      {found.map((d) => (
        <Button
          key={d.text}
          size={size}
          variant="light"
          leftSection={<IconStopwatch size={size === 'sm' ? 16 : 20} />}
          title={d.isRange ? 'Starts at the low end of the range' : undefined}
          onClick={() => api.start(`${stepLabel}: ${d.isRange ? `${d.text} (low end)` : d.text}`, d.seconds)}
        >
          {d.text}
        </Button>
      ))}
    </Group>
  )
}
