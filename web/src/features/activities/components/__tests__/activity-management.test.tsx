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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it, vi } from 'vitest'

import { ActivityManagement } from '../activity-management'

vi.mock('../../hooks/use-activities', () => ({
  useActivities: () => ({
    data: [],
    isLoading: false,
    isError: false,
    error: null,
    refetch: vi.fn(),
  }),
}))

it('opens the activity form dialog when Add activity is clicked', async () => {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={client}>
      <ActivityManagement />
    </QueryClientProvider>
  )

  const addButtons = screen.getAllByRole('button', { name: 'Add activity' })
  await userEvent.click(addButtons[0])

  expect(await screen.findByText('Activity date')).toBeInTheDocument()
  expect(screen.getByText('Activity title')).toBeInTheDocument()
  client.clear()
})
