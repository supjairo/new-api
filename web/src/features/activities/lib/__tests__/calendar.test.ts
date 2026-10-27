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
import { describe, expect, it } from 'vitest'

import type { Activity } from '../../types'
import {
  ACTIVITY_COLOR_COUNT,
  buildMonthGrid,
  getActivityColor,
  groupActivitiesByDate,
} from '../calendar'

const createActivity = (id: number, date: string): Activity => ({
  id,
  date,
  title: `Activity ${id}`,
  description: '',
  created_time: 0,
  updated_time: 0,
})

describe('buildMonthGrid', () => {
  it('builds a Monday-first six-week grid for the target month', () => {
    const days = buildMonthGrid(new Date(2026, 9, 1), [])

    expect(days).toHaveLength(42)
    expect(days[0].date).toBe('2026-09-28')
    expect(days[41].date).toBe('2026-11-08')
    expect(days.filter((day) => day.inMonth)).toHaveLength(31)
  })

  it('attaches activities to their date and marks today', () => {
    const days = buildMonthGrid(
      new Date(2026, 9, 1),
      [createActivity(1, '2026-10-05')],
      new Date(2026, 9, 5)
    )
    const target = days.find((day) => day.date === '2026-10-05')

    expect(target?.activities).toHaveLength(1)
    expect(target?.isToday).toBe(true)
    expect(days.find((day) => day.date === '2026-10-06')?.isToday).toBe(false)
  })
})

describe('groupActivitiesByDate', () => {
  it('keeps activities with the same date together', () => {
    const grouped = groupActivitiesByDate([
      createActivity(1, '2026-10-05'),
      createActivity(2, '2026-10-05'),
      createActivity(3, '2026-10-08'),
    ])

    expect(grouped.get('2026-10-05')?.map((item) => item.id)).toEqual([1, 2])
    expect(grouped.get('2026-10-08')?.map((item) => item.id)).toEqual([3])
  })
})

describe('getActivityColor', () => {
  it('returns a stable color for the same activity id', () => {
    const activity = createActivity(7, '2026-10-05')

    expect(getActivityColor(activity)).toBe(getActivityColor(activity))
  })

  it('cycles through every color class by id', () => {
    const colors = new Set(
      Array.from(
        { length: ACTIVITY_COLOR_COUNT * 2 },
        (_, index) => getActivityColor(createActivity(index, '2026-10-05')).dot
      )
    )

    expect(colors.size).toBe(ACTIVITY_COLOR_COUNT)
  })
})
