import type { ApiSnapshot } from '../lib/api'
import { useI18n } from '../lib/i18n'
import { MetricCard } from '../components/metric-card'
import { ShellCard, SectionTitle } from '../components/shell-card'
import { Badge } from '../components/badge'

export function StatusPage({ snapshot }: { snapshot: ApiSnapshot }) {
  const { t } = useI18n()
  const { status, sources, health } = snapshot

  return (
    <div className="space-y-7">
      <ShellCard className="overflow-hidden bg-shell-strong">
        <div className="grid gap-6 xl:grid-cols-[1.45fr_0.88fr]">
          <div className="space-y-5">
            <div className="inline-flex rounded-full bg-accent-main/12 px-3 py-1 text-xs font-semibold tracking-[0.14em] uppercase text-accent-main">{t('runtimeState')}</div>
            <div>
              <h1 className="font-display text-[2.55rem] leading-none tracking-[-0.055em] text-text-main md:text-[2.95rem]">{t('statusHero')}</h1>
              <p className="mt-4 max-w-2xl text-sm leading-7 text-text-soft">{t('statusHint')}</p>
            </div>
            <div className="grid gap-3 sm:grid-cols-3">
              <div className="rounded-[24px] bg-card-muted px-4 py-4">
                <div className="text-xs uppercase tracking-[0.18em] text-text-soft/80">{t('strategy')}</div>
                <div className="mt-3"><Badge tone="good">{status.strategy}</Badge></div>
              </div>
              <div className="rounded-[24px] bg-card-muted px-4 py-4">
                <div className="text-xs uppercase tracking-[0.18em] text-text-soft/80">{t('sourceCount')}</div>
                <div className="mt-3 font-display text-3xl leading-none tracking-[-0.04em] text-text-main">{status.source_count}</div>
              </div>
              <div className="rounded-[24px] bg-card-muted px-4 py-4">
                <div className="text-xs uppercase tracking-[0.18em] text-text-soft/80">{t('candidateCount')}</div>
                <div className="mt-3 font-display text-3xl leading-none tracking-[-0.04em] text-text-main">{status.candidate_node_count}</div>
              </div>
            </div>
          </div>

          <div className="rounded-[30px] bg-card-muted p-5">
            <div className="text-sm text-text-soft">{t('configSummary')}</div>
            <div className="mt-5 space-y-3 text-sm text-text-main">
              <div className="flex items-center justify-between rounded-[18px] bg-shell-strong px-4 py-3"><span>{t('startedAt')}</span><span>{formatDate(status.started_at)}</span></div>
              <div className="flex items-center justify-between rounded-[18px] bg-shell-strong px-4 py-3"><span>{t('lastRefreshAt')}</span><span>{formatDate(status.last_refresh_at)}</span></div>
              <div className="flex items-center justify-between rounded-[18px] bg-shell-strong px-4 py-3"><span>API</span><span>{status.api.listen}</span></div>
            </div>
          </div>
        </div>
      </ShellCard>

      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        <MetricCard label={t('sourceCount')} value={status.source_count} accent="mint" hint={`${status.raw_node_count} raw / ${status.deduped_node_count} deduped`} />
        <MetricCard label={t('candidateCount')} value={status.candidate_node_count} accent="gold" hint={`${status.resolved_node_count} resolved`} />
        <MetricCard label={t('trackedNodes')} value={health.summary.tracked_nodes} accent="mint" hint={`${status.core_supported_count} supported`} />
        <MetricCard label={t('penalizedNodes')} value={health.summary.penalized_nodes} accent="coral" hint={`${status.core_unsupported_count} unsupported`} />
      </div>

      <div className="grid gap-6 xl:grid-cols-[1.18fr_0.82fr]">
        <ShellCard>
          <SectionTitle title={t('latestSources')} description={t('sourceStatus')} />
          <div className="space-y-3">
            {sources.items.slice(0, 4).map((item) => (
              <div key={item.name} className="flex items-start justify-between gap-4 rounded-[24px] bg-card-muted p-4">
                <div className="min-w-0">
                  <div className="font-display text-lg text-text-main">{item.name}</div>
                  <div className="mt-1 truncate text-sm text-text-soft">{item.normalized_url}</div>
                </div>
                <div className="text-right text-sm">
                  <Badge tone={item.success ? 'good' : 'bad'}>{item.success ? t('yes') : t('no')}</Badge>
                  <div className="mt-2 text-text-soft">{formatDate(item.updated_at)}</div>
                </div>
              </div>
            ))}
          </div>
        </ShellCard>

        <ShellCard tone="muted">
          <SectionTitle title={t('coreSummary')} description="Runtime pipeline" />
          <div className="space-y-3 text-sm text-text-main">
            <div className="flex items-center justify-between rounded-[20px] bg-shell-strong px-4 py-3"><span>raw</span><span>{status.raw_node_count}</span></div>
            <div className="flex items-center justify-between rounded-[20px] bg-shell-strong px-4 py-3"><span>deduped</span><span>{status.deduped_node_count}</span></div>
            <div className="flex items-center justify-between rounded-[20px] bg-shell-strong px-4 py-3"><span>resolved</span><span>{status.resolved_node_count}</span></div>
            <div className="flex items-center justify-between rounded-[20px] bg-shell-strong px-4 py-3"><span>dropped</span><span>{status.dropped_node_count}</span></div>
          </div>
        </ShellCard>
      </div>
    </div>
  )
}

function formatDate(value: string) {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}
