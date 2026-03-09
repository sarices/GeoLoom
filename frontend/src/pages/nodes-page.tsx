import type { NodesResponse } from '../lib/api'
import { useI18n } from '../lib/i18n'
import { Badge } from '../components/badge'
import { DataTable, type Column } from '../components/data-table'

export function NodesPage({ title, nodes }: { title: string; nodes: NodesResponse }) {
  const { t } = useI18n()

  const columns: Column<NodesResponse['items'][number]>[] = [
    { key: 'name', header: t('sourceName'), render: (row) => <div className="font-medium">{row.name}</div> },
    { key: 'protocol', header: t('protocol'), render: (row) => <Badge tone="neutral">{row.protocol}</Badge> },
    { key: 'country', header: t('country'), render: (row) => row.country_code || '—' },
    { key: 'address', header: t('address'), render: (row) => row.address },
    { key: 'port', header: t('port'), render: (row) => row.port },
    { key: 'source_names', header: t('sourceNames'), render: (row) => <div className="max-w-52 text-text-soft">{row.source_names.join(', ')}</div> },
    { key: 'fingerprint', header: t('fingerprint'), render: (row) => <code className="text-xs text-text-soft">{row.fingerprint}</code> },
  ]

  return (
    <DataTable
      title={title}
      items={nodes.items}
      columns={columns}
      searchableText={(row) => `${row.name} ${row.protocol} ${row.country_code} ${row.address} ${row.source_names.join(' ')}`}
      getRowKey={(row) => row.fingerprint || row.id}
    />
  )
}
