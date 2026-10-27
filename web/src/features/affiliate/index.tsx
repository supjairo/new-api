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
  Gift,
  Share2,
  TrendingUp,
  UserPlus,
  WalletCards,
} from 'lucide-react'
import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { CopyButton } from '@/components/copy-button'
import { SectionPageLayout } from '@/components/layout'
import { PageTransition } from '@/components/page-transition'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { IconBadge, type IconBadgeTone } from '@/components/ui/icon-badge'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { TitledCard } from '@/components/ui/titled-card'
import { TransferDialog } from '@/features/wallet/components/dialogs/transfer-dialog'
import { useAffiliate } from '@/features/wallet/hooks/use-affiliate'
import type { UserWalletData } from '@/features/wallet/types'
import { getSelf } from '@/lib/api'
import { formatQuota } from '@/lib/format'
import { cn } from '@/lib/utils'

export function AffiliateProgram() {
  const { t } = useTranslation()
  const {
    affiliateLink,
    rewards,
    loading: affiliateLoading,
    transferQuota,
    transferring,
  } = useAffiliate()
  const [user, setUser] = useState<UserWalletData | null>(null)
  const [userLoading, setUserLoading] = useState(true)
  const [transferDialogOpen, setTransferDialogOpen] = useState(false)

  const fetchUser = useCallback(async () => {
    try {
      setUserLoading(true)
      const response = await getSelf()
      if (response.success && response.data) {
        setUser(response.data as UserWalletData)
      }
    } finally {
      setUserLoading(false)
    }
  }, [])

  useEffect(() => {
    void fetchUser()
  }, [fetchUser])

  const hasRewards = (user?.aff_quota ?? 0) > 0

  const stats: {
    label: string
    value: string
    icon: typeof WalletCards
    tone: IconBadgeTone
  }[] = [
    {
      label: t('Pending'),
      value: formatQuota(user?.aff_quota ?? 0),
      icon: WalletCards,
      tone: 'success',
    },
    {
      label: t('Total Earned'),
      value: formatQuota(user?.aff_history_quota ?? 0),
      icon: Gift,
      tone: 'warning',
    },
    {
      label: t('Invites'),
      value: String(user?.aff_count ?? 0),
      icon: UserPlus,
      tone: 'info',
    },
  ]

  const handleTransfer = async (amount: number) => {
    const success = await transferQuota(amount)
    if (success) {
      await fetchUser()
    }
    return success
  }

  const rewardRules = [
    {
      icon: UserPlus,
      title: t('Sign-up reward for the invited user'),
      reward: formatQuota(rewards.quota_for_invitee),
      enabled: rewards.quota_for_invitee > 0,
      description: t(
        'Your friend receives this bonus automatically right after signing up through your link.'
      ),
    },
    {
      icon: Gift,
      title: t('Sign-up reward for you'),
      reward: formatQuota(rewards.quota_for_inviter),
      enabled: rewards.quota_for_inviter > 0,
      description: t(
        'You receive this bonus in your invitation balance every time a friend signs up through your link.'
      ),
    },
    {
      icon: TrendingUp,
      title: t('Top-up rebate'),
      reward: t('{{percent}}% of each top-up', {
        percent: rewards.topup_rebate_percentage,
      }),
      enabled: rewards.topup_rebate_percentage > 0,
      description: t(
        'Every time an invited user tops up, you earn this percentage of the amount as invitation balance.'
      ),
    },
  ]

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>
          {t('Referral Program')}
        </SectionPageLayout.Title>
        <SectionPageLayout.Content>
          <PageTransition className='mx-auto flex w-full max-w-7xl flex-col gap-4 sm:gap-5'>
            <p className='text-muted-foreground text-sm'>
              {t('Invite friends and earn rewards when they top up.')}
            </p>

            <div className='grid grid-cols-3 divide-x rounded-lg border'>
              {affiliateLoading || userLoading
                ? stats.map((item) => (
                    <div
                      key={item.label}
                      className='min-w-0 px-2.5 py-2.5 sm:px-5 sm:py-4'
                    >
                      <Skeleton className='h-3.5 w-full' />
                      <Skeleton className='mt-2 h-6 w-full sm:h-7' />
                    </div>
                  ))
                : stats.map((item) => (
                    <div
                      key={item.label}
                      className='min-w-0 px-2.5 py-2.5 sm:px-5 sm:py-4'
                    >
                      <div className='flex items-center gap-2'>
                        <IconBadge tone={item.tone} size='xs'>
                          <item.icon />
                        </IconBadge>
                        <span className='text-muted-foreground truncate text-xs font-medium tracking-wider uppercase'>
                          {item.label}
                        </span>
                      </div>
                      <div className='mt-1.5 truncate text-lg font-semibold tabular-nums sm:text-xl'>
                        {item.value}
                      </div>
                    </div>
                  ))}
            </div>

            <TitledCard
              title={t('Referral link')}
              description={t('Share this link to invite friends')}
              icon={<Share2 className='h-4 w-4' />}
              iconTone='info'
              disableHoverEffect
            >
              <div className='flex items-center gap-2'>
                <Input
                  value={affiliateLink}
                  readOnly
                  className='bg-muted/30 h-9 min-w-0 flex-1 font-mono text-xs'
                />
                <CopyButton
                  value={affiliateLink}
                  variant='outline'
                  className='bg-background size-9 shrink-0'
                  iconClassName='size-4'
                  tooltip={t('Copy referral link')}
                  aria-label={t('Copy referral link')}
                />
              </div>
            </TitledCard>

            <TitledCard
              title={t('How rewards work')}
              description={t(
                'You can earn rewards in the following ways based on the current program settings.'
              )}
              icon={<TrendingUp className='h-4 w-4' />}
              iconTone='chart-3'
              disableHoverEffect
            >
              {affiliateLoading ? (
                <div className='space-y-3'>
                  <Skeleton className='h-12 w-full' />
                  <Skeleton className='h-12 w-full' />
                  <Skeleton className='h-12 w-full' />
                </div>
              ) : (
                <div className='space-y-3'>
                  {rewardRules.map((rule) => (
                    <div
                      key={rule.title}
                      className='bg-muted/30 flex items-start gap-3 rounded-lg p-3'
                    >
                      <span className='bg-background flex size-8 shrink-0 items-center justify-center rounded-lg border'>
                        <rule.icon className='text-muted-foreground size-4' />
                      </span>
                      <div className='min-w-0 flex-1'>
                        <div className='flex flex-wrap items-center gap-2'>
                          <h4 className='text-sm font-medium'>{rule.title}</h4>
                          <StatusBadge
                            label={rule.reward}
                            variant={rule.enabled ? 'info' : 'neutral'}
                            size='sm'
                            copyable={false}
                            className={cn(
                              'font-mono',
                              rule.enabled && 'text-green-600'
                            )}
                          />
                        </div>
                        <p className='text-muted-foreground mt-0.5 text-sm'>
                          {rule.description}
                        </p>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </TitledCard>

            <TitledCard
              title={t('Invitation rewards')}
              description={t(
                'Rewards accumulate in your invitation balance. Transfer at least $1 to your balance anytime.'
              )}
              icon={<WalletCards className='h-4 w-4' />}
              iconTone='success'
              disableHoverEffect
              action={
                hasRewards ? (
                  <Button
                    onClick={() => setTransferDialogOpen(true)}
                    disabled={transferring}
                    size='sm'
                    className='w-full sm:w-auto'
                  >
                    {t('Transfer to Balance')}
                  </Button>
                ) : null
              }
            >
              <div className='flex items-baseline gap-2'>
                <span className='text-muted-foreground text-sm'>
                  {t('Pending')}
                </span>
                <span className='text-2xl font-semibold tabular-nums'>
                  {formatQuota(user?.aff_quota ?? 0)}
                </span>
              </div>
              {!hasRewards && !userLoading ? (
                <p className='text-muted-foreground mt-2 text-sm'>
                  {t(
                    'No rewards to transfer yet. They will appear here after your invited users top up.'
                  )}
                </p>
              ) : null}
            </TitledCard>
          </PageTransition>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <TransferDialog
        open={transferDialogOpen}
        onOpenChange={setTransferDialogOpen}
        onConfirm={handleTransfer}
        availableQuota={user?.aff_quota ?? 0}
        transferring={transferring}
      />
    </>
  )
}
