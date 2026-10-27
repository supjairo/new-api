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
import { useMutation, useQueryClient } from '@tanstack/react-query'
import dayjs from 'dayjs'
import {
  CalendarDays,
  ChevronLeft,
  ChevronRight,
  Pencil,
  Plus,
  Trash2,
} from 'lucide-react'
import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { SectionPageLayout } from '@/components/layout'
import { LoadingState } from '@/components/loading-state'
import { Button } from '@/components/ui/button'
import { toIntlLocale } from '@/i18n/languages'

import { ActivityFormDialog } from './activity-form-dialog'
import { MonthCalendar } from './month-calendar'
import { deleteActivity } from '../api'
import { useActivities } from '../hooks/use-activities'
import {
  formatDate,
  formatDayLabel,
  formatMonthLabel,
} from '../lib/calendar'
import type { Activity } from '../types'

export function ActivityManagement() {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const queryClient = useQueryClient()
  const [month, setMonth] = useState(() => dayjs().startOf('month').toDate())
  const [selectedDate, setSelectedDate] = useState(() => formatDate(new Date()))
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<Activity | null>(null)
  const [deleting, setDeleting] = useState<Activity | null>(null)

  const activitiesQuery = useActivities({ admin: true })
  const activities = activitiesQuery.data ?? []
  const dayActivities = activities.filter(
    (activity) => activity.date === selectedDate
  )

  const deleteMutation = useMutation({
    mutationFn: async (activity: Activity) => {
      const response = await deleteActivity(activity.id)
      if (!response.success) {
        throw new Error(response.message || t('Failed to delete activity'))
      }
      return response
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['activities'] })
      toast.success(t('Deleted'))
      setDeleting(null)
    },
  })

  const shiftMonth = (delta: number) => {
    setMonth((previous) =>
      dayjs(previous).add(delta, 'month').startOf('month').toDate()
    )
  }

  const selectDate = (date: string) => {
    setSelectedDate(date)
    if (dayjs(date).format('YYYY-MM') !== dayjs(month).format('YYYY-MM')) {
      setMonth(dayjs(date).startOf('month').toDate())
    }
  }

  const openCreate = () => {
    setEditing(null)
    setFormOpen(true)
  }

  const openEdit = (activity: Activity) => {
    setEditing(activity)
    setFormOpen(true)
  }

  let content: ReactNode
  if (activitiesQuery.isError) {
    content = (
      <ErrorState
        description={activitiesQuery.error.message}
        onRetry={() => void activitiesQuery.refetch()}
      />
    )
  } else if (activitiesQuery.isLoading) {
    content = <LoadingState message={t('Loading activities')} />
  } else {
    content = (
      <div className='grid gap-4 lg:grid-cols-[minmax(0,1fr)_360px]'>
        <MonthCalendar
          month={month}
          activities={activities}
          selectedDate={selectedDate}
          onSelectDate={selectDate}
        />
        <section className='bg-card rounded-xl border p-4'>
          <h2 className='font-semibold'>
            {formatDayLabel(selectedDate, locale)}
          </h2>
          {dayActivities.length === 0 ? (
            <EmptyState
              icon={CalendarDays}
              className='min-h-[180px] border-0 px-2'
              title={t('No activities on this day')}
              action={
                <Button size='sm' variant='outline' onClick={openCreate}>
                  <Plus className='size-4' />
                  {t('Add activity')}
                </Button>
              }
            />
          ) : (
            <ul className='mt-3 space-y-3'>
              {dayActivities.map((activity) => (
                <li key={activity.id} className='bg-muted/40 rounded-lg p-3'>
                  <div className='flex items-start justify-between gap-2'>
                    <h3 className='font-medium'>{activity.title}</h3>
                    <div className='flex shrink-0 items-center gap-1'>
                      <Button
                        type='button'
                        variant='ghost'
                        size='icon'
                        aria-label={t('Edit activity')}
                        onClick={() => openEdit(activity)}
                      >
                        <Pencil className='size-4' />
                      </Button>
                      <Button
                        type='button'
                        variant='ghost'
                        size='icon'
                        aria-label={t('Delete activity')}
                        onClick={() => setDeleting(activity)}
                      >
                        <Trash2 className='size-4' />
                      </Button>
                    </div>
                  </div>
                  {activity.description && (
                    <p className='text-muted-foreground mt-1 text-sm whitespace-pre-wrap'>
                      {activity.description}
                    </p>
                  )}
                </li>
              ))}
            </ul>
          )}
        </section>
      </div>
    )
  }

  return (
    <>
      <SectionPageLayout>
      <SectionPageLayout.Title>{t('Activity Calendar')}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <Button size='sm' onClick={openCreate}>
          <Plus className='size-4' />
          {t('Add activity')}
        </Button>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='space-y-4'>
          <div className='flex flex-wrap items-center justify-between gap-3'>
            <div className='flex items-center gap-2'>
              <Button
                type='button'
                variant='outline'
                size='icon'
                aria-label={t('Previous month')}
                onClick={() => shiftMonth(-1)}
              >
                <ChevronLeft className='size-4' />
              </Button>
              <span className='min-w-40 text-center text-lg font-semibold'>
                {formatMonthLabel(month, locale)}
              </span>
              <Button
                type='button'
                variant='outline'
                size='icon'
                aria-label={t('Next month')}
                onClick={() => shiftMonth(1)}
              >
                <ChevronRight className='size-4' />
              </Button>
            </div>
            <Button
              type='button'
              variant='outline'
              size='sm'
              onClick={() => {
                const today = new Date()
                setMonth(dayjs(today).startOf('month').toDate())
                setSelectedDate(formatDate(today))
              }}
            >
              {t('Today')}
            </Button>
          </div>

          {content}
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>

      <ActivityFormDialog
        open={formOpen}
        onOpenChange={setFormOpen}
        activity={editing}
        defaultDate={selectedDate}
      />

      <ConfirmDialog
        open={deleting !== null}
        onOpenChange={(open) => {
          if (!open) setDeleting(null)
        }}
        title={t('Delete activity')}
        desc={t(
          'This activity will be removed from the calendar. This action cannot be undone.'
        )}
        confirmText={t('Delete')}
        destructive
        isLoading={deleteMutation.isPending}
        handleConfirm={() => {
          if (deleting) deleteMutation.mutate(deleting)
        }}
      />
    </>
  )
}
