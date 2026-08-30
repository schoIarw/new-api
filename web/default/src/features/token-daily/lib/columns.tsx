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
import { createColumnHelper } from '@tanstack/react-table'
import { useTranslation } from 'react-i18next'

import { DataTableColumnHeader } from '@/components/data-table'
import type { TokenDailyModelItem, TokenDailyTotalItem } from '../types'

// ============================================================================
// Token Daily Model Columns
// ============================================================================

const modelHelper = createColumnHelper<TokenDailyModelItem>()

export function useTokenDailyModelColumns() {
  const { t } = useTranslation()

  return [
    modelHelper.accessor('stat_date', {
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Date')} />
      ),
      cell: ({ getValue }) => getValue(),
      enableSorting: true,
    }),
    modelHelper.accessor('token_name', {
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Token Name')} />
      ),
      cell: ({ getValue }) => getValue(),
    }),
    modelHelper.accessor('token_key', {
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Token Key')} />
      ),
      cell: ({ getValue }) => {
        const val = getValue()
        return val ? `${val.substring(0, 8)}...${val.slice(-4)}` : '-'
      },
    }),
    modelHelper.accessor('token_group', {
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Group')} />
      ),
      cell: ({ getValue }) => getValue(),
    }),
    modelHelper.accessor('model_name', {
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Model')} />
      ),
      cell: ({ getValue }) => getValue(),
    }),
    modelHelper.accessor('billing_type', {
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Billing Type')} />
      ),
      cell: ({ getValue }) => getValue(),
    }),
    modelHelper.accessor('request_count', {
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Requests')} />
      ),
      cell: ({ getValue }) => getValue().toLocaleString(),
    }),
    modelHelper.accessor('total_quota', {
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Quota')} />
      ),
      cell: ({ getValue }) => getValue().toLocaleString(),
    }),
    modelHelper.accessor('estimated_cny', {
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Cost (CNY)')} />
      ),
      cell: ({ getValue }) => `¥${getValue().toFixed(4)}`,
    }),
    modelHelper.accessor('total_tokens', {
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Total Tokens')} />
      ),
      cell: ({ getValue }) => getValue().toLocaleString(),
    }),
  ]
}

// ============================================================================
// Token Daily Total Columns
// ============================================================================

const totalHelper = createColumnHelper<TokenDailyTotalItem>()

export function useTokenDailyTotalColumns() {
  const { t } = useTranslation()

  return [
    totalHelper.accessor('stat_date', {
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Date')} />
      ),
      cell: ({ getValue }) => getValue(),
      enableSorting: true,
    }),
    totalHelper.accessor('token_name', {
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Token Name')} />
      ),
      cell: ({ getValue }) => getValue(),
    }),
    totalHelper.accessor('token_key', {
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Token Key')} />
      ),
      cell: ({ getValue }) => {
        const val = getValue()
        return val ? `${val.substring(0, 8)}...${val.slice(-4)}` : '-'
      },
    }),
    totalHelper.accessor('token_group', {
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Group')} />
      ),
      cell: ({ getValue }) => getValue(),
    }),
    totalHelper.accessor('models_used', {
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Models Used')} />
      ),
      cell: ({ getValue }) => getValue(),
    }),
    totalHelper.accessor('distinct_models', {
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Distinct Models')} />
      ),
      cell: ({ getValue }) => getValue(),
      enableSorting: false,
      enableHiding: false,
    }),
    totalHelper.accessor('total_requests', {
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Total Requests')} />
      ),
      cell: ({ getValue }) => getValue().toLocaleString(),
      enableSorting: false,
      enableHiding: false,
    }),
    totalHelper.accessor('total_quota', {
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Total Quota')} />
      ),
      cell: ({ getValue }) => getValue().toLocaleString(),
      enableSorting: false,
      enableHiding: false,
    }),
    totalHelper.accessor('estimated_cny', {
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Cost (CNY)')} />
      ),
      cell: ({ getValue }) => `¥${getValue().toFixed(4)}`,
    }),
    totalHelper.accessor('total_tokens', {
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Total Tokens')} />
      ),
      cell: ({ getValue }) => getValue().toLocaleString(),
    }),
  ]
}
