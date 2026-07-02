package linea.anchoring.clients

import linea.EthLogsSearcher
import linea.SearchDirection
import linea.contract.events.L1RollingHashUpdatedEvent
import linea.contract.events.MessageSentEvent
import linea.domain.BlockParameter
import linea.domain.CommonDomainFunctions
import linea.domain.EthLogEvent
import linea.domain.toBlockParameter
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.atomic.AtomicReference
import kotlin.time.Duration

/**
 * This class is responsible for fetching L1 MessageSent events from the Ethereum blockchain.
 * It leverages the invariant of increasing message numbers to efficiently search for events on L1 by never going back
 *
 * This is relevant when MessageSent events are very sparse on the chain, it requires 10_000K l1 blocks
 * to collect 100 messages.
 */
internal class L1MessageSentEventsFetcher(
  private val l1SmartContractAddress: String,
  private val l1EventsSearcher: EthLogsSearcher,
  private val l1HighestBlock: BlockParameter,
  private val l1EventSearchMaxBlockRange: UInt,
  private val log: Logger = LogManager.getLogger(L1MessageSentEventsFetcher::class.java),
) {
  private data class LastSearch(
    val highestL1AlreadySearchedBlockNumber: ULong,
    val lastStartingMessageNumber: ULong,
  )

  private val lastSearch = AtomicReference(
    LastSearch(
      highestL1AlreadySearchedBlockNumber = 0UL,
      lastStartingMessageNumber = 0UL,
    ),
  )

  fun findL1MessageSentEvents(
    startingMessageNumber: ULong,
    targetMessagesToFetch: UInt,
    fetchTimeout: Duration,
    blockChunkSize: UInt,
  ): SafeFuture<List<EthLogEvent<MessageSentEvent>>> {
    require(startingMessageNumber >= lastSearch.get().lastStartingMessageNumber) {
      "startingMessageNumber=$startingMessageNumber must greater than " +
        "or equal to lastStartingMessageNumber=${lastSearch.get().lastStartingMessageNumber}"
    }

    return findL1RollingHashUpdatedEvent(
      fromBlock = lastSearch.get().highestL1AlreadySearchedBlockNumber,
      messageNumber = startingMessageNumber,
    ).thenCompose { event ->
      if (event == null) {
        return@thenCompose SafeFuture.completedFuture(emptyList())
      }

      l1EventsSearcher.getLogsRollingForward(
        fromBlock = event.log.blockNumber.toBlockParameter(),
        toBlock = l1HighestBlock,
        address = l1SmartContractAddress,
        topics = listOf(
          MessageSentEvent.topic,
        ),
        chunkSize = blockChunkSize,
        searchTimeout = fetchTimeout,
        stopAfterTargetLogsCount = targetMessagesToFetch,
      ).thenApply { result ->
        lastSearch.set(LastSearch(result.endBlockNumber, startingMessageNumber))
        val events = result.logs.map(MessageSentEvent::fromEthLog)
        log.debug(
          "fetched MessageSent events from L1: messageNumbers={} l1Blocks={}",
          CommonDomainFunctions.blockIntervalString(
            events.first().event.messageNumber,
            events.last().event.messageNumber,
          ),
          result.intervalString(),
        )
        events
      }
    }
  }

  private fun findL1RollingHashUpdatedEvent(
    fromBlock: ULong,
    messageNumber: ULong,
  ): SafeFuture<EthLogEvent<L1RollingHashUpdatedEvent>?> {
    return l1EventsSearcher.findLog(
      fromBlock = fromBlock.toBlockParameter(),
      toBlock = l1HighestBlock,
      chunkSize = l1EventSearchMaxBlockRange.toInt(),
      address = l1SmartContractAddress,
      topics = listOf(L1RollingHashUpdatedEvent.topic),
    ) { ethLog ->
      val foundMessageNumber = L1RollingHashUpdatedEvent.fromEthLog(ethLog).event.messageNumber
      when {
        foundMessageNumber < messageNumber -> SearchDirection.FORWARD
        foundMessageNumber > messageNumber -> SearchDirection.BACKWARD
        else -> null
      }
    }.thenApply { ethLog ->
      ethLog?.let { L1RollingHashUpdatedEvent.fromEthLog(it) }
    }
  }
}
