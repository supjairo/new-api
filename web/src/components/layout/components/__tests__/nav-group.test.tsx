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
import {
  createMemoryHistory,
  createRootRoute,
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import { render, screen } from '@testing-library/react'
import { Send } from 'lucide-react'
import { expect, test } from 'vitest'

import { SidebarProvider } from '@/components/ui/sidebar'

import { NavGroup } from '../nav-group'

test('external entries render as new-tab anchors instead of router links', async () => {
  const router = createRouter({
    routeTree: createRootRoute({
      component: () => (
        <SidebarProvider>
          <NavGroup
            title='Support'
            items={[
              {
                title: 'Telegram Channel',
                url: 'https://t.me/i2pp_studio_channel',
                icon: Send,
                external: true,
              },
            ]}
          />
        </SidebarProvider>
      ),
    }),
    history: createMemoryHistory({ initialEntries: ['/dashboard/overview'] }),
  })
  await router.load()
  render(<RouterProvider router={router} />)

  const link = screen.getByRole('link', { name: /Telegram Channel/ })
  expect(link).toHaveAttribute('href', 'https://t.me/i2pp_studio_channel')
  expect(link).toHaveAttribute('target', '_blank')
  expect(link).toHaveAttribute('rel', 'noopener noreferrer')
})
