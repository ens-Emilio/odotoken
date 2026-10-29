/**
 * The single source of truth for translatable strings.
 *
 * `en` implements this interface completely; every other locale is a
 * `Partial<Dict>` and falls back to English key by key. Adding a key here
 * breaks the build until English defines it, which is the point.
 *
 * Keys ending in `.one` / `.other` (and the extra plural categories some
 * languages need) are resolved by `Intl.PluralRules`, never by `n === 1`.
 */
export interface Dict {
  // Navigation
  'nav.overview': string
  'nav.models': string
  'nav.clients': string
  'nav.events': string
  'nav.throughput': string
  'nav.guard': string
  'nav.settings': string
  'nav.import': string
  'nav.monitor': string
  'nav.control': string

  // Shell and header
  'shell.openNavigation': string
  'shell.collapse': string
  'shell.collapseSidebar': string
  'shell.expandSidebar': string
  'shell.closeSidebar': string
  'shell.theme': string
  'shell.palette': string
  'shell.colorPalette': string

  // Filters and time ranges
  'filter.title': string
  'filter.source': string
  'filter.project': string
  'filter.tag': string
  'filter.allSources': string
  'filter.allProjects': string
  'filter.allTags': string
  'filter.proxy': string
  'filter.imported': string
  'time.today': string
  'time.7d': string
  'time.30d': string
  'time.90d': string
  'time.180d': string
  'time.365d': string
  'time.all': string

  // Shared vocabulary
  'common.all': string
  'common.unknown': string
  'common.tryAgain': string
  'common.cancel': string
  'common.close': string
  'common.clear': string
  'common.filter': string
  'common.filterTable': string
  'common.noRecords': string
  'common.previousPage': string
  'common.nextPage': string
  'common.loading': string
  'common.select': string
  'common.saving': string
  'common.rescan': string
  'common.dryRun': string
  'common.status': string
  'common.time': string
  'common.reason': string
  'common.provider': string
  'common.model': string
  'common.tokens': string
  'common.cost': string
  'common.requests': string
  'common.total': string
  'common.new': string
  'common.duplicates': string
  'common.source': string
  'common.sources': string
  'common.project': string
  'common.live': string

  // Table chrome
  'table.rows.one': string
  'table.rows.other': string
  'table.filtered.one': string
  'table.filtered.other': string

  // App shell
  'doc.title': string
  'app.renderError': string
  'app.guardWarning': string
  'app.guardBreached': string
  'app.thresholdReached': string
  'app.statsFailed': string

  // Overview
  'overview.totalSpent': string
  'overview.totalTokens': string
  'overview.cacheRead': string
  'overview.throughput': string
  'overview.totalRequests': string
  'overview.savedVsCloud': string
  'overview.estimatedUsd': string
  'overview.promptCompletion': string
  'overview.cachedTokens': string
  'overview.peak': string
  'overview.avg': string
  'overview.shadow': string
  'overview.exportCsv': string
  'overview.snapshotSvg': string
  'overview.dailyConsumption': string
  'overview.dailyConsumptionSub': string
  'overview.costByModel': string
  'overview.totalSuffix': string
  'overview.noCostRecorded': string
  'overview.topModels': string
  'overview.byTokenVolume': string
  'overview.projects': string
  'overview.costPerProjectTag': string
  'overview.bySource': string
  'overview.proxyVsImported': string
  'overview.liveRuns': string
  'overview.liveRunsSub': string
  'overview.noInflight': string
  'overview.elapsed': string
  'overview.lastActivity': string
  'overview.recentRequests': string
  'overview.latest50': string
  'overview.refreshesEvery10s': string
  'overview.forecast': string
  'overview.anomaly.one': string
  'overview.anomalyDetail': string
  'overview.anomaly.other': string
  'overview.tokensSuffix': string
  'overview.reqsSuffix': string
  'overview.otherModels': string
  'overview.loadingDetails': string
  'overview.previousMonth': string
  'overview.nextMonth': string
  'overview.today': string
  'overview.less': string
  'overview.more': string
  'overview.shadowCost': string

  // Models
  'models.loadFailed': string
  'models.empty': string
  'models.changeRange': string
  'models.breakdown': string
  'models.seen': string
  'models.clickHeaders': string
  'models.unpriced': string
  'models.noCatalogPrice': string
  'models.headerModel': string
  'models.prompt': string
  'models.completion': string
  'models.cacheRead': string
  'models.costUsd': string

  // Clients
  'clients.loadFailed': string
  'clients.empty': string
  'clients.usageByClient': string
  'clients.agentsSeen': string
  'clients.clientApp': string

  // Events
  'events.loadFailed': string
  'events.empty': string
  'events.stream': string
  'events.streamSub': string
  'events.app': string
  'events.cacheRw': string
  'events.status': string
  'events.ttft': string
  'events.latency': string
  'events.detailTitle': string
  'events.timestamp': string
  'events.statusCode': string
  'events.promptTokens': string
  'events.completionTokens': string
  'events.cacheRead': string
  'events.cacheWrite': string
  'events.totalTokens': string
  'events.rawJson': string

  // Throughput
  'throughput.loadFailed': string
  'throughput.avgTps': string
  'throughput.peakTps': string
  'throughput.completionTokens': string
  'throughput.buckets': string
  'throughput.reqsAvg': string
  'throughput.maxBucket': string
  'throughput.interval': string
  'throughput.overTime': string
  'throughput.avgToks': string
  'throughput.peakToks': string
  'throughput.byModel': string
  'throughput.detailsByModel': string
  'throughput.byProject': string
  'throughput.tps': string
  'throughput.avgLatency': string
  'throughput.outTokens': string
  'throughput.project': string

