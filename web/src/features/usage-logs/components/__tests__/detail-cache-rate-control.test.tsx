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
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import {
  afterAll,
  afterEach,
  beforeEach,
  expect,
  test,
  vi,
} from 'vitest'

import en from '@/i18n/locales/en.json'
import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'

import type { UsageLog } from '../../data/schema'
import type { LogOtherData } from '../../types'
import { DetailsDialog } from '../dialogs/details-dialog'

vi.mock('@lobehub/icons', () => ({}))
vi.hoisted(() => {
  vi.stubGlobal('localStorage', {
    getItem: () => null,
    setItem: () => undefined,
    removeItem: () => undefined,
  })
})
afterAll(() => vi.unstubAllGlobals())

function makeLog(other: LogOtherData): UsageLog {
  return {
    id: 1,
    user_id: 1,
    created_at: 1,
    type: 2,
    content: '',
    username: 'user',
    token_name: 'token',
    model_name: 'gpt-4o-mini',
    quota: 5000,
    prompt_tokens: 100,
    completion_tokens: 50,
    use_time: 0,
    is_stream: false,
    channel: 1,
    channel_name: '',
    token_id: 1,
    group: 'default',
    ip: '',
    other: JSON.stringify(other),
    request_id: 'req-1',
    upstream_request_id: '',
  }
}

function renderDialog(other: LogOtherData, isAdmin: boolean) {
  render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={client}>
        <DetailsDialog
          log={makeLog(other)}
          isAdmin={isAdmin}
          isRoot={false}
          open
          onOpenChange={() => undefined}
        />
      </QueryClientProvider>
    </I18nextProvider>
  )
}

const previousConfig = useSystemConfigStore.getState().config
let client: QueryClient
const i18n = createInstance()
beforeEach(async () => {
  await i18n.init({
    lng: 'en',
    resources: { en },
    interpolation: { escapeValue: false },
  })
  useSystemConfigStore
    .getState()
    .setConfig({ currency: { ...DEFAULT_CURRENCY_CONFIG } })
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  client.setQueryData(['status'], {}, { updatedAt: Date.now() + 60_000 })
  client.setQueryData(
    ['pricing'],
    { data: [], vendors: [] },
    { updatedAt: Date.now() + 60_000 }
  )
})
afterEach(() => {
  client.clear()
  useSystemConfigStore.getState().setConfig(previousConfig)
})

test('renders the cache rate control section with all four values for admins', () => {
  renderDialog(
    {
      admin_info: {
        upstream_cache_rate_control: {
          model: 'deepseek-chat',
          read_percent: 70,
          original_read: 12345,
          adjusted_read: 8641,
        },
      },
    },
    true
  )
  expect(screen.getByText('Cache Rate Control')).toBeVisible()
  expect(screen.getByText('Controlled model')).toBeVisible()
  expect(screen.getByText('deepseek-chat')).toBeVisible()
  expect(screen.getByText('Read cache percent')).toBeVisible()
  expect(screen.getByText('70%')).toBeVisible()
  expect(screen.getByText('Original cache read')).toBeVisible()
  expect(screen.getByText('12,345')).toBeVisible()
  expect(screen.getByText('Adjusted cache read')).toBeVisible()
  expect(screen.getByText('8,641')).toBeVisible()
})

test('falls back to the legacy cache_billing_adjustment field for historical logs', () => {
  renderDialog(
    {
      admin_info: {
        cache_billing_adjustment: {
          model: 'claude-3-7-sonnet',
          read_percent: 0,
          original_read: 900,
          adjusted_read: 0,
        },
      },
    },
    true
  )
  expect(screen.getByText('Cache Rate Control')).toBeVisible()
  expect(screen.getByText('claude-3-7-sonnet')).toBeVisible()
  expect(screen.getByText('0%')).toBeVisible()
  expect(screen.getByText('900')).toBeVisible()
})

test('prefers the new field when both field names are present', () => {
  renderDialog(
    {
      admin_info: {
        upstream_cache_rate_control: {
          model: 'new-model',
          read_percent: 50,
          original_read: 10,
          adjusted_read: 5,
        },
        cache_billing_adjustment: {
          model: 'old-model',
          read_percent: 10,
          original_read: 99,
          adjusted_read: 9,
        },
      },
    },
    true
  )
  expect(screen.getByText('new-model')).toBeVisible()
  expect(screen.queryByText('old-model')).not.toBeInTheDocument()
})

test('does not render the section when the field is absent', () => {
  renderDialog({ admin_info: { task_plugin: undefined } }, true)
  expect(screen.queryByText('Cache Rate Control')).not.toBeInTheDocument()
})

test('does not render the section for non-admin users', () => {
  renderDialog(
    {
      admin_info: {
        upstream_cache_rate_control: {
          model: 'deepseek-chat',
          read_percent: 70,
          original_read: 10,
          adjusted_read: 7,
        },
      },
    },
    false
  )
  expect(screen.queryByText('Cache Rate Control')).not.toBeInTheDocument()
})
