'use client'

import * as React from 'react'
import { format } from 'date-fns'
import { CalendarIcon } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Calendar } from '@/components/ui/calendar'
import { Field, FieldLabel } from '@/components/ui/field'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { cn } from '@/lib/utils'

const HOURS = Array.from({ length: 24 }, (_, i) =>
  String(i).padStart(2, '0')
)
const MINUTES = Array.from({ length: 12 }, (_, i) =>
  String(i * 5).padStart(2, '0')
)

function parseRFC3339(value: string): Date | undefined {
  if (!value) return undefined
  const d = new Date(value)
  return Number.isNaN(d.getTime()) ? undefined : d
}

function toRFC3339(date: Date): string {
  return date.toISOString()
}

type DateTimePickerProps = {
  label: string
  value: string
  onChange: (rfc3339: string) => void
  className?: string
}

export function DateTimePicker({
  label,
  value,
  onChange,
  className,
}: DateTimePickerProps) {
  const selected = parseRFC3339(value)
  const [open, setOpen] = React.useState(false)
  const hour = selected ? format(selected, 'HH') : '00'
  const minute = selected
    ? String(Math.floor(selected.getMinutes() / 5) * 5).padStart(2, '0')
    : '00'

  function applyDate(day: Date | undefined) {
    if (!day) {
      onChange('')
      return
    }
    const next = new Date(day)
    next.setHours(Number(hour), Number(minute), 0, 0)
    onChange(toRFC3339(next))
  }

  function applyTime(nextHour: string, nextMinute: string) {
    const base = selected ?? new Date()
    const next = new Date(base)
    next.setHours(Number(nextHour), Number(nextMinute), 0, 0)
    onChange(toRFC3339(next))
  }

  const hourItems = HOURS.map((h) => ({ label: h, value: h }))
  const minuteItems = MINUTES.map((m) => ({ label: m, value: m }))

  return (
    <Field className={className}>
      <FieldLabel>{label}</FieldLabel>
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger
          render={
            <Button
              type="button"
              variant="outline"
              className={cn(
                'w-full justify-start font-normal',
                !selected && 'text-muted-foreground'
              )}
            />
          }
        >
          <CalendarIcon data-icon="inline-start" />
          {selected ? format(selected, 'MMM d, yyyy HH:mm') : 'Pick date & time'}
        </PopoverTrigger>
        <PopoverContent align="start" className="w-auto">
          <Calendar
            mode="single"
            selected={selected}
            onSelect={applyDate}
            captionLayout="dropdown"
          />
          <div className="flex items-center gap-2 border-t pt-2">
            <Select
              items={hourItems}
              value={hour}
              onValueChange={(v) => applyTime(String(v ?? '00'), minute)}
            >
              <SelectTrigger className="w-[4.5rem]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  {hourItems.map((item) => (
                    <SelectItem key={item.value} value={item.value}>
                      {item.label}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
            <span className="text-muted-foreground">:</span>
            <Select
              items={minuteItems}
              value={minute}
              onValueChange={(v) => applyTime(hour, String(v ?? '00'))}
            >
              <SelectTrigger className="w-[4.5rem]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  {minuteItems.map((item) => (
                    <SelectItem key={item.value} value={item.value}>
                      {item.label}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
            {value && (
              <Button
                type="button"
                variant="ghost"
                size="sm"
                onClick={() => {
                  onChange('')
                  setOpen(false)
                }}
              >
                Clear
              </Button>
            )}
          </div>
        </PopoverContent>
      </Popover>
    </Field>
  )
}
