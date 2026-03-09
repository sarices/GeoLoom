import type { LogsResponse } from '../lib/api'
import { useI18n } from '../lib/i18n'
import { MetricCard } from '../components/metric-card'
import { JsonPanel } from '../components/json-panel'
import { ShellCard, SectionTitle } from '../components/shell-card'
import { Badge } from '../components/badge'

export function LogsPage({ logs }: { logs: LogsResponse }) {
  const { t } = useI18n()
  const latest = logs.items.at(-1)

  return (
    <div className="space-y-7">
      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
        <MetricCard label={t('logCount')} value={logs.count} accent="mint" hint={t('logsTitle')} />
        <MetricCard label={t('logCapacity')} value={logs.capacity} accent="gold" hint={t('logLatestHint')} />
        <MetricCard label={t('logTruncated')} value={logs.truncated ? t('yes') : t('no')} accent={logs.truncated ? 'coral' : 'mint'} hint={latest ? formatDate(latest.time) : '—'} />
      </div>

      <div className="grid gap-6 xl:grid-cols-[1.2fr_0.8fr]">
        <ShellCard>
          <SectionTitle title={t('logsTitle')} description={t('logsHint')} action={latest ? <Badge tone="good">{latest.level}</Badge> : undefined} />
          {logs.items.length === 0 ? (
            <div className="rounded-[24px] bg-card-muted px-4 py-12 text-sm text-text-soft">{t('logEmpty')}</div>
          ) : (
            <div className="max-h-[36rem] space-y-3 overflow-y-auto pr-1">
              {logs.items.map((item, index) => (
                <div key={`${item.time}-${index}`} className="rounded-[22px] bg-card-muted px-4 py-4">
                  <div className="flex flex-wrap items-center gap-3 text-xs text-text-soft">
                    <Badge tone={badgeTone(item.level)}>{item.level}</Badge>
                    <span>{formatDate(item.time)}</span>
                  </div>
                  <pre className="mt-3 overflow-x-auto whitespace-pre-wrap break-words rounded-[18px] bg-shell-strong p-4 text-xs leading-6 text-text-main">{item.text}</pre>
                </div>
              ))}
            </div>
          )}
        </ShellCard>

        <JsonPanel title={t('logAttrs')} description={latest ? latest.message : t('empty')} data={latest?.attrs ?? {}} />
      </div>
    </div>
  )
}

function badgeTone(level: string): 'good' | 'warn' | 'bad' | 'neutral' {
  switch (level.toUpperCase()) {
    case 'ERROR':
      return 'bad'
    case 'WARN':
      return 'warn'
    case 'INFO':
      return 'good'
    default:
      return 'neutral'
  }
}

function formatDate(value: string) {
  if (!value) return '—'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString()
}
