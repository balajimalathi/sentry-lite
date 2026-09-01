import { useMemo, useState } from 'react'
import { Button } from '@/components/ui/button'
import type { Frame } from '@/api'
import { Badge } from '@/components/ui/badge'

function frameLine(f: Frame) {
  const file = f.filename || f.abs_path || f.module || '?'
  const loc = f.lineno ? `:${f.lineno}` : ''
  const fn = f.function ? ` in ${f.function}` : ''
  return `${file}${loc}${fn}`
}

export function StackTrace({ frames }: { frames: Frame[] }) {
  const [showLibrary, setShowLibrary] = useState(false)
  const reversed = useMemo(() => [...frames].reverse(), [frames])
  const inAppCount = reversed.filter((f) => f.in_app).length
  const visible = showLibrary ? reversed : reversed.filter((f) => f.in_app)
  const collapsed = reversed.length - visible.length

  if (frames.length === 0) {
    return (
      <p className="text-sm text-muted-foreground">No stack frames on this event.</p>
    )
  }

  const text = reversed.map(frameLine).join('\n')

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center gap-2">
        {inAppCount > 0 && inAppCount < reversed.length ? (
          <Button
            type="button"
            size="sm"
            variant="outline"
            onClick={() => setShowLibrary((v) => !v)}
          >
            {showLibrary
              ? 'Hide library frames'
              : `Show ${collapsed} library frames`}
          </Button>
        ) : null}
        <Button
          type="button"
          size="sm"
          variant="outline"
          onClick={() => void navigator.clipboard.writeText(text)}
        >
          Copy
        </Button>
      </div>
      <ol className="flex list-decimal flex-col gap-2 pl-4">
        {(visible.length > 0 ? visible : reversed).map((f, i) => (
          <li
            key={`${frameLine(f)}-${i}`}
            className={
              f.in_app
                ? 'min-w-0 font-medium text-foreground'
                : 'min-w-0 text-muted-foreground'
            }
          >
            <div className="flex flex-wrap items-center gap-2 font-mono text-sm break-all">
              <span>{frameLine(f)}</span>
              {f.in_app ? (
                <Badge variant="outline" className="font-sans font-normal">
                  in-app
                </Badge>
              ) : null}
            </div>
          </li>
        ))}
      </ol>
    </div>
  )
}
