import { useEffect, useMemo, useState } from 'react'
import { fetchSnapshot, isUnauthorizedError, loadConnectionSettings, saveConnectionSettings, type ApiSnapshot, type ConnectionSettings } from './lib/api'
import { useI18n } from './lib/i18n'
import { useTheme } from './lib/theme'
import { StatusPage } from './pages/status-page'
import { SourcesPage } from './pages/sources-page'
import { NodesPage } from './pages/nodes-page'
import { HealthPage } from './pages/health-page'
import { LogsPage } from './pages/logs-page'
import { ShellCard } from './components/shell-card'
import { Badge } from './components/badge'

type NavKey = 'status' | 'sources' | 'nodes' | 'candidates' | 'health' | 'logs'

const navKeys: NavKey[] = ['status', 'sources', 'nodes', 'candidates', 'health', 'logs']

export function App() {
  const { locale, setLocale, t } = useI18n()
  const { theme, toggleTheme } = useTheme()
  const [active, setActive] = useState<NavKey>('status')
  const [connection, setConnection] = useState<ConnectionSettings>(() => loadConnectionSettings())
  const [draftConnection, setDraftConnection] = useState<ConnectionSettings>(() => loadConnectionSettings())
  const [snapshot, setSnapshot] = useState<ApiSnapshot | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string>('')
  const [lastRefresh, setLastRefresh] = useState<string>('')

  const navLabels = useMemo<Record<NavKey, string>>(
    () => ({
      status: t('navStatus'),
      sources: t('navSources'),
      nodes: t('navNodes'),
      candidates: t('navCandidates'),
      health: t('navHealth'),
      logs: t('navLogs'),
    }),
    [t],
  )

  const connectionStatusTone = error ? 'bad' : snapshot ? 'good' : 'warn'
  const connectionStatusLabel = snapshot && !error ? t('online') : t('offline')

  useEffect(() => {
    void refresh(connection)
  }, [connection])

  async function refresh(settings: ConnectionSettings) {
    setLoading(true)
    setError('')
    try {
      const nextSnapshot = await fetchSnapshot(settings)
      setSnapshot(nextSnapshot)
      setLastRefresh(new Date().toISOString())
    } catch (requestError) {
      setSnapshot(null)
      setError(isUnauthorizedError(requestError) ? t('unauthorized') : requestError instanceof Error ? requestError.message : t('failed'))
    } finally {
      setLoading(false)
    }
  }

  function applyConnection() {
    saveConnectionSettings(draftConnection)
    setConnection(draftConnection)
  }

  return (
    <div className="min-h-screen px-4 py-5 sm:px-6 lg:px-8 xl:px-10 xl:py-8">
      <div className="mx-auto max-w-[1540px] rounded-[42px] border border-white/45 bg-shell p-3 shadow-shell backdrop-blur md:p-4 xl:p-5">
        <div className="grid gap-4 xl:grid-cols-[minmax(0,1.72fr)_340px] 2xl:grid-cols-[minmax(0,1.78fr)_360px]">
          <main className="space-y-8 rounded-[36px] bg-shell-strong px-5 py-6 md:px-7 md:py-7 xl:px-8 xl:py-8">
            <header className="space-y-6 border-b border-line-soft/70 pb-7">
              <div className="flex flex-col gap-6 xl:flex-row xl:items-start xl:justify-between">
                <div className="max-w-3xl">
                  <div className="text-xs uppercase tracking-[0.34em] text-text-soft">GeoLoom</div>
                  <h1 className="mt-3 font-display text-[2.7rem] leading-none tracking-[-0.06em] text-text-main md:text-[3.2rem]">{t('appTitle')}</h1>
                  <p className="mt-3 max-w-2xl text-sm leading-7 text-text-soft">{t('appSubtitle')}</p>
                </div>
                <div className="flex items-center gap-3 self-start rounded-full bg-card-muted px-4 py-3 text-sm text-text-soft">
                  <span>{t('lastRefresh')}</span>
                  <span className="font-medium text-text-main">{formatDate(lastRefresh)}</span>
                </div>
              </div>

              <div className="flex flex-col gap-4 xl:flex-row xl:items-center xl:justify-between">
                <nav className="flex flex-wrap gap-2.5">
                  {navKeys.map((key) => (
                    <button
                      key={key}
                      onClick={() => setActive(key)}
                      className={`rounded-full px-4 py-2.5 text-sm transition ${active === key ? 'bg-text-main text-white dark:text-slate-900 shadow-card-soft' : 'bg-card text-text-soft hover:text-text-main'}`}
                    >
                      {navLabels[key]}
                    </button>
                  ))}
                </nav>
                <button onClick={() => void refresh(connection)} className="self-start rounded-full bg-accent-main px-5 py-2.5 text-sm font-medium text-white shadow-card-soft">
                  {t('refresh')}
                </button>
              </div>
            </header>

            {loading ? <StateCard label={t('loading')} /> : error ? <StateCard label={error} tone="bad" /> : snapshot ? renderPage(active, snapshot, t) : <StateCard label={t('empty')} />}
          </main>

          <aside className="space-y-4 rounded-[36px] bg-sidebar p-4 md:p-5 xl:space-y-5 xl:p-5">
            <ShellCard tone="muted" className="overflow-hidden">
              <div className="flex items-start justify-between gap-4">
                <div>
                  <div className="text-sm text-text-soft">{t('connection')}</div>
                  <div className="mt-1 font-display text-[1.9rem] tracking-[-0.04em] text-text-main">API</div>
                </div>
                <Badge tone={connectionStatusTone}>{connectionStatusLabel}</Badge>
              </div>

              <div className="mt-5 rounded-[24px] bg-shell-strong px-4 py-4 text-sm text-text-soft">
                <div className="flex items-center justify-between gap-3">
                  <span>{t('apiBase')}</span>
                  <span className="max-w-[11rem] truncate text-right text-text-main">{draftConnection.baseUrl.trim() || '/'}</span>
                </div>
              </div>

              <div className="mt-5 space-y-4 text-sm">
                <Field label={t('apiBase')}>
                  <input className="field" value={draftConnection.baseUrl} onChange={(event) => setDraftConnection((current) => ({ ...current, baseUrl: event.target.value }))} placeholder="http://127.0.0.1:9090" />
                </Field>
                <Field label={t('authHeader')}>
                  <input className="field" value={draftConnection.authHeader} onChange={(event) => setDraftConnection((current) => ({ ...current, authHeader: event.target.value }))} />
                </Field>
                <Field label={t('token')}>
                  <input className="field" type="password" value={draftConnection.token} onChange={(event) => setDraftConnection((current) => ({ ...current, token: event.target.value }))} />
                </Field>
                <button onClick={applyConnection} className="w-full rounded-full bg-text-main px-4 py-3 text-sm font-medium text-white shadow-card-soft dark:text-slate-900">
                  {t('save')}
                </button>
              </div>
            </ShellCard>

            <ShellCard>
              <div className="text-sm text-text-soft">{t('preferences')}</div>
              <div className="mt-4">
                <div className="text-xs uppercase tracking-[0.18em] text-text-soft/80">{t('language')}</div>
                <div className="mt-3 flex gap-2">
                  {(['zh', 'en'] as const).map((item) => (
                    <button key={item} onClick={() => setLocale(item)} className={`rounded-full px-4 py-2 text-sm ${locale === item ? 'bg-accent-main text-white shadow-card-soft' : 'bg-card-muted text-text-soft'}`}>
                      {item.toUpperCase()}
                    </button>
                  ))}
                </div>
              </div>

              <div className="mt-5">
                <div className="text-xs uppercase tracking-[0.18em] text-text-soft/80">{t('theme')}</div>
                <button onClick={toggleTheme} className="mt-3 w-full rounded-[22px] bg-card-muted px-4 py-3 text-left text-sm text-text-main">
                  {theme === 'light' ? t('themeLight') : t('themeDark')}
                </button>
              </div>
            </ShellCard>

            <ShellCard tone="accent">
              <div className="text-sm text-text-soft">{t('runtimeNoteTitle')}</div>
              <div className="mt-2 font-display text-[1.8rem] leading-none tracking-[-0.04em] text-text-main">{formatDate(lastRefresh)}</div>
              <p className="mt-4 text-sm leading-7 text-text-soft">{t('rightRailNote')}</p>
              {error ? <div className="mt-4 rounded-[18px] bg-white/45 px-4 py-3 text-sm text-accent-coral dark:bg-black/10">{error}</div> : null}
            </ShellCard>
          </aside>
        </div>
      </div>
    </div>
  )
}

