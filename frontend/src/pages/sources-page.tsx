import type { SourcesResponse } from '../lib/api'
import { useI18n } from '../lib/i18n'
import { Badge } from '../components/badge'
import { DataTable, type Column } from '../components/data-table'

export function SourcesPage({ sources }: { sources: SourcesResponse }) {
  const { t } = useI18n()

  const columns: Column<SourcesResponse['items'][number]>[] = [
    { key: 'name', header: t('sourceName'), render: (row) => <div className="font-medium">{row.name}</div> },
    { key: 'type', header: t('sourceType'), render: (row) => row.input_type },
    { key: 'node_count', header: t('nodeCount'), render: (row) => row.node_count },
    { key: 'unsupported_count', header: t('unsupportedCount'), render: (row) => row.unsupported_count },
    { key: 'success', header: t('success'), render: (row) => <Badge tone={row.success ? 'good' : 'bad'}>{row.success ? t('yes') : t('no')}</Badge> },
    { key: 'updated_at', header: t('updatedAt'), render: (row) => formatDate(row.updated_at) },
    { key: 'details', header: t('details'), render: (row) => <div className="max-w-72 text-text-soft">{row.error ?? row.normalized_url}</div> },
  ]

  return (
    <DataTable
      title={t('sourceStatus')}
      items={sources.items}
      columns={columns}
      searchableText={(row) => `${row.name} ${row.input_type} ${row.normalized_url} ${row.error ?? ''}`}
      getRowKey={(row) => `${row.name}-${row.updated_at}`}
    />
  )
}

function formatDate(value: string) {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString()
}
