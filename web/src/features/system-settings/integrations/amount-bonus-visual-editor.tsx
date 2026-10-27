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
import { Pencil, Plus, Trash2 } from 'lucide-react'
import { useState, useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { StaticDataTable } from '@/components/data-table/static/static-data-table'
import { StaticRowActions } from '@/components/data-table/static/static-row-actions'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'

import { safeJsonParseWithValidation } from '../utils/json-parser'
import { isObjectRecord } from '../utils/json-validators'
import { AmountBonusDialog, type AmountBonusData } from './amount-bonus-dialog'

type AmountBonusVisualEditorProps = {
  value: string
  onChange: (value: string) => void
}

export function AmountBonusVisualEditor({
  value,
  onChange,
}: AmountBonusVisualEditorProps) {
  const { t } = useTranslation()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editData, setEditData] = useState<AmountBonusData | null>(null)

  const bonuses = useMemo(() => {
    const parsed = safeJsonParseWithValidation<Record<string, unknown>>(value, {
      fallback: {},
      validator: isObjectRecord,
      validatorMessage: 'Amount bonus must be a JSON object',
      context: 'amount bonuses',
    })

    return Object.entries(parsed)
      .map(([amount, percent]) => ({
        amount: Number.parseInt(amount, 10),
        bonusPercent:
          typeof percent === 'number'
            ? percent
            : Number.parseFloat(String(percent)),
      }))
      .filter(
        (item) => !Number.isNaN(item.amount) && !Number.isNaN(item.bonusPercent)
      )
      .sort((a, b) => a.amount - b.amount)
  }, [value])

  const handleSave = (data: AmountBonusData) => {
    const bonusObject = safeJsonParseWithValidation<Record<string, unknown>>(
      value,
      {
        fallback: {},
        validator: isObjectRecord,
        silent: true,
      }
    )

    if (editData && editData.amount !== data.amount) {
      delete bonusObject[editData.amount.toString()]
    }

    bonusObject[data.amount.toString()] = data.bonusPercent

    onChange(JSON.stringify(bonusObject, null, 2))
  }

  const handleDelete = (amount: number) => {
    const bonusObject = safeJsonParseWithValidation<Record<string, unknown>>(
      value,
      {
        fallback: {},
        validator: isObjectRecord,
        silent: true,
      }
    )

    delete bonusObject[amount.toString()]

    onChange(JSON.stringify(bonusObject, null, 2))
  }

  const handleEdit = (bonus: AmountBonusData) => {
    setEditData(bonus)
    setDialogOpen(true)
  }

  const handleAdd = () => {
    setEditData(null)
    setDialogOpen(true)
  }

  return (
    <div className='space-y-4'>
      <div className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
        <p className='text-muted-foreground text-sm'>
          {t('Configure bonus percentages based on recharge amounts')}
        </p>
        <Button
          type='button'
          onClick={(e) => {
            e.preventDefault()
            e.stopPropagation()
            handleAdd()
          }}
          size='sm'
          className='w-full sm:w-auto'
        >
          <Plus className='h-4 w-4 sm:mr-2' />
          <span className='sm:inline'>{t('Add bonus tier')}</span>
        </Button>
      </div>

      {bonuses.length === 0 ? (
        <div className='text-muted-foreground rounded-lg border border-dashed p-6 text-center text-sm'>
          {t(
            'No bonus tiers configured. Click "Add bonus tier" to get started.'
          )}
        </div>
      ) : (
        <div className='rounded-md border'>
          {/* Desktop table view */}
          <StaticDataTable
            className='hidden rounded-none border-0 sm:block'
            data={bonuses}
            getRowKey={(bonus) => bonus.amount}
            columns={[
              {
                id: 'amount',
                header: t('Recharge Amount'),
                cell: (bonus) => (
                  <span className='font-mono text-sm'>${bonus.amount}</span>
                ),
              },
              {
                id: 'bonus-percent',
                header: t('Bonus Percentage'),
                cell: (bonus) => (
                  <code className='bg-muted rounded px-1.5 py-0.5 text-sm'>
                    {bonus.bonusPercent}%
                  </code>
                ),
              },
              {
                id: 'bonus',
                header: t('Bonus'),
                cell: (bonus) => (
                  <StatusBadge
                    variant={bonus.bonusPercent > 0 ? 'info' : 'neutral'}
                    className='font-mono'
                    copyable={false}
                  >
                    +{bonus.bonusPercent}% {t('quota')}
                  </StatusBadge>
                ),
              },
              {
                id: 'actions',
                header: t('Actions'),
                className: 'text-right',
                cellClassName: 'text-right',
                cell: (bonus) => (
                  <StaticRowActions
                    editLabel={t('Edit')}
                    deleteLabel={t('Delete')}
                    menuLabel={t('Open menu')}
                    onEdit={() => handleEdit(bonus)}
                    onDelete={() => handleDelete(bonus.amount)}
                  />
                ),
              },
            ]}
          />

          {/* Mobile card view */}
          <div className='divide-y sm:hidden'>
            {bonuses.map((bonus) => (
              <div key={bonus.amount} className='p-4'>
                <div className='mb-3 flex items-start justify-between'>
                  <div className='flex-1'>
                    <div className='mb-2 font-mono text-base font-medium'>
                      ${bonus.amount}
                    </div>
                    <StatusBadge
                      variant={bonus.bonusPercent > 0 ? 'info' : 'neutral'}
                      className='font-mono'
                      copyable={false}
                    >
                      +{bonus.bonusPercent}% {t('quota')}
                    </StatusBadge>
                  </div>
                  <div className='flex gap-1'>
                    <Button
                      type='button'
                      variant='ghost'
                      size='sm'
                      onClick={(e) => {
                        e.preventDefault()
                        e.stopPropagation()
                        handleEdit(bonus)
                      }}
                    >
                      <Pencil className='h-4 w-4' />
                    </Button>
                    <Button
                      type='button'
                      variant='ghost'
                      size='sm'
                      onClick={(e) => {
                        e.preventDefault()
                        e.stopPropagation()
                        handleDelete(bonus.amount)
                      }}
                    >
                      <Trash2 className='h-4 w-4' />
                    </Button>
                  </div>
                </div>
                <div className='text-sm'>
                  <span className='text-muted-foreground'>
                    {t('Bonus Percentage:')}{' '}
                  </span>
                  <code className='bg-muted rounded px-1.5 py-0.5 text-xs'>
                    {bonus.bonusPercent}%
                  </code>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      <AmountBonusDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        onSave={handleSave}
        editData={editData}
      />
    </div>
  )
}
