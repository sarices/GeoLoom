import { useEffect, useState } from 'react'
import type { NodesResponse } from '../lib/api'
import { useI18n } from '../lib/i18n'
import { Badge } from '../components/badge'
import { DataTable, type Column } from '../components/data-table'
import { MetricCard } from '../components/metric-card'

type CandidateTier = 'primary' | 'stable' | 'observe' | 'fallback'

export function NodesPage({ title, nodes, variant = 'nodes' }: { title: string; nodes: NodesResponse; variant?: 'nodes' | 'candidates' }) {
  const { t } = useI18n()
  const [activeTier, setActiveTier] = useState<CandidateTier | null>(null)

  useEffect(() => {
    if (variant !== 'candidates' || !activeTier) {
      return
    }

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setActiveTier(null)
      }
    }

    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [activeTier, variant])

  const sortedItems = variant === 'candidates' ? [...nodes.items].sort((left, right) => (right.health_score ?? -1) - (left.health_score ?? -1)) : nodes.items
  const primaryItems = sortedItems.filter((item) => typeof item.health_score === 'number' && item.health_score >= 90)
  const stableItems = sortedItems.filter((item) => typeof item.health_score === 'number' && item.health_score >= 70 && item.health_score < 90)
  const observeItems = sortedItems.filter((item) => typeof item.health_score === 'number' && item.health_score >= 40 && item.health_score < 70)
  const fallbackItems = sortedItems.filter((item) => typeof item.health_score === 'number' && item.health_score < 40)
  const filteredItems = variant === 'candidates'
    ? activeTier === 'primary'
      ? primaryItems
      : activeTier === 'stable'
        ? stableItems
        : activeTier === 'observe'
          ? observeItems
          : activeTier === 'fallback'
            ? fallbackItems
            : sortedItems
    : sortedItems
  const activeTierLabel = activeTier === 'primary'
    ? t('tierPrimary')
    : activeTier === 'stable'
      ? t('tierStable')
      : activeTier === 'observe'
        ? t('tierObserve')
        : activeTier === 'fallback'
          ? t('tierFallback')
          : ''
  const columns: Column<NodesResponse['items'][number]>[] = [
    { key: 'name', header: t('sourceName'), render: (row) => <div className="font-medium">{row.name}</div> },
    { key: 'protocol', header: t('protocol'), render: (row) => <Badge tone="neutral">{row.protocol}</Badge> },
    { key: 'country', header: t('country'), render: (row) => row.country_code || '—' },
    { key: 'address', header: t('address'), render: (row) => row.address },
    { key: 'port', header: t('port'), render: (row) => row.port },
  ]

  if (variant === 'candidates') {
    columns.unshift({
      key: 'rank',
      header: t('candidateRank'),
      render: (row) => {
        const rank = sortedItems.findIndex((item) => (item.fingerprint || item.id) === (row.fingerprint || row.id)) + 1
        return <Badge tone={rank <= 3 ? 'good' : rank <= 6 ? 'warn' : 'neutral'}>#{rank}</Badge>
      },
    })

    columns.push(
      {
        key: 'quality',
        header: t('quality'),
        render: (row) => {
          if (typeof row.health_score !== 'number') return '—'
          return <span className="font-medium">{row.health_score}</span>
        },
      },
      {
        key: 'quality_tier',
        header: t('qualityTier'),
        render: (row) => {
          if (typeof row.health_score !== 'number') return <Badge tone="neutral">—</Badge>
          if (row.health_score >= 90) return <Badge tone="good">{t('tierPrimary')}</Badge>
          if (row.health_score >= 70) return <Badge tone="good">{t('tierStable')}</Badge>
          if (row.health_score >= 40) return <Badge tone="warn">{t('tierObserve')}</Badge>
          return <Badge tone="bad">{t('tierFallback')}</Badge>
        },
      },
      {
        key: 'quality_state',
        header: t('qualityState'),
        render: (row) => {
          if (typeof row.health_score !== 'number') return <Badge tone="neutral">—</Badge>
          return <Badge tone={row.health_score >= 80 ? 'good' : 'warn'}>{row.health_score >= 80 ? t('ready') : t('degraded')}</Badge>
        },
      },
    )
  }

  columns.push(
    { key: 'source_names', header: t('sourceNames'), render: (row) => <div className="max-w-52 text-text-soft">{row.source_names.join(', ')}</div> },
    { key: 'fingerprint', header: t('fingerprint'), render: (row) => <code className="text-xs text-text-soft">{row.fingerprint}</code> },
  )

  return (
    <div className="space-y-6">
      {variant === 'candidates' ? (
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          <MetricCard label={t('tierPrimary')} value={primaryItems.length} accent="mint" hint={t('tierPrimaryHint')} active={activeTier === 'primary'} onClick={() => setActiveTier((current) => (current === 'primary' ? null : 'primary'))} />
          <MetricCard label={t('tierStable')} value={stableItems.length} accent="mint" hint={t('tierStableHint')} active={activeTier === 'stable'} onClick={() => setActiveTier((current) => (current === 'stable' ? null : 'stable'))} />
          <MetricCard label={t('tierObserve')} value={observeItems.length} accent="gold" hint={t('tierObserveHint')} active={activeTier === 'observe'} onClick={() => setActiveTier((current) => (current === 'observe' ? null : 'observe'))} />
          <MetricCard label={t('tierFallback')} value={fallbackItems.length} accent="coral" hint={t('tierFallbackHint')} active={activeTier === 'fallback'} onClick={() => setActiveTier((current) => (current === 'fallback' ? null : 'fallback'))} />
        </div>
      ) : null}

      <DataTable
        title={title}
        description={variant === 'candidates' ? t('candidatesHint') : undefined}
        titleAction={variant === 'candidates' && activeTier ? (
          <div className="flex flex-wrap items-center justify-end gap-2">
            <span className="text-xs text-text-soft">{t('currentFilter')}</span>
            <Badge tone="good">{activeTierLabel}</Badge>
            <button type="button" onClick={() => setActiveTier(null)} className="rounded-full bg-card-muted px-3 py-1.5 text-xs font-medium text-text-main transition hover:bg-card">
              {t('clearFilter')}
            </button>
            <span className="rounded-full bg-shell px-2.5 py-1 text-[11px] font-medium text-text-soft">{t('clearFilterHotkey')}</span>
          </div>
        ) : undefined}
        emptyLabel={variant === 'candidates' && activeTier && filteredItems.length === 0 ? t('emptyCandidatesForFilter') : undefined}
        items={filteredItems}
        columns={columns}
        searchableText={(row) => `${row.name} ${row.protocol} ${row.country_code} ${row.address} ${row.source_names.join(' ')} ${row.health_score ?? ''}`}
        getRowKey={(row) => row.fingerprint || row.id}
      />
    </div>
  )
}
