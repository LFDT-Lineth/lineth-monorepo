package net.consensys.linea.contract

import linea.kotlin.toULong
import linea.web3j.Web3JLogsClient
import net.consensys.zkevm.coordinator.clients.L2MessageServiceLogsClient
import net.consensys.zkevm.domain.BridgeLogsData
import net.consensys.zkevm.domain.L2RollingHashUpdatedEvent
import org.web3j.abi.EventEncoder
import org.web3j.abi.datatypes.Event
import org.web3j.protocol.core.DefaultBlockParameter
import org.web3j.protocol.core.methods.request.EthFilter
import org.web3j.protocol.core.methods.response.Log
import tech.pegasys.teku.infrastructure.async.SafeFuture

fun parseBridgeLogsData(logObject: Log): BridgeLogsData {
  return BridgeLogsData(
    removed = logObject.isRemoved,
    logIndex = logObject.logIndexRaw,
    transactionIndex = logObject.transactionIndexRaw,
    transactionHash = logObject.transactionHash,
    blockHash = logObject.blockHash,
    blockNumber = logObject.blockNumberRaw,
    address = logObject.address,
    data = logObject.data,
    topics = logObject.topics
  )
}

class Web3JL2MessageServiceLogsClient(
  val logsClient: Web3JLogsClient,
  val l2MessageServiceAddress: String,
  private val messageEvents: List<Event> = listOf(
    L2MessageService.L1L2MESSAGEHASHESADDEDTOINBOX_EVENT,
    L2MessageService.MESSAGESENT_EVENT,
    L2MessageService.ROLLINGHASHUPDATED_EVENT
  )
) : L2MessageServiceLogsClient {
  override fun getBridgeLogs(
    blockNumber: Long
  ): SafeFuture<List<BridgeLogsData>> {
    // Load all the requests to the worker pool to run in parallel
    val ethFilter =
      EthFilter(
        DefaultBlockParameter.valueOf(blockNumber.toBigInteger()),
        DefaultBlockParameter.valueOf(blockNumber.toBigInteger()),
        l2MessageServiceAddress
      )
    ethFilter.addOptionalTopics(
      *messageEvents.map { EventEncoder.encode(it) }.toTypedArray()
    )

    return logsClient.getLogs(ethFilter)
      .thenApply { logs -> logs.map { parseBridgeLogsData(it) } }
  }

  override fun findLastRollingHashUpdatedEvent(
    upToBlockNumberInclusive: Long,
    lookBackBlockNumberLimitInclusive: Long
  ): SafeFuture<L2RollingHashUpdatedEvent?> {
    return logsClient.findLastLog(
      upToBlockNumberInclusive,
      l2MessageServiceAddress,
      lookBackBlockNumberLimitInclusive,
      listOf(L2MessageService.ROLLINGHASHUPDATED_EVENT)
    ).thenApply { log ->
      log?.let {
        val event = L2MessageService.getRollingHashUpdatedEventFromLog(log)
        L2RollingHashUpdatedEvent(
          messageNumber = event.messageNumber.toULong(),
          messageRollingHash = event.rollingHash
        )
      }
    }
  }
}
