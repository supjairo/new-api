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
import dayjs from 'dayjs'
import { CalendarDays, ChevronLeft, ChevronRight } from 'lucide-react'
import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { PublicLayout } from '@/components/layout'
import { LoadingState } from '@/components/loading-state'
import { PageTransition } from '@/components/page-transition'
import { Button } from '@/components/ui/button'
import { toIntlLocale } from '@/i18n/languages'

import { MonthCalendar } from './month-calendar'
import { useActivities } from '../hooks/use-activities'
import {
  formatDate,
  formatDayLabel,
  formatMonth,
  formatMonthLabel,
} from '../lib/calendar'

export function ActivityCalendarPage() {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const [month, setMonth] = useState(() =>
    dayjs().startOf('month').toDate()
  )
  const [selectedDate, setSelectedDate] = useState(() => formatDate(new Date()))
  const activitiesQuery = useActivities()

  const activities = activitiesQuery.data ?? []
  const monthKey = formatMonth(month)
  const monthCount = activities.filter((activity) =>
    activity.date.startsWith(monthKey)
  ).length
  const selectedActivities = activities.filter(
    (activity) => activity.date === selectedDate
  )

  const shiftMonth = (delta: number) => {
    setMonth((previous) =>
      dayjs(previous).add(delta, 'month').startOf('month').toDate()
    )
  }

  const goToToday = () => {
    const today = new Date()
    setMonth(dayjs(today).startOf('month').toDate())
    setSelectedDate(formatDate(today))
  }

  const selectDate = (date: string) => {
    setSelectedDate(date)
    if (dayjs(date).format('YYYY-MM') !== monthKey) {
      setMonth(dayjs(date).startOf('month').toDate())
    }
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
      <div className='grid gap-4 lg:grid-cols-[minmax(0,1fr)_340px]'>
        <MonthCalendar
          month={month}
          activities={activities}
          selectedDate={selectedDate}
          onSelectDate={selectDate}
        />
        <aside className='bg-card flex flex-col rounded-xl border p-4'>
          <h2 className='font-semibold'>
            {formatDayLabel(selectedDate, locale)}
          </h2>
          {selectedActivities.length === 0 ? (
            <EmptyState
              icon={CalendarDays}
              className='min-h-[180px] border-0 px-2'
              title={t('No activities on this day')}
            />
          ) : (
            <ul className='mt-3 space-y-3'>
              {selectedActivities.map((activity) => (
                <li key={activity.id} className='bg-muted/40 rounded-lg p-3'>
                  <h3 className='font-medium'>{activity.title}</h3>
                  {activity.description && (
                    <p className='text-muted-foreground mt-1 text-sm whitespace-pre-wrap'>
                      {activity.description}
                    </p>
                  )}
                </li>
              ))}
            </ul>
          )}
        </aside>
      </div>
    )
  }

  return (
    <PublicLayout showMainContainer={false}>
      <PageTransition className='mx-auto w-full max-w-[1100px] space-y-6 px-3 pt-20 pb-12 sm:px-6 sm:pt-24'>
        <header className='space-y-2'>
          <h1 className='text-3xl font-bold tracking-tight'>
            {t('Activity Calendar')}
          </h1>
          <p className='text-muted-foreground'>
            {t(
              'Browse scheduled activities and read what is happening on each day.'
            )}
          </p>
        </header>

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
          <div className='flex items-center gap-2'>
            <span className='text-muted-foreground text-sm'>
              {t('{{count}} activities this month', { count: monthCount })}
            </span>
            <Button type='button' variant='outline' size='sm' onClick={goToToday}>
              {t('Today')}
            </Button>
          </div>
        </div>

        {content}
      </PageTransition>
    </PublicLayout>
  )
}