  // Guard
  'guard.noLimits': string
  'guard.notConfigured': string
  'guard.limitsUsage': string
  'guard.nullLimit': string
  'guard.configuration': string
  'guard.configurationSub': string
  'guard.blocking': string
  'guard.enabled': string
  'guard.alertsOnly': string
  'guard.editsPersist': string
  'guard.history': string
  'guard.historySub': string
  'guard.noTransitions': string
  'guard.state': string
  'guard.dailyCost': string
  'guard.dailyCostLimit': string
  'guard.monthlyCostLimit': string
  'guard.dailyTokensLimit': string
  'guard.monthlyTokensLimit': string
  'guard.warningThreshold': string
  'guard.actionOnBreach': string
  'guard.alert': string
  'guard.block': string
  'guard.timezone': string
  'guard.saveLimits': string
  'guard.saveFailed': string
  'guard.saved': string
  'guard.thresholdReached': string
  'guard.emptyInfinity': string

  // Guard reason codes: the server sends a code plus numbers, never prose, so
  // the dashboard renders the reason in the reader's language.
  'guard.dailyCostBreached': string
  'guard.monthlyCostBreached': string
  'guard.dailyCostWarning': string
  'guard.monthlyCostWarning': string
  'guard.dailyTokensBreached': string
  'guard.monthlyTokensBreached': string
  'guard.dailyTokensWarning': string
  'guard.monthlyTokensWarning': string

  // Settings
  'settings.timezone': string
  'settings.timezoneSub': string
  'settings.timezoneLabel': string
  'settings.detectFromBrowser': string
  'settings.service': string
  'settings.serviceSub': string
  'settings.endpoint': string
  'settings.version': string
  'settings.refresh': string
  'settings.every10s': string
  'settings.configuration': string
  'settings.configurationSub': string
  'settings.configFile': string
  'settings.database': string
  'settings.prices': string
  'settings.manage': string
  'settings.manageSub': string
  'settings.routedProviders': string
  'settings.routedProvidersSub': string
  'settings.language': string
  'settings.languageSub': string

  // Import
  'import.sources': string
  'import.countingWindow': string
  'import.everySession': string
  'import.searchCli': string
  'import.searchPlaceholder': string
  'import.reachFailed': string
  'import.requestFailed': string
  'import.noSessionData': string
  'import.noSourceMatch': string
  'import.notDetected': string
  'import.noSessionsInRange': string
  'import.customPath': string
  'import.lastImport': string
  'import.savePath': string
  'import.useDefault': string
  'import.dataDirectory': string
  'import.title': string
  'import.titleSub': string
  'import.range': string
  'import.last7': string
  'import.last30': string
  'import.last90': string
  'import.everything': string
  'import.preview': string
  'import.importN.one': string
  'import.importN.other': string
  'import.selectAtLeastOne': string
  'import.step': string
  'import.scanned': string
  'import.previewTitle': string
  'import.previewSub': string
  'import.cancelled': string
  'import.imported': string
  'import.pastImports': string
  'import.newestFirst': string
  'import.noImports': string
  'import.started': string
  'import.allDetected': string
  'import.reportScanned': string
  'import.reportTokens': string
  'import.reportUnpriced': string
  'import.alreadyPresent': string
  'import.noPrice': string
  'import.startFailed': string
  'import.lostTrack': string
  'import.pathSaveFailed': string
  'import.savedMissing': string
  'import.nowReads': string

  // Calendar and day detail
  'calendar.title': string
  'calendar.subtitle': string
  'calendar.detailTitle': string
  'calendar.detailSub': string
  'calendar.close': string
  'calendar.totalCost': string
  'calendar.totalTokens': string
  'calendar.requests': string
  'calendar.avgLatency': string
  'calendar.byClient': string
  'calendar.bySource': string
  'calendar.noClients': string
  'calendar.noSources': string
  'calendar.loadError': string

  // API error codes
  'error.generic': string
  'error.guard.unavailable': string
  'error.guard.history': string
  'error.request.invalidJson': string
  'error.guard.fieldNumber': string
  'error.guard.fieldInteger': string
  'error.guard.action': string
  'error.guard.timezone': string
  'error.guard.unknownField': string
  'error.guard.range': string
  'error.guard.persist': string
  'error.import.alreadyRunning': string
  'error.import.unknownJob': string
  'error.import.unknownCli': string
  'error.import.pathAbsolute': string
  'error.import.persistPath': string
  'error.stats.unavailable': string
  'error.stats.partitions': string
  'error.stats.dayRequired': string
  'error.stats.detailUnavailable': string
  'error.stats.liveRuns': string
  'error.filters.unavailable': string
  'error.count.modelRequired': string
  'error.request.tooLarge': string
  'error.proxy.badGateway': string
  'error.proxy.rateLimited': string
  'error.auth.unauthorized': string
  'error.ui.unknown': string
}

export type LocaleCode =
  | 'en'
  | 'pt-BR'
  | 'es'
  | 'zh-CN'
  | 'zh-TW'
  | 'ja'
  | 'ko'
  | 'de'
  | 'fr'
  | 'it'
  | 'ru'
  | 'uk'
  | 'pl'
  | 'tr'
  | 'hi'
  | 'id'
  | 'vi'
  | 'th'
  | 'ar'
  | 'fa'

export type LocaleMessages = Partial<Dict>
export type MessageParams = Record<string, string | number>
export type DictKey = keyof Dict