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
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'

import { createServerError } from '@/lib/server-error-message'

import { getAdminActivities, getPublicActivities } from '../api'

export function useActivities(options: { admin?: boolean } = {}) {
  const { t } = useTranslation()
  return useQuery({
    queryKey: ['activities', options.admin ? 'admin' : 'public'],
    queryFn: async () => {
      const response = options.admin
        ? await getAdminActivities()
        : await getPublicActivities()
      if (!response.success) {
        throw createServerError(response, t('Unable to load activities'))
      }
      return response.data ?? []
    },
    staleTime: 30 * 1000,
  })
}
