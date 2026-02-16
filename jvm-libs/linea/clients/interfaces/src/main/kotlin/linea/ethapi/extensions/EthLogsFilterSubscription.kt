package linea.ethapi.extensions

import linea.LongRunningService
import linea.domain.EthLog
import linea.ethapi.EthLogsFilterOptions

fun interface EthLogConsumer {
  fun accept(ethLog: EthLog)
}

sealed class EthLogsFilterState {
  /**
   * Initial state before any polling has started
   */
  data object Idle : EthLogsFilterState()

  /**
   * Actively searching through historical blocks that exist on chain
   * @param startBlockNumber The block number where the current search iteration started
   */
  data class Searching(val startBlockNumber: ULong) : EthLogsFilterState()

  /**
   * Caught up to the target block (e.g., FINALIZED/SAFE/LATEST tag)
   * Waiting for new blocks to be mined
   * @param lastSearchedBlockNumber The last block number that was fully searched
   */
  data class CaughtUp(val lastSearchedBlockNumber: ULong) : EthLogsFilterState()
}

fun interface EthLogsFilterStateListener {
  fun onStateChange(oldState: EthLogsFilterState, newState: EthLogsFilterState)
}

interface EthLogsFilterSubscriptionManager : LongRunningService {
  fun setConsumer(logsConsumer: EthLogConsumer)

  /**
   * Sets a listener to be notified when the filter state changes.
   * Useful for knowing when the search has caught up to the chain head
   * and when it resumes searching after new blocks are mined.
   */
  fun setStateListener(stateListener: EthLogsFilterStateListener?)
}

interface EthLogsFilterSubscriptionFactory {
  fun create(
    filterOptions: EthLogsFilterOptions,
    logsConsumer: EthLogConsumer? = null,
  ): EthLogsFilterSubscriptionManager
}
