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
import { act, render, screen } from '@testing-library/react'
import { createInstance } from 'i18next'
import { I18nextProvider, initReactI18next } from 'react-i18next'
import { beforeEach, expect, it } from 'vitest'

import { TermsFooter } from '@/features/auth/components/terms-footer'
import type { SystemStatus } from '@/features/auth/types'
import en from '@/i18n/locales/en.json'
import zh from '@/i18n/locales/zh.json'

const i18n = createInstance()

beforeEach(async () => {
  await i18n.use(initReactI18next).init({
    lng: 'zh',
    fallbackLng: false,
    nsSeparator: false,
    resources: { en, zh },
    interpolation: { escapeValue: false },
  })
})

function renderFooter(
  features: { userAgreement?: boolean; privacyPolicy?: boolean },
  variant: 'sign-in' | 'sign-up' = 'sign-in'
) {
  const status = {
    user_agreement_enabled: features.userAgreement ?? false,
    privacy_policy_enabled: features.privacyPolicy ?? false,
  } as SystemStatus
  return render(
    <I18nextProvider i18n={i18n}>
      <TermsFooter variant={variant} status={status} />
    </I18nextProvider>
  )
}

it('renders the whole sign-in consent sentence in Chinese with both links', () => {
  const { container } = renderFooter({
    userAgreement: true,
    privacyPolicy: true,
  })
  expect(container.querySelector('p')).toHaveTextContent(
    '点击登录即表示您同意我们的用户协议和隐私政策。'
  )
  const links = screen.getAllByRole('link')
  expect(links[0]).toHaveAttribute('href', '/user-agreement')
  expect(links[1]).toHaveAttribute('href', '/privacy-policy')
})

it('drops the conjunction when only the privacy policy is enabled', () => {
  const { container } = renderFooter({ privacyPolicy: true })
  expect(container.querySelector('p')).toHaveTextContent(
    '点击登录即表示您同意我们的隐私政策。'
  )
  expect(screen.getAllByRole('link')).toHaveLength(1)
})

it('renders nothing when neither agreement is enabled', () => {
  const { container } = renderFooter({})
  expect(container.querySelector('p')).toBeNull()
})

it('renders the complete English sentence after switching language', async () => {
  const { container } = renderFooter(
    { userAgreement: true, privacyPolicy: true },
    'sign-up'
  )
  expect(container.querySelector('p')).toHaveTextContent(
    '创建账户即表示您同意我们的用户协议和隐私政策。'
  )
  await act(() => i18n.changeLanguage('en'))
  expect(container.querySelector('p')).toHaveTextContent(
    'By creating an account, you agree to our User Agreement and Privacy Policy.'
  )
})
