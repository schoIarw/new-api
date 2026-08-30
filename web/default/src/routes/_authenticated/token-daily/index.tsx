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
import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'

import { TokenDailyPage } from '@/features/token-daily/token-daily-page'

const tokenDailySearchSchema = z.object({
  start_date: z.string().optional().catch(''),
  end_date: z.string().optional().catch(''),
  token_key: z.string().optional().catch(''),
  view: z.enum(['model', 'total']).optional().catch('total'),
  page: z.number().optional().catch(1),
  page_size: z.number().optional().catch(20),
})

export const Route = createFileRoute('/_authenticated/token-daily/')({
  validateSearch: tokenDailySearchSchema,
  component: TokenDailyPage,
})
