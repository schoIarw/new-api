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
along with this program. All rights reserved.
See <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useCallback, useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

interface TokenDailyFilterBarProps {
  startDate: string
  endDate: string
  tokenKey: string
  onStartDateChange: (val: string) => void
  onEndDateChange: (val: string) => void
  onTokenKeyChange: (val: string) => void
  onSearch: () => void
  onReset: () => void
}

export function TokenDailyFilterBar({
  startDate,
  endDate,
  tokenKey,
  onStartDateChange,
  onEndDateChange,
  onTokenKeyChange,
  onSearch,
  onReset,
}: TokenDailyFilterBarProps) {
  const { t } = useTranslation()

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === 'Enter') {
        onSearch()
      }
    },
    [onSearch]
  )

  return useMemo(
    () => (
      <div className='flex flex-wrap items-end gap-4 rounded-lg border p-4'>
        <div className='space-y-1'>
          <Label>{t('Start Date')}</Label>
          <Input
            type='date'
            value={startDate}
            onChange={(e) => onStartDateChange(e.target.value)}
            onKeyDown={handleKeyDown}
            className='w-40'
          />
        </div>
        <div className='space-y-1'>
          <Label>{t('End Date')}</Label>
          <Input
            type='date'
            value={endDate}
            onChange={(e) => onEndDateChange(e.target.value)}
            onKeyDown={handleKeyDown}
            className='w-40'
          />
        </div>
        <div className='space-y-1'>
          <Label>{t('Token Key')}</Label>
          <Input
            type='text'
            placeholder={t('Search by token key...')}
            value={tokenKey}
            onChange={(e) => onTokenKeyChange(e.target.value)}
            onKeyDown={handleKeyDown}
            className='w-56'
          />
        </div>
        <div className='flex items-center gap-2'>
          <Button variant='default' onClick={onSearch}>
            {t('Search')}
          </Button>
          <Button variant='outline' onClick={onReset}>
            {t('Reset')}
          </Button>
        </div>
      </div>
    ),
    [startDate, endDate, tokenKey, onStartDateChange, onEndDateChange, onTokenKeyChange, onSearch, onReset, handleKeyDown, t]
  )
}
