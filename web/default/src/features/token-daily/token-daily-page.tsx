import { getRouteApi, useNavigate } from "@tanstack/react-router"
import {
  flexRender,
  getCoreRowModel,
  getPaginationRowModel,
  useReactTable,
} from "@tanstack/react-table"
import { useCallback, useEffect, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { DataTablePagination } from "@/components/data-table"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { SectionPageLayout } from "@/components/layout"
import { useMinimumLoadingTime } from "@/hooks/use-minimum-loading-time"
import {
  getAllTokenDailyModel,
  getAllTokenDailyTotal,
  getUserTokenDailyModel,
  getUserTokenDailyTotal,
} from "./api"
import { TokenDailyFilterBar, TokenDailyViewToggle } from "./components"
import {
  useTokenDailyModelColumns,
  useTokenDailyTotalColumns,
} from "./lib"
import type { TokenDailyModelItem, TokenDailyTotalItem } from "./types"

const route = getRouteApi("/_authenticated/token-daily/")

function getDefaultDateRange() {
  const now = new Date()
  const start = new Date(now)
  start.setDate(start.getDate() - 30)
  const fmt = (d: Date) => {
    const y = d.getFullYear()
    const m = String(d.getMonth() + 1).padStart(2, "0")
    const day = String(d.getDate()).padStart(2, "0")
    return y + "-" + m + "-" + day
  }
  return { startDate: fmt(start), endDate: fmt(now) }
}

function SummaryCards({ data, view, t }: { data: (TokenDailyModelItem | TokenDailyTotalItem)[]; view: string; t: (key: string) => string }) {
  const stats = useMemo(() => {
    const totalRequests = data.reduce((sum, item) => sum + (Number((item as any).request_count) || Number((item as any).total_requests) || 0), 0)
    const totalQuota = data.reduce((sum, item) => sum + (Number((item as any).total_quota) || 0), 0)
    const totalTokens = data.reduce((sum, item) => sum + (Number((item as any).total_tokens) || 0), 0)
    const models = new Set(data.map((item) => (item as any).model_name).filter(Boolean))
    const tokens = new Set(data.map((item) => (item as any).token_name).filter(Boolean))
    return { totalRequests, totalQuota, totalTokens, modelCount: models.size, tokenCount: tokens.size }
  }, [data])

  if (data.length === 0) return null

  return (
    <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5">
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-xs font-medium text-muted-foreground">{t("Total Requests")}</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="text-xl font-bold tabular-nums">{stats.totalRequests.toLocaleString()}</div>
        </CardContent>
      </Card>
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-xs font-medium text-muted-foreground">{t("Total Quota")}</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="text-xl font-bold tabular-nums">{stats.totalQuota.toLocaleString()}</div>
        </CardContent>
      </Card>
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-xs font-medium text-muted-foreground">{t("Total Tokens")}</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="text-xl font-bold tabular-nums">{stats.totalTokens.toLocaleString()}</div>
        </CardContent>
      </Card>
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-xs font-medium text-muted-foreground">{t("Models Used")}</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="text-xl font-bold tabular-nums">{stats.modelCount}</div>
        </CardContent>
      </Card>
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-xs font-medium text-muted-foreground">{t("Total Items")}</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="text-xl font-bold tabular-nums">{data.length}</div>
        </CardContent>
      </Card>
    </div>
  )
}

export function TokenDailyPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const search = route.useSearch()
  const [view, setView] = useState(search.view || "total")
  const [startDate, setStartDate] = useState(search.start_date || getDefaultDateRange().startDate)
  const [endDate, setEndDate] = useState(search.end_date || getDefaultDateRange().endDate)
  const [tokenKey, setTokenKey] = useState(search.token_key || "")
  const [data, setData] = useState<(TokenDailyModelItem | TokenDailyTotalItem)[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const minLoading = useMinimumLoadingTime(loading, 300)
  const [pagination, setPagination] = useState({
    pageIndex: (search.page || 1) - 1,
    pageSize: search.page_size || 20,
  })
  const modelColumns = useTokenDailyModelColumns()
  const totalColumns = useTokenDailyTotalColumns()
  const columns = view === "model" ? modelColumns : totalColumns
  const isAdmin = true

  const fetchData = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const params = {
        start_date: startDate, end_date: endDate,
        token_key: tokenKey || undefined,
        page: pagination.pageIndex + 1, page_size: pagination.pageSize,
      }
      const res = view === "model"
        ? (isAdmin ? await getAllTokenDailyModel(params) : await getUserTokenDailyModel(params))
        : (isAdmin ? await getAllTokenDailyTotal(params) : await getUserTokenDailyTotal(params))
      if (res.success && res.data) {
        setData(res.data.items as (TokenDailyModelItem | TokenDailyTotalItem)[])
        setTotal(res.data.total)
      } else {
        setError(res.message || t("Failed to load data"))
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : t("Failed to load data"))
    } finally {
      setLoading(false)
    }
  }, [view, startDate, endDate, tokenKey, pagination, isAdmin, t])

  useEffect(() => { void fetchData() }, [fetchData])

  useEffect(() => {
    void navigate({
      to: "/token-daily",
      search: {
        start_date: startDate || undefined, end_date: endDate || undefined,
        token_key: tokenKey || undefined, view,
        page: pagination.pageIndex + 1 > 1 ? pagination.pageIndex + 1 : undefined,
        page_size: pagination.pageSize !== 20 ? pagination.pageSize : undefined,
      },
      replace: true,
    })
  }, [startDate, endDate, tokenKey, view, pagination, navigate])

  const table = useReactTable({
    data, columns,
    pageCount: Math.ceil(total / pagination.pageSize),
    state: { pagination },
    onPaginationChange: setPagination,
    getCoreRowModel: getCoreRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    manualPagination: true,
  })

  const handleSearch = useCallback(() => { setPagination((prev) => ({ ...prev, pageIndex: 0 })) }, [])
  const handleReset = useCallback(() => {
    const range = getDefaultDateRange()
    setStartDate(range.startDate); setEndDate(range.endDate); setTokenKey("")
    setPagination((prev) => ({ ...prev, pageIndex: 0 }))
  }, [])
  const handleViewChange = useCallback((newView: string) => {
    setView(newView)
    setData([])
    setTotal(0)
    setPagination((prev) => ({ ...prev, pageIndex: 0 }))
  }, [])

  return (
    <SectionPageLayout fixedContent>
      <SectionPageLayout.Title>{t("Token Daily Stats")}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <TokenDailyViewToggle value={view} onChange={handleViewChange} />
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className="flex h-full min-h-0 flex-col gap-4">
          <TokenDailyFilterBar startDate={startDate} endDate={endDate} tokenKey={tokenKey}
            onStartDateChange={setStartDate} onEndDateChange={setEndDate} onTokenKeyChange={setTokenKey}
            onSearch={handleSearch} onReset={handleReset} />
          {error && (<div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">{error}</div>)}
          {!minLoading && data.length > 0 && (
            <SummaryCards data={data} view={view} t={t} />
          )}
          <div className="min-h-0 flex-1">
            <div className="rounded-md border">
              <Table>
                <TableHeader>
                  {table.getHeaderGroups().map((hg) => (
                    <TableRow key={hg.id}>
                      {hg.headers.map((h) => (
                        <TableHead key={h.id}>{h.isPlaceholder ? null : flexRender(h.column.columnDef.header, h.getContext())}</TableHead>
                      ))}
                    </TableRow>
                  ))}
                </TableHeader>
                <TableBody>
                  {minLoading ? Array.from({ length: 5 }).map((_, i) => (
                    <TableRow key={i}>
                      {columns.map((col) => (<TableCell key={col.id}><div className="h-4 w-full animate-pulse rounded bg-muted"/></TableCell>))}
                    </TableRow>
                  )) : data.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={columns.length} className="h-32 text-center">
                        <div className="flex flex-col items-center gap-2 text-muted-foreground">
                          <div className="text-4xl">📊</div>
                          <div>{t("No results")}</div>
                          <div className="text-xs">{t("Date Range")}: {startDate} ~ {endDate}</div>
                        </div>
                      </TableCell>
                    </TableRow>
                  ) : table.getRowModel().rows.map((row) => (
                    <TableRow key={row.id}>
                      {row.getVisibleCells().map((cell) => (<TableCell key={cell.id}>{flexRender(cell.column.columnDef.cell, cell.getContext())}</TableCell>))}
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
            <DataTablePagination table={table} />
          </div>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
