import type { HealthResponse } from '../lib/api'
import { useI18n } from '../lib/i18n'
import { MetricCard } from '../components/metric-card'
import { JsonPanel } from '../components/json-panel'
import { ShellCard, SectionTitle } from '../components/shell-card'
import { Badge } from '../components/badge'

export function HealthPage({ health }: { health: HealthResponse }) {
  const { t } = useI18n()
  const trackedEntries = Object.entries(health.health.nodes)

  return (
    <div className="space-y-7">
      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        <MetricCard label={t('trackedNodes')} value={health.summary.tracked_nodes} accent="mint" hint={`interval ${health.config.interval}`} />
        <MetricCard label={t('penalizedNodes')} value={health.summary.penalized_nodes} accent="coral" hint={health.config.url} />
        <MetricCard label="debounce" value={health.health.debounce} accent="gold" hint="ns" />
        <MetricCard label="timeout" value={health.health.timeout} accent="gold" hint="ns" />
      </div>

      <div className="grid gap-6 xl:grid-cols-[1.08fr_0.92fr]">
        <ShellCard>
          <SectionTitle title={t('healthSummary')} description={t('healthTitle')} />
          <div className="space-y-3 text-sm text-text-main">
            <div className="flex items-center justify-between rounded-[20px] bg-card-muted px-4 py-3"><span>enabled</span><Badge tone={health.config.enabled ? 'good' : 'bad'}>{health.config.enabled ? t('yes') : t('no')}</Badge></div>
            <div className="flex items-center justify-between rounded-[20px] bg-card-muted px-4 py-3"><span>interval</span><span>{health.config.interval}</span></div>
            <div className="flex items-center justify-between rounded-[20px] bg-card-muted px-4 py-3"><span>last rebuild</span><span>{formatDate(health.summary.last_rebuild_at)}</span></div>
            <div className="rounded-[22px] bg-card-muted px-4 py-4">
              <div className="text-text-soft">last candidates</div>
              <div className="mt-3 flex flex-wrap gap-2">
                {health.health.last_candidates.length === 0 ? <span className="text-text-soft">—</span> : health.health.last_candidates.map((item) => <Badge key={item}>{item}</Badge>)}
              </div>
            </div>
          </div>
        </ShellCard>

        <ShellCard tone="muted">
          <SectionTitle title={t('penaltyPool')} description={trackedEntries.length === 0 ? 'idle' : `${trackedEntries.length} tracked`} />
          <div className="space-y-3">
            {Object.entries(health.penalty_pool).length === 0 ? (
              <div className="rounded-[22px] bg-shell-strong px-4 py-8 text-sm text-text-soft">—</div>
            ) : (
              Object.entries(health.penalty_pool).map(([fingerprint, until]) => (
                <div key={fingerprint} className="rounded-[22px] bg-shell-strong px-4 py-4 text-sm">
                  <div className="break-all font-medium text-text-main">{fingerprint}</div>
                  <div className="mt-2 text-text-soft">{formatDate(until)}</div>
                </div>
              ))
            )}
          </div>
        </ShellCard>
      </div>

      <div className="grid gap-6 xl:grid-cols-[1.02fr_0.98fr]">
        <ShellCard>
          <SectionTitle title={t('trackedMap')} description={trackedEntries.length === 0 ? '—' : `${trackedEntries.length} items`} />
          <div className="space-y-3">
            {trackedEntries.length === 0 ? (
              <div className="rounded-[22px] bg-card-muted px-4 py-8 text-sm text-text-soft">{t('empty')}</div>
            ) : (
              trackedEntries.slice(0, 8).map(([fingerprint, item]) => (
                <div key={fingerprint} className="rounded-[22px] bg-card-muted px-4 py-4 text-sm text-text-main">
                  <div className="break-all font-medium">{fingerprint}</div>
                  <div className="mt-3 flex flex-wrap items-center gap-3 text-text-soft">
                    <Badge tone={item.last_reachable ? 'good' : 'bad'}>{item.last_reachable ? t('yes') : t('no')}</Badge>
                    <span>{formatDate(item.last_check_at)}</span>
                  </div>
                </div>
              ))
            )}
          </div>
        </ShellCard>

        <JsonPanel title={t('trackedMap')} description="raw health payload" data={health.health.nodes} />
      </div>
    </div>
  )
}

function formatDate(value: string) {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString()
}
