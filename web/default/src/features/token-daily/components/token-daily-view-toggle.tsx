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
import { useCallback } from 'react'
import { useTranslation } from 'react-i18next'

import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'

import type { TokenDailyView } from '../types'

interface TokenDailyViewToggleProps {
  value: TokenDailyView
  onChange: (view: TokenDailyView) => void
}

export function TokenDailyViewToggle({
  value,
  onChange,
}: TokenDailyViewToggleProps) {
  const { t } = useTranslation()

  const handleChange = useCallback(
    (newVal: string) => {
      if (newVal === 'model' || newVal === 'total') {
        onChange(newVal as TokenDailyView)
      }
    },
    [onChange]
  )

  return (
    <Tabs value={value} onValueChange={handleChange}>
      <TabsList>
        <TabsTrigger value='model'>{t('By Model')}</TabsTrigger>
        <TabsTrigger value='total'>{t('Total Summary')}</TabsTrigger>
      </TabsList>
    </Tabs>
  )
}
