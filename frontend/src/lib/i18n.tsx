import { createContext, useContext, useMemo, useState, type PropsWithChildren } from 'react'

export type Locale = 'zh' | 'en'

type DictValue = string | ((args: Record<string, string | number>) => string)

type Dictionary = Record<string, DictValue>

type I18nContextValue = {
  locale: Locale
  setLocale: (locale: Locale) => void
  t: (key: string, args?: Record<string, string | number>) => string
}

const dictionaries: Record<Locale, Dictionary> = {
  zh: {
    appTitle: 'GeoLoom 控制台',
    appSubtitle: '只读管理与运行态观测面板',
    navStatus: '总览',
    navSources: '来源',
    navNodes: '节点',
    navCandidates: '候选',
    navHealth: '健康',
    navLogs: '日志',
    refresh: '刷新',
    loading: '加载中…',
    empty: '暂无数据',
    failed: '请求失败',
    unauthorized: '鉴权失败，请检查 Token/Header。',
    connection: '连接设置',
    preferences: '偏好设置',
    theme: '主题',
    themeLight: '浅色',
    themeDark: '深色',
    token: 'Token',
    authHeader: '鉴权 Header',
    apiBase: 'API Base URL',
    save: '保存',
    lastRefresh: '最近刷新',
    online: '已连接',
    offline: '未连接',
    statusHero: '运行状态概览',
    statusHint: '面向当前只读 API 的轻量控制台。',
    sourceCount: '来源数',
    candidateCount: '候选数',
    trackedNodes: '跟踪节点',
    penalizedNodes: '惩罚节点',
    strategy: '策略',
    startedAt: '启动时间',
    lastRefreshAt: '刷新时间',
    configSummary: '配置摘要',
    latestSources: '最近来源状态',
    coreSummary: 'Core 构建摘要',
    sourceStatus: '来源状态',
    nodesTitle: '节点列表',
    candidatesTitle: '候选节点',
    healthTitle: '健康检查',
    logsTitle: '最近日志',
    logsHint: '展示当前进程最近一段内存日志，采用手动刷新模式。',
    logCount: '日志条数',
    logCapacity: '缓冲容量',
    logTruncated: '已截断',
    logStream: '日志流',
    logEmpty: '当前没有可展示的日志。',
    logLatestHint: '最新日志显示在底部。',
    logAttrs: '结构化字段',

    sourceType: '类型',
    success: '成功',
    protocol: '协议',
    country: '国家/地区',
    address: '地址',
    port: '端口',
    fingerprint: '指纹',
    sourceNames: '来源',
    details: '详情',
    penaltyPool: '惩罚池',
    trackedMap: '跟踪详情',
    yes: '是',
    no: '否',
    updatedAt: '更新时间',
    nodeCount: '节点数',
    unsupportedCount: '不支持数',
    searchPlaceholder: '按名称、协议、国家筛选…',
    healthSummary: '健康摘要',
    runtimeState: '运行状态',
    runtimeNoteTitle: '最近同步',
    rightRailNote: '右侧辅助栏按“连接 → 偏好 → 最近同步”组织，用于快速校验同源页面、授权状态与界面偏好。',
    pageCount: ({ count }) => `共 ${count} 条`,
  },
  en: {
    appTitle: 'GeoLoom Console',
    appSubtitle: 'Read-only management and runtime observability',
    navStatus: 'Status',
    navSources: 'Sources',
    navNodes: 'Nodes',
    navCandidates: 'Candidates',
    navHealth: 'Health',
    navLogs: 'Logs',
    refresh: 'Refresh',
    loading: 'Loading…',
    empty: 'No data',
    failed: 'Request failed',
    unauthorized: 'Unauthorized. Check token and header.',
    connection: 'Connection',
    preferences: 'Preferences',
    theme: 'Theme',
    themeLight: 'Light',
    themeDark: 'Dark',
    token: 'Token',
    authHeader: 'Auth Header',
    apiBase: 'API Base URL',
    save: 'Save',
    lastRefresh: 'Last refresh',
    online: 'Connected',
    offline: 'Offline',
    statusHero: 'Runtime Overview',
    statusHint: 'A lightweight console over the existing read-only API.',
    sourceCount: 'Sources',
    candidateCount: 'Candidates',
    trackedNodes: 'Tracked Nodes',
    penalizedNodes: 'Penalized',
    strategy: 'Strategy',
    startedAt: 'Started At',
    lastRefreshAt: 'Refreshed At',
    configSummary: 'Config Summary',
    latestSources: 'Recent Source State',
    coreSummary: 'Core Summary',
    sourceStatus: 'Source State',
    nodesTitle: 'Node Inventory',
    candidatesTitle: 'Candidate Pool',
    healthTitle: 'Health Checks',
    logsTitle: 'Recent Logs',
    logsHint: 'Shows the latest in-memory logs from the current process with manual refresh only.',
    logCount: 'Log Count',
    logCapacity: 'Buffer Size',
    logTruncated: 'Truncated',
    logStream: 'Log Stream',
    logEmpty: 'No logs available yet.',
    logLatestHint: 'Newest log entries appear at the bottom.',
    logAttrs: 'Structured Fields',

    sourceType: 'Type',
    success: 'Success',
    protocol: 'Protocol',
    country: 'Country',
    address: 'Address',
    port: 'Port',
    fingerprint: 'Fingerprint',
    sourceNames: 'Sources',
    details: 'Details',
    penaltyPool: 'Penalty Pool',
    trackedMap: 'Tracked Details',
    yes: 'Yes',
    no: 'No',
    updatedAt: 'Updated At',
    nodeCount: 'Nodes',
    unsupportedCount: 'Unsupported',
    searchPlaceholder: 'Filter by name, protocol, country…',
    healthSummary: 'Health Summary',
    runtimeState: 'Runtime State',
    runtimeNoteTitle: 'Recent Sync',
    rightRailNote: 'The right rail is organized as connection, preferences, and recent sync so operators can quickly verify same-origin access, auth state, and UI preferences.',
    pageCount: ({ count }) => `${count} items`,
  },
}

const I18nContext = createContext<I18nContextValue | null>(null)

export function I18nProvider({ children }: PropsWithChildren) {
  const [locale, setLocale] = useState<Locale>('zh')

  const value = useMemo<I18nContextValue>(() => ({
    locale,
    setLocale,
    t: (key, args) => {
      const dict = dictionaries[locale][key] ?? dictionaries.zh[key] ?? key
      if (typeof dict === 'function') {
        return dict(args ?? {})
      }
      return dict
    },
  }), [locale])

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>
}

export function useI18n() {
  const context = useContext(I18nContext)
  if (!context) {
    throw new Error('useI18n 必须在 I18nProvider 内使用')
  }
  return context
}
