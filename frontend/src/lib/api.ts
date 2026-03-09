export type StatusResponse = {
  version: string
  started_at: string
  last_refresh_at: string
  source_count: number
  strategy: string
  raw_node_count: number
  deduped_node_count: number
  resolved_node_count: number
  candidate_node_count: number
  dropped_node_count: number
  core_supported_count: number
  core_unsupported_count: number
  refresh: { enabled: boolean; interval: string }
  api: { enabled: boolean; listen: string }
  state: { enabled: boolean; path: string }
}

export type SourcesResponse = {
  items: Array<{
    name: string
    type: string
    url: string
    normalized_url: string
    input_type: string
    node_count: number
    unsupported_count: number
    success: boolean
    error?: string
    updated_at: string
  }>
}

export type NodeItem = {
  id: string
  fingerprint: string
  name: string
  source_names: string[]
  country_code: string
  protocol: string
  address: string
  port: number
  last_checked: string
  raw_config: Record<string, unknown>
}

export type NodesResponse = {
  count: number
  items: NodeItem[]
}

export type HealthResponse = {
  config: {
    enabled: boolean
    interval: string
    url: string
  }
  summary: {
    tracked_nodes: number
    penalized_nodes: number
    last_rebuild_at: string
  }
  health: {
    interval: number
    debounce: number
    test_url: string
    timeout: number
    last_candidates: string[]
    last_rebuild_at: string
    nodes: Record<string, { last_check_at: string; last_reachable: boolean }>
  }
  penalty_pool: Record<string, string>
}

type RawHealthResponse = {
  config: {
    enabled: boolean
    interval: string
    url: string
  }
  summary: {
    tracked_nodes: number
    penalized_nodes: number
    last_rebuild_at: string
  }
  health: {
    interval: number
    debounce: number
    test_url: string
    timeout: number
    last_candidates: string[] | null
    last_rebuild_at: string
    nodes: Record<string, { last_check_at: string; last_reachable: boolean }> | null
  }
  penalty_pool: Record<string, string> | null
}

export type LogsResponse = {
  items: Array<{
    time: string
    level: string
    message: string
    attrs: Record<string, unknown>
    text: string
  }>
  count: number
  capacity: number
  truncated: boolean
}

export type ApiSnapshot = {
  status: StatusResponse
  sources: SourcesResponse
  nodes: NodesResponse
  candidates: NodesResponse
  health: HealthResponse
  logs: LogsResponse
}

export type ConnectionSettings = {
  baseUrl: string
  token: string
  authHeader: string
}

const STORAGE_KEY = 'geoloom-connection'

const defaultConnection: ConnectionSettings = {
  baseUrl: '',
  token: '',
  authHeader: 'X-GeoLoom-Token',
}

export function loadConnectionSettings(): ConnectionSettings {
  if (typeof window === 'undefined') {
    return defaultConnection
  }
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY)
    if (!raw) {
      return defaultConnection
    }
    const parsed = JSON.parse(raw) as Partial<ConnectionSettings>
    return {
      baseUrl: typeof parsed.baseUrl === 'string' ? parsed.baseUrl : '',
      token: typeof parsed.token === 'string' ? parsed.token : '',
      authHeader: typeof parsed.authHeader === 'string' && parsed.authHeader.trim() !== '' ? parsed.authHeader : 'X-GeoLoom-Token',
    }
  } catch {
    return defaultConnection
  }
}

export function saveConnectionSettings(settings: ConnectionSettings) {
  if (typeof window === 'undefined') {
    return
  }
  window.localStorage.setItem(STORAGE_KEY, JSON.stringify(settings))
}

class ApiError extends Error {
  status: number

  constructor(message: string, status: number) {
    super(message)
    this.status = status
  }
}

function withBase(baseUrl: string, path: string) {
  const normalizedBase = baseUrl.trim().replace(/\/$/, '')
  return normalizedBase ? `${normalizedBase}${path}` : path
}

async function requestJson<T>(path: string, settings: ConnectionSettings): Promise<T> {
  const headers = new Headers()
  if (settings.token.trim() !== '') {
    headers.set(settings.authHeader.trim() || 'X-GeoLoom-Token', settings.token.trim())
  }

  const response = await fetch(withBase(settings.baseUrl, path), { headers })
  if (!response.ok) {
    let message = `HTTP ${response.status}`
    try {
      const payload = (await response.json()) as { error?: string }
      if (payload.error) {
        message = payload.error
      }
    } catch {
      // ignore
    }
    throw new ApiError(message, response.status)
  }
  return (await response.json()) as T
}

function normalizeStringArray(value: string[] | null | undefined) {
  return Array.isArray(value) ? value : []
}

function normalizeRecord<T>(value: Record<string, T> | null | undefined) {
  return value && typeof value === 'object' && !Array.isArray(value) ? value : {}
}

function normalizeHealthResponse(health: RawHealthResponse): HealthResponse {
  return {
    ...health,
    health: {
      ...health.health,
      last_candidates: normalizeStringArray(health.health.last_candidates),
      nodes: normalizeRecord(health.health.nodes),
    },
    penalty_pool: normalizeRecord(health.penalty_pool),
  }
}

export async function fetchSnapshot(settings: ConnectionSettings): Promise<ApiSnapshot> {
  const [status, sources, nodes, candidates, health, logs] = await Promise.all([
    requestJson<StatusResponse>('/api/v1/status', settings),
    requestJson<SourcesResponse>('/api/v1/sources', settings),
    requestJson<NodesResponse>('/api/v1/nodes', settings),
    requestJson<NodesResponse>('/api/v1/candidates', settings),
    requestJson<RawHealthResponse>('/api/v1/health', settings),
    requestJson<LogsResponse>('/api/v1/logs', settings),
  ])

  return { status, sources, nodes, candidates, health: normalizeHealthResponse(health), logs }
}

export function isUnauthorizedError(error: unknown) {
  return error instanceof ApiError && error.status === 401
}
