import { useMemo, useState, type ReactNode } from 'react'
import { useI18n } from '../lib/i18n'
import { ShellCard, SectionTitle } from './shell-card'

export type Column<T> = {
  key: string
  header: string
  render: (row: T) => string | number | ReactNode | null
}

export function DataTable<T>({
  title,
  description,
  items,
  columns,
  searchableText,
  getRowKey,
  titleAction,
  emptyLabel,
}: {
  title: string
  description?: string
  items: T[]
  columns: Column<T>[]
  searchableText: (row: T) => string
  getRowKey: (row: T, index: number) => string | number
  titleAction?: ReactNode
  emptyLabel?: string
}) {
  const { t } = useI18n()
  const [keyword, setKeyword] = useState('')

  const filtered = useMemo(() => {
    const normalized = keyword.trim().toLowerCase()
    if (!normalized) {
      return items
    }
    return items.filter((item) => searchableText(item).toLowerCase().includes(normalized))
  }, [items, keyword, searchableText])

  return (
    <ShellCard className="overflow-hidden p-0">
      <div className="border-b border-line-soft/70 px-6 py-6">
        <SectionTitle
          title={title}
          description={description}
          action={
            <div className="flex w-full flex-col items-stretch gap-3 xl:w-auto xl:items-end">
              {titleAction}
              <input
                value={keyword}
                onChange={(event) => setKeyword(event.target.value)}
                placeholder={t('searchPlaceholder')}
                className="field w-full max-w-60"
              />
            </div>
          }
        />
        <div className="text-sm text-text-soft">{t('pageCount', { count: filtered.length })}</div>
      </div>
      {filtered.length === 0 ? (
        <div className="px-6 py-12 text-sm text-text-soft">{emptyLabel ?? t('empty')}</div>
      ) : (
        <div className="overflow-x-auto px-3 pb-3">
          <table className="min-w-full border-separate border-spacing-y-3 text-left text-sm">
            <thead>
              <tr>
                {columns.map((column) => (
                  <th key={column.key} className="px-4 py-1 font-medium text-text-soft">
                    {column.header}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {filtered.map((row, index) => (
                <tr key={getRowKey(row, index)}>
                  {columns.map((column) => (
                    <td key={column.key} className="bg-card-muted px-4 py-4 align-top text-text-main first:rounded-l-[20px] last:rounded-r-[20px]">
                      {column.render(row)}
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </ShellCard>
  )
}
