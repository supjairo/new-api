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
import { zodResolver } from '@hookform/resolvers/zod'
import dayjs from 'dayjs'
import { useEffect, useMemo } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import * as z from 'zod'

import { Dialog } from '@/components/dialog'
import { DatePicker } from '@/components/date-picker'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { getServerErrorMessage } from '@/lib/server-error-message'

import { createActivity, updateActivity } from '../api'
import { DATE_FORMAT } from '../lib/calendar'
import type { Activity } from '../types'

type ActivityFormValues = {
  date: string
  title: string
  description: string
}

type ActivityFormDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  activity: Activity | null
  defaultDate: string
}

export function ActivityFormDialog(props: ActivityFormDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  const schema = useMemo(
    () =>
      z.object({
        date: z.string().min(1, t('Activity date is required')),
        title: z
          .string()
          .trim()
          .min(1, t('Activity title is required'))
          .max(100, t('Activity title cannot exceed 100 characters')),
        description: z
          .string()
          .max(2000, t('Activity description cannot exceed 2000 characters')),
      }),
    [t]
  )

  const form = useForm<ActivityFormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      date: props.defaultDate,
      title: '',
      description: '',
    },
  })

  useEffect(() => {
    if (!props.open) return
    form.reset({
      date: props.activity?.date ?? props.defaultDate,
      title: props.activity?.title ?? '',
      description: props.activity?.description ?? '',
    })
    form.clearErrors()
  }, [form, props.activity, props.defaultDate, props.open])

  const mutation = useMutation({
    meta: { errorToast: false },
    mutationFn: async (values: ActivityFormValues) => {
      const payload = {
        date: values.date,
        title: values.title.trim(),
        description: values.description.trim(),
      }
      const response = props.activity
        ? await updateActivity({ ...payload, id: props.activity.id })
        : await createActivity(payload)
      if (!response.success) {
        throw new Error(response.message || t('Failed to save activity'))
      }
      return response.data
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['activities'] })
      toast.success(
        props.activity ? t('Saved successfully') : t('Added successfully')
      )
      props.onOpenChange(false)
    },
    onError: (error: Error) => {
      form.setError('root', {
        message: getServerErrorMessage(error, t('Failed to save activity')),
      })
    },
  })

  const onSubmit = (values: ActivityFormValues) => {
    form.clearErrors('root')
    mutation.mutate(values)
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={props.activity ? t('Edit activity') : t('Add activity')}
      description={t('Scheduled activities are visible on the activity calendar.')}
      footer={
        <>
          <Button
            type='button'
            variant='outline'
            onClick={() => props.onOpenChange(false)}
            disabled={mutation.isPending}
          >
            {t('Cancel')}
          </Button>
          <Button
            type='submit'
            form='activity-form'
            disabled={mutation.isPending}
          >
            {t('Save')}
          </Button>
        </>
      }
    >
      <Form {...form}>
        <form
          id='activity-form'
          className='space-y-4'
          onSubmit={form.handleSubmit(onSubmit)}
        >
          <FormField
            control={form.control}
            name='date'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Activity date')}</FormLabel>
                <DatePicker
                  selected={field.value ? dayjs(field.value).toDate() : undefined}
                  onSelect={(date) =>
                    field.onChange(date ? dayjs(date).format(DATE_FORMAT) : '')
                  }
                  disableFuture={false}
                />
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='title'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Activity title')}</FormLabel>
                <FormControl>
                  <Input
                    {...field}
                    placeholder={t('Enter an activity title')}
                    maxLength={100}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='description'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Activity description')}</FormLabel>
                <FormControl>
                  <Textarea
                    {...field}
                    rows={5}
                    maxLength={2000}
                    placeholder={t('Describe the activity and how users can join')}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          {form.formState.errors.root?.message && (
            <p role='alert' className='text-destructive text-sm'>
              {form.formState.errors.root.message}
            </p>
          )}
        </form>
      </Form>
    </Dialog>
  )
}