function renderPage(active: NavKey, snapshot: ApiSnapshot, t: (key: string, args?: Record<string, string | number>) => string) {
  switch (active) {
    case 'status':
      return <StatusPage snapshot={snapshot} />
    case 'sources':
      return <SourcesPage sources={snapshot.sources} />
    case 'nodes':
      return <NodesPage title={t('nodesTitle')} nodes={snapshot.nodes} variant="nodes" />
    case 'candidates':
      return <NodesPage title={t('candidatesTitle')} nodes={snapshot.candidates} variant="candidates" />
    case 'health':
      return <HealthPage health={snapshot.health} />
    case 'logs':
      return <LogsPage logs={snapshot.logs} />
    default:
      return null
  }
}

function StateCard({ label, tone = 'neutral' }: { label: string; tone?: 'neutral' | 'bad' }) {
  return (
    <div className={`rounded-[32px] border px-6 py-14 text-center text-sm ${tone === 'bad' ? 'border-accent-coral/25 bg-accent-coral/10 text-accent-coral' : 'border-line-soft bg-card text-text-soft'}`}>
      {label}
    </div>
  )
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="block">
      <div className="mb-2 text-text-soft">{label}</div>
      {children}
    </label>
  )
}

function formatDate(value: string) {
  if (!value) return '—'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString()
}
