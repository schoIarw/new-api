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

import type { TokenDailyApiResponse, TokenDailyQueryParams } from './types'

function buildTokenDailyParams(
  params: TokenDailyQueryParams
): Record<string, string | number> {
  const query: Record<string, string | number> = {}

  if (params.page) query.p = params.page
  if (params.page_size) query.page_size = params.page_size
  if (params.start_date) query.start_date = params.start_date
  if (params.end_date) query.end_date = params.end_date
  if (params.token_key) query.token_key = params.token_key

  return query
}

// ============================================================================
// Admin APIs
// ============================================================================

export async function getAllTokenDailyModel(
  params: TokenDailyQueryParams = {}
): Promise<TokenDailyApiResponse> {
  const query = buildTokenDailyParams(params)
  const res = await api.get('/api/token-daily/model', { params: query })
  return res.data
}

export async function getAllTokenDailyTotal(
  params: TokenDailyQueryParams = {}
): Promise<TokenDailyApiResponse> {
  const query = buildTokenDailyParams(params)
  const res = await api.get('/api/token-daily/total', { params: query })
  return res.data
}

// ============================================================================
// User Self APIs
// ============================================================================

export async function getUserTokenDailyModel(
  params: TokenDailyQueryParams = {}
): Promise<TokenDailyApiResponse> {
  const query = buildTokenDailyParams(params)
  const res = await api.get('/api/token-daily/model/self', { params: query })
  return res.data
}

export async function getUserTokenDailyTotal(
  params: TokenDailyQueryParams = {}
): Promise<TokenDailyApiResponse> {
  const query = buildTokenDailyParams(params)
  const res = await api.get('/api/token-daily/total/self', { params: query })
  return res.data
}
