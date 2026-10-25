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
import { api } from '@/lib/api'

import type {
  ActivityInput,
  ActivityListResponse,
  ActivityResponse,
} from './types'

const ADMIN_BASE = '/api/activity/'

export async function getPublicActivities() {
  const res = await api.get<ActivityListResponse>('/api/activities')
  return res.data
}

export async function getAdminActivities() {
  const res = await api.get<ActivityListResponse>(ADMIN_BASE)
  return res.data
}

export async function createActivity(input: ActivityInput) {
  const res = await api.post<ActivityResponse>(ADMIN_BASE, input)
  return res.data
}

export async function updateActivity(input: ActivityInput & { id: number }) {
  const res = await api.put<ActivityResponse>(ADMIN_BASE, input)
  return res.data
}

export async function deleteActivity(id: number) {
  const res = await api.delete<ActivityResponse>(`${ADMIN_BASE}${id}`)
  return res.data
}
