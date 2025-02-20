package net.consensys.zkevm.ethereum.coordination.messageanchoring

import build.linea.contract.LineaRollupV6
import io.vertx.core.Vertx
import linea.kotlin.toULong
import net.consensys.linea.async.toSafeFuture
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.apache.tuweni.bytes.Bytes32
import org.web3j.abi.EventEncoder
import org.web3j.protocol.Web3j
import org.web3j.protocol.core.DefaultBlockParameter
import org.web3j.protocol.core.methods.request.EthFilter
import org.web3j.protocol.core.methods.response.EthLog
import org.web3j.protocol.core.methods.response.Log
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.math.BigInteger
import java.util.concurrent.Callable
import kotlin.time.Duration

class L1EventQuerierImpl(
  private val vertx: Vertx,
  private val config: Config,
  private val l1Web3jClient: Web3j
) : L1EventQuerier {
  companion object {
    val encodedMessageSentEvent: String = EventEncoder.encode(LineaRollupV6.MESSAGESENT_EVENT)

    fun parseMessageSentEventLogs(log: Log): SendMessageEvent {
      val messageSentEvent = LineaRollupV6.getMessageSentEventFromLog(log)
      return SendMessageEvent(
        Bytes32.wrap(messageSentEvent._messageHash),
        messageSentEvent._nonce.toULong(),
        messageSentEvent.log.blockNumber.toULong()
      )
    }
  }

  data class QueryStateParameters(
    var startingBlock: BigInteger,
    var finalBlock: BigInteger,
    var startingEventLogIndex: BigInteger
  )

  private val startingLogIndexToIncludeAllLogs = BigInteger.valueOf(-1)
  private val log: Logger = LogManager.getLogger(this::class.java)

  class Config(
    val pollingInterval: Duration,
    val maxEventScrapingTime: Duration,
    val earliestL1Block: BigInteger,
    val maxMessagesToCollect: UInt,
    val l1MessageServiceAddress: String,
    val finalized: String,
    val blockRangeLoopLimit: UInt
  )

  override fun getSendMessageEventsForAnchoredMessage(
    messageHash: MessageHashAnchoredEvent?
  ): SafeFuture<List<SendMessageEvent>> {
    return vertx.executeBlocking(
      Callable {
        val finalBlock = getFinalBlock()
        val initialQueryStateParameters =
          if (messageHash != null) {
            // get startingBlock and index to start ignoring from for first round of data
            log.debug("Starting with message hash: {}", messageHash.messageHash)
            getBlockAtEventEmission(finalBlock, messageHash)
          } else {
            log.debug("Starting hash is null, using earliest block and latest block")
            QueryStateParameters(config.earliestL1Block, finalBlock, startingLogIndexToIncludeAllLogs)
          }

        val collectedEvents = collectEvents(initialQueryStateParameters)
        log.debug(
          "Completing with events: ${collectedEvents.count()} with maxMessagesToAnchor:${config.maxMessagesToCollect}"
        )
        collectedEvents.take(config.maxMessagesToCollect.toInt())
      },
      true
    )
      .toSafeFuture()
  }

  private fun collectEvents(queryStateParameters: QueryStateParameters): List<SendMessageEvent> {
    val collectedEvents: MutableList<SendMessageEvent> = mutableListOf()
    var eventCollectionQueryParameters = queryStateParameters
    val startTimestampMillis = System.currentTimeMillis()
    var elapsedTimeMillis: Long
    do {
      val finalBlock = getFinalBlock()

      // NB! make sure we only use the latest finalized block or within limits
      eventCollectionQueryParameters.finalBlock = getFinalBlockForQueryingWithinLimits(
        eventCollectionQueryParameters.startingBlock,
        finalBlock
      )

      log.trace("Querying for events with {}", eventCollectionQueryParameters)
      // get the mapped events and next block number to start from
      val (newEvents, nextQueryParameters) = getEventsFromLogIndexInRange(eventCollectionQueryParameters)
      eventCollectionQueryParameters = nextQueryParameters
      collectedEvents.addAll(newEvents)

      // we may have enough messages, so we could end
      if (collectedEvents.count().toUInt() >= config.maxMessagesToCollect) {
        break
      }

      Thread.sleep(config.pollingInterval.inWholeMilliseconds)
      elapsedTimeMillis = System.currentTimeMillis() - startTimestampMillis
    } while (elapsedTimeMillis < config.maxEventScrapingTime.inWholeMilliseconds)
    return collectedEvents
  }

  private fun getFinalBlock(): BigInteger = l1Web3jClient
    .ethGetBlockByNumber(DefaultBlockParameter.valueOf(config.finalized), false)
    .send()
    .block
    .number

  private fun getEventsFromLogIndexInRange(
    queryStateParameters: QueryStateParameters
  ): Pair<List<SendMessageEvent>, QueryStateParameters> {
    val getLogsResponse = l1Web3jClient.ethGetLogs(
      buildEventFilter(
        queryStateParameters.startingBlock,
        queryStateParameters.finalBlock
      )
    ).send()
    val tempLogs: MutableList<EthLog.LogResult<Any>>? = getLogsResponse.logs

    log.trace("Getting events with {}", queryStateParameters)

    val newLogs = tempLogs?.filter { logResult ->
      isEventAfterEventOnInitialBlock(
        logResult.get(),
        queryStateParameters.startingEventLogIndex,
        queryStateParameters.startingBlock
      )
    }

    return when {
      tempLogs == null -> {
        log.debug("Logs request failed! Error: {}", getLogsResponse.error)
        Pair(emptyList(), queryStateParameters)
      }

      !newLogs.isNullOrEmpty() -> {
        val lastLog = (newLogs.last().get() as Log)

        val newStartingBlock = BigInteger.valueOf(lastLog.blockNumber.toLong())
        val newStartingIndex = BigInteger.valueOf(lastLog.logIndex.toLong())

        val nextQueryStateParameters = QueryStateParameters(
          newStartingBlock,
          queryStateParameters.finalBlock,
          newStartingIndex
        )

        val events = newLogs.map { mappingLog -> parseMessageSentEventLogs(mappingLog.get() as Log) }

        Pair(events, nextQueryStateParameters)
      }

      startAndFinalBlockAreSame(queryStateParameters) -> {
        Pair(
          listOf(),
          QueryStateParameters(
            queryStateParameters.finalBlock,
            queryStateParameters.finalBlock,
            queryStateParameters.startingEventLogIndex
          )
        )
      }

      else -> {
        Pair(
          listOf(),
          QueryStateParameters(
            queryStateParameters.finalBlock,
            queryStateParameters.finalBlock,
            startingLogIndexToIncludeAllLogs
          )
        )
      }
    }
  }

  private fun startAndFinalBlockAreSame(queryStateParameters: QueryStateParameters): Boolean {
    return queryStateParameters.startingBlock == queryStateParameters.finalBlock
  }

  private fun isEventAfterEventOnInitialBlock(
    logResult: Any,
    logIndex: BigInteger,
    startingBlock: BigInteger
  ): Boolean {
    val eventLog = (logResult as Log)
    return (eventLog.blockNumber == startingBlock && eventLog.logIndex > logIndex) ||
      (eventLog.blockNumber > startingBlock)
  }

  private fun buildEventFilter(startingBlock: BigInteger, finalBlock: BigInteger): EthFilter {
    val sentMessagesFilter =
      EthFilter(
        DefaultBlockParameter.valueOf(startingBlock),
        DefaultBlockParameter.valueOf(finalBlock),
        config.l1MessageServiceAddress
      )

    sentMessagesFilter.addSingleTopic(encodedMessageSentEvent)
    return sentMessagesFilter
  }

  private fun getBlockAtEventEmission(
    finalBlock: BigInteger,
    messageHash: MessageHashAnchoredEvent
  ): QueryStateParameters {
    val messageHashFilter =
      EthFilter(
        DefaultBlockParameter.valueOf(config.earliestL1Block),
        DefaultBlockParameter.valueOf(finalBlock),
        config.l1MessageServiceAddress
      )

    messageHashFilter.addSingleTopic(encodedMessageSentEvent)
    messageHashFilter.addNullTopic()
    messageHashFilter.addNullTopic()
    messageHashFilter.addSingleTopic(messageHash.messageHash.toString())

    log.trace("Trying to find the event in range [{} .. {}]", config.earliestL1Block, finalBlock)
    // get the block where the hash was found
    val logs = l1Web3jClient.ethGetLogs(messageHashFilter).send().logs

    return if (!logs.isNullOrEmpty()) {
      val eventLog = logs.first().get() as Log
      val finalBlockWithinLimits = getFinalBlockForQueryingWithinLimits(eventLog.blockNumber, finalBlock)
      log.trace("Found event hash at block {}", eventLog.blockNumber)
      QueryStateParameters(eventLog.blockNumber, finalBlockWithinLimits, eventLog.logIndex)
    } else {
      val finalBlockWithinLimits = getFinalBlockForQueryingWithinLimits(config.earliestL1Block, finalBlock)
      QueryStateParameters(config.earliestL1Block, finalBlockWithinLimits, startingLogIndexToIncludeAllLogs)
    }
  }

  private fun getFinalBlockForQueryingWithinLimits(
    startingBlock: BigInteger,
    finalBlock: BigInteger
  ): BigInteger {
    val loopLimit = BigInteger.valueOf(config.blockRangeLoopLimit.toLong())

    return minOf(startingBlock + loopLimit, finalBlock)
  }
}
