package linea.build.staterecover.clients.smartcontract

import build.linea.contract.LineaRollupV6
import build.linea.domain.EthLogEvent
import build.linea.staterecover.clients.DataFinalizedV3
import build.linea.staterecover.clients.DataSubmittedV3
import build.linea.staterecover.clients.FinalizationAndDataEventsV3
import build.linea.staterecover.clients.LineaRollupSubmissionEventsClient
import build.linea.web3j.domain.toDomain
import build.linea.web3j.domain.toWeb3j
import net.consensys.encodeHex
import net.consensys.linea.BlockParameter
import net.consensys.linea.BlockParameter.Companion.toBlockParameter
import net.consensys.linea.contract.Web3JLogsClient
import net.consensys.toHexStringUInt256
import org.web3j.abi.EventEncoder
import org.web3j.protocol.core.methods.request.EthFilter
import org.web3j.protocol.core.methods.response.Log
import tech.pegasys.teku.infrastructure.async.SafeFuture

class LineaSubmissionEventsClientWeb3jIpml(
  private val logsClient: Web3JLogsClient,
  private val smartContractAddress: String,
  private val mostRecentBlockTag: BlockParameter = BlockParameter.Tag.FINALIZED
) : LineaRollupSubmissionEventsClient {

  override fun findDataFinalizedEventContainingBlock(l2BlockNumber: ULong): SafeFuture<EthLogEvent<DataFinalizedV3>?> {
    TODO("Not yet implemented")
  }

  override fun findDataFinalizedEventByStartBlockNumber(
    l2BlockNumber: ULong
  ): SafeFuture<EthLogEvent<DataFinalizedV3>?> {
    // TODO: be less eager on block range to search
    return findDataFinalizedV3Event(
      fromL1BlockNumber = BlockParameter.Tag.EARLIEST,
      toL1BlockNumber = mostRecentBlockTag,
      startBlockNumber = l2BlockNumber
    )
  }

  override fun findDataSubmittedV3EventsUntilNextFinalization(
    l2StartBlockNumberInclusive: ULong
  ): SafeFuture<FinalizationAndDataEventsV3?> {
    return findDataFinalizedV3Event(
      fromL1BlockNumber = BlockParameter.Tag.EARLIEST,
      toL1BlockNumber = mostRecentBlockTag,
      startBlockNumber = l2StartBlockNumberInclusive
    )
      .thenCompose { finalizationEvent ->
        finalizationEvent
          ?.let {
            findAggregationDataSubmittedV3Events(it)
              .thenApply { dataSubmittedEvents ->
                FinalizationAndDataEventsV3(dataSubmittedEvents, it)
              }
          }
          ?: SafeFuture.completedFuture(null)
      }
  }

  private fun findDataFinalizedV3Event(
    fromL1BlockNumber: BlockParameter,
    toL1BlockNumber: BlockParameter,
    startBlockNumber: ULong? = null,
    endBlockNumber: ULong? = null
  ): SafeFuture<EthLogEvent<DataFinalizedV3>?> {
    assert(startBlockNumber != null || endBlockNumber != null) {
      "Either startBlockNumber or endBlockNumber must be provided"
    }

    val ethFilter =
      EthFilter(
        fromL1BlockNumber.toWeb3j(),
        toL1BlockNumber.toWeb3j(),
        smartContractAddress
      ).apply {
        /**
         event DataFinalizedV3(
         uint256 indexed startBlockNumber,
         uint256 indexed endBlockNumber,
         bytes32 indexed shnarf,
         bytes32 parentStateRootHash,
         bytes32 finalStateRootHash
         );
         */
        addSingleTopic(EventEncoder.encode(LineaRollupV6.DATAFINALIZEDV3_EVENT))
        addSingleTopic(startBlockNumber?.toHexStringUInt256())
        addSingleTopic(endBlockNumber?.toHexStringUInt256())
      }

    return logsClient
      .getLogs(ethFilter)
      .thenApply(Companion::parseDataFinalizedV3)
      .thenCompose { finalizedEvents ->
        if (finalizedEvents.size > 1) {
          // just a safety check
          // this should never happen: Finalization events shall be sequential and deterministic
          val errorMessage =
            "More than one DataFinalizedV3 event found for startBlockNumber=$startBlockNumber events=$finalizedEvents"
          SafeFuture.failedFuture(IllegalStateException(errorMessage))
        } else {
          SafeFuture.completedFuture(finalizedEvents.firstOrNull())
        }
      }
  }

  private fun findAggregationDataSubmittedV3Events(
    finalizationEvent: EthLogEvent<DataFinalizedV3>
  ): SafeFuture<List<EthLogEvent<DataSubmittedV3>>> {
    val dataEvents = mutableListOf<EthLogEvent<DataSubmittedV3>>()
    val futureResult = SafeFuture<List<EthLogEvent<DataSubmittedV3>>>()
    fun fetchParentDataSubmission(dataSubmission: EthLogEvent<DataSubmittedV3>?) {
      if (
        dataSubmission == null ||
        dataSubmission.event.finalStateRootHash.contentEquals(finalizationEvent.event.parentStateRootHash)
      ) {
        // if dataSubmission == null
        // means there is no parent event so we are done.
        futureResult.complete(dataEvents)
      } else {
        // adding to the head of the list so client gets the events in the order of submission
        dataEvents.addFirst(dataSubmission)
        findDataSubmittedV3EventByShnarf(
          fromL1BlockParameter = BlockParameter.Tag.EARLIEST,
          tol1BlockParameter = dataSubmission.log.blockNumber.toLong().toBlockParameter(),
          shnarf = dataSubmission.event.parentShnarf
        ).thenPeek(::fetchParentDataSubmission)
      }
    }

    getDataSubmittedV3EventByShnarf(
      fromL1BlockParameter = BlockParameter.Tag.EARLIEST,
      tol1BlockParameter = finalizationEvent.log.blockNumber.toLong().toBlockParameter(),
      shnarf = finalizationEvent.event.shnarf
    ).thenPeek(::fetchParentDataSubmission)

    return futureResult
  }

  private fun getDataSubmittedV3EventByShnarf(
    fromL1BlockParameter: BlockParameter,
    tol1BlockParameter: BlockParameter,
    shnarf: ByteArray
  ): SafeFuture<EthLogEvent<DataSubmittedV3>> {
    return findDataSubmittedV3EventByShnarf(fromL1BlockParameter, tol1BlockParameter, shnarf)
      .thenApply { event ->
        event ?: throw IllegalStateException("DataSubmittedV3 event not found for shnarf=${shnarf.encodeHex()}")
      }
  }

  private fun findDataSubmittedV3EventByShnarf(
    fromL1BlockParameter: BlockParameter,
    tol1BlockParameter: BlockParameter,
    shnarf: ByteArray
  ): SafeFuture<EthLogEvent<DataSubmittedV3>?> {
    val ethFilter =
      EthFilter(
        fromL1BlockParameter.toWeb3j(),
        tol1BlockParameter.toWeb3j(),
        smartContractAddress
      ).apply {
        // event DataSubmittedV3(bytes32 parentShnarf, bytes32 indexed shnarf, bytes32 finalStateRootHash);
        addSingleTopic(EventEncoder.encode(LineaRollupV6.DATASUBMITTEDV3_EVENT))
        addSingleTopic(shnarf.encodeHex()) // shnarf
      }

    return logsClient
      .getLogs(ethFilter)
      .thenApply(Companion::parseDataSubmittedV3)
      .thenApply { events ->
        if (events.size > 1) {
          // just a safety check
          // this should never happen: having more than blob with the same shnarf
          val errorMessage =
            "More than one DataSubmittedV3 event found with shnarf=${shnarf.encodeHex()} events=$events"
          throw IllegalStateException(errorMessage)
        } else {
          events.firstOrNull()
        }
      }
  }

  companion object {
    private fun parseDataSubmittedV3(logs: List<Log>): List<EthLogEvent<DataSubmittedV3>> {
      return logs.map { log -> DataSubmittedV3.fromEthLog(log.toDomain()) }
    }

    private fun parseDataFinalizedV3(logs: List<Log>): List<EthLogEvent<DataFinalizedV3>> {
      return logs.map { log -> DataFinalizedV3.fromEthLog(log.toDomain()) }
    }
  }
}
