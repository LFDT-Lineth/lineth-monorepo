package linea.ethapi.extensions

import linea.LongRunningService
import linea.domain.EthLog
import linea.ethapi.EthLogsFilterOptions

fun interface EthLogConsumer {
  fun accept(ethLog: EthLog)
}

interface EthLogsFilterSubscriptionManager : LongRunningService {
  fun setConsumer(logsConsumer: EthLogConsumer)
}

interface EthLogsFilterSubscriptionFactory {
  fun create(
    filterOptions: EthLogsFilterOptions,
    logsConsumer: EthLogConsumer? = null,
  ): EthLogsFilterSubscriptionManager
}
