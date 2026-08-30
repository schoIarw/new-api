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
/**
 * Type definitions for Token Daily feature
 */

// ============================================================================
// Token Daily Model (per key per day per model)
// ============================================================================

export interface TokenDailyModelItem {
  stat_date: string
  token_id: number
  token_name: string
  token_key: string
  token_group: string
  model_name: string
  billing_type: string
  current_model_ratio: number
  current_fixed_price: number
  current_completion_ratio: number
  request_count: number
  total_quota: number
  estimated_usd: number
  estimated_cny: number
  total_prompt_tokens: number
  total_completion_tokens: number
  total_tokens: number
  first_request_at: string
  last_request_at: string
}

// ============================================================================
// Token Daily Total (per key per day, aggregated)
// ============================================================================

export interface TokenDailyTotalItem {
  stat_date: string
  token_id: number
  token_name: string
  token_key: string
  token_group: string
  models_used: string
  distinct_models: number
  total_requests: number
  total_quota: number
  estimated_usd: number
  estimated_cny: number
  total_prompt_tokens: number
  total_completion_tokens: number
  total_tokens: number
  day_first_request: string
  day_last_request: string
}

// ============================================================================
// API Types
// ============================================================================

export interface TokenDailyQueryParams {
  start_date?: string
  end_date?: string
  token_key?: string
  page?: number
  page_size?: number
}

export interface TokenDailyApiResponse {
  success: boolean
  message?: string
  data?: {
    items: TokenDailyModelItem[] | TokenDailyTotalItem[]
    total: number
    page: number
    page_size: number
  }
}

// ============================================================================
// View Types
// ============================================================================

export type TokenDailyView = 'model' | 'total'
