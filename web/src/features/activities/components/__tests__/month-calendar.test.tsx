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
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'

import { getActivityColor } from '../../lib/calendar'
import type { Activity } from '../../types'
import { MonthCalendar } from '../month-calendar'

const createActivity = (id: number, date: string): Activity => ({
  id,
  date,
  title: `Activity ${id}`,
  description: '',
  created_time: 0,
  updated_time: 0,
})

const renderCalendar = (
  props: Partial<React.ComponentProps<typeof MonthCalendar>> = {}
) => {
  const onSelectDate = vi.fn()
  const utils = render(
    <MonthCalendar
      month={new Date(2026, 9, 1)}
      activities={[]}
      selectedDate={null}
      onSelectDate={onSelectDate}
      {...props}
    />
  )
  return { onSelectDate, ...utils }
}

describe('MonthCalendar', () => {
  it('renders activity titles with their stable color classes', () => {
    const activity = createActivity(3, '2026-10-05')
    renderCalendar({ activities: [activity] })

    const chip = screen.getByText('Activity 3')
    expect(chip).toHaveClass(...getActivityColor(activity).chip.split(' '))
  })

  it('calls onSelectDate with the clicked date', async () => {
    const { onSelectDate, container } = renderCalendar()
    // Monday-first grid for October 2026 starts on 2026-09-28, so cell 9
    // is 2026-10-07.
    const dayButtons = container.querySelectorAll('button')
    await userEvent.click(dayButtons[9])

    expect(onSelectDate).toHaveBeenCalledWith('2026-10-07')
  })

  it('shows the remaining count when a day has more than two activities', () => {
    renderCalendar({
      activities: [
        createActivity(1, '2026-10-05'),
        createActivity(2, '2026-10-05'),
        createActivity(3, '2026-10-05'),
      ],
    })

    expect(screen.getByText('+1 more')).toBeInTheDocument()
  })
})
