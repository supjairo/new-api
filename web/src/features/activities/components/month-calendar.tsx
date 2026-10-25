/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { toIntlLocale } from '@/i18n/languages'
import { cn } from '@/lib/utils'

import {
  buildMonthGrid,
  formatDayLabel,
  getWeekdayLabels,
} from '../lib/calendar'
import type { Activity } from '../types'

type MonthCalendarProps = {
  month: Date
  activities: Activity[]
  selectedDate: string | null
  onSelectDate: (date: string) => void
}

export function MonthCalendar(props: MonthCalendarProps) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const weekdays = useMemo(() => getWeekdayLabels(locale), [locale])
  const days = useMemo(
    () => buildMonthGrid(props.month, props.activities),
    [props.month, props.activities]
  )

  return (
    <div className='bg-card rounded-xl border p-3 sm:p-4'>
      <div className='text-muted-foreground grid grid-cols-7 gap-1 pb-2 text-center text-xs font-medium'>
        {weekdays.map((weekday) => (
          <span key={weekday}>{weekday}</span>
        ))}
      </div>
      <div className='grid grid-cols-7 gap-1'>
        {days.map((day) => {
          const selected = props.selectedDate === day.date
          return (
            <button
              key={day.date}
              type='button'
              onClick={() => props.onSelectDate(day.date)}
              aria-label={formatDayLabel(day.date, locale)}
              aria-pressed={selected}
              className={cn(
                'hover:bg-accent focus-visible:ring-ring flex min-h-14 flex-col items-start gap-1 rounded-lg border p-1.5 text-start transition-colors focus-visible:ring-2 focus-visible:outline-none sm:min-h-24',
                day.inMonth
                  ? 'bg-background'
                  : 'bg-muted/30 text-muted-foreground',
                day.isToday && 'border-primary/60',
                selected && 'ring-primary ring-2'
              )}
            >
              <span
                className={cn(
                  'text-xs font-medium',
                  day.isToday &&
                    'bg-primary text-primary-foreground flex size-5 items-center justify-center rounded-full'
                )}
              >
                {day.day}
              </span>
              <span className='hidden w-full flex-col gap-0.5 sm:flex'>
                {day.activities.slice(0, 2).map((activity) => (
                  <span
                    key={activity.id}
                    className='bg-muted text-foreground truncate rounded px-1 py-0.5 text-[11px]'
                  >
                    {activity.title}
                  </span>
                ))}
                {day.activities.length > 2 && (
                  <span className='text-muted-foreground text-[11px]'>
                    {t('+{{count}} more', { count: day.activities.length - 2 })}
                  </span>
                )}
              </span>
              {day.activities.length > 0 && (
                <span
                  aria-hidden='true'
                  className='bg-primary size-1.5 rounded-full sm:hidden'
                />
              )}
            </button>
          )
        })}
      </div>
    </div>
  )
}
