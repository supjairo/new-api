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
import dayjs from '@/lib/dayjs'

import type { Activity } from '../types'

export type CalendarDay = {
  date: string
  day: number
  inMonth: boolean
  isToday: boolean
  activities: Activity[]
}

export const DATE_FORMAT = 'YYYY-MM-DD'
export const MONTH_FORMAT = 'YYYY-MM'
export const GRID_CELL_COUNT = 42

export function formatDate(date: Date): string {
  return dayjs(date).format(DATE_FORMAT)
}

export function formatMonth(date: Date): string {
  return dayjs(date).format(MONTH_FORMAT)
}

export function groupActivitiesByDate(
  activities: Activity[]
): Map<string, Activity[]> {
  const grouped = new Map<string, Activity[]>()
  activities.forEach((activity) => {
    const existing = grouped.get(activity.date)
    if (existing) {
      existing.push(activity)
      return
    }
    grouped.set(activity.date, [activity])
  })
  return grouped
}

/**
 * Build a Monday-first month grid with a fixed six-week height so the
 * calendar does not jump between months.
 */
export function buildMonthGrid(
  month: Date,
  activities: Activity[],
  today: Date = new Date()
): CalendarDay[] {
  const monthStart = dayjs(month).startOf('month')
  const gridStart = monthStart.subtract((monthStart.day() + 6) % 7, 'day')
  const todayKey = dayjs(today).format(DATE_FORMAT)
  const grouped = groupActivitiesByDate(activities)

  return Array.from({ length: GRID_CELL_COUNT }, (_, index) => {
    const cell = gridStart.add(index, 'day')
    const key = cell.format(DATE_FORMAT)
    return {
      date: key,
      day: cell.date(),
      inMonth: cell.month() === monthStart.month(),
      isToday: key === todayKey,
      activities: grouped.get(key) ?? [],
    }
  })
}

export type ActivityColor = {
  chip: string
  dot: string
}

const ACTIVITY_COLORS: ActivityColor[] = [
  {
    chip: 'bg-sky-500/15 text-sky-700 dark:text-sky-300',
    dot: 'bg-sky-500',
  },
  {
    chip: 'bg-emerald-500/15 text-emerald-700 dark:text-emerald-300',
    dot: 'bg-emerald-500',
  },
  {
    chip: 'bg-amber-500/15 text-amber-700 dark:text-amber-300',
    dot: 'bg-amber-500',
  },
  {
    chip: 'bg-violet-500/15 text-violet-700 dark:text-violet-300',
    dot: 'bg-violet-500',
  },
  {
    chip: 'bg-rose-500/15 text-rose-700 dark:text-rose-300',
    dot: 'bg-rose-500',
  },
]

export const ACTIVITY_COLOR_COUNT = ACTIVITY_COLORS.length

export function getActivityColor(activity: Activity): ActivityColor {
  return ACTIVITY_COLORS[Math.abs(activity.id) % ACTIVITY_COLORS.length]
}

export function formatMonthLabel(month: Date, locale?: string): string {
  return new Intl.DateTimeFormat(locale, {
    year: 'numeric',
    month: 'long',
  }).format(month)
}

export function formatDayLabel(date: string, locale?: string): string {
  return new Intl.DateTimeFormat(locale, {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
    weekday: 'long',
  }).format(dayjs(date).toDate())
}

export function getWeekdayLabels(locale?: string): string[] {
  const formatter = new Intl.DateTimeFormat(locale, { weekday: 'short' })
  const monday = dayjs('2024-01-01')
  return Array.from({ length: 7 }, (_, index) =>
    formatter.format(monday.add(index, 'day').toDate())
  )
}
