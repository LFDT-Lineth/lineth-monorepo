package linea.staterecovery

import lineth.contract.events.DataFinalizedV3
import lineth.contract.events.DataSubmittedV3
import lineth.domain.BlockParameter
import lineth.domain.EthLogEvent
import tech.pegasys.teku.infrastructure.async.SafeFuture

data class FinalizationAndDataEventsV3(
  val dataSubmittedEvents: List<EthLogEvent<DataSubmittedV3>>,
  val dataFinalizedEvent: EthLogEvent<DataFinalizedV3>,
)

interface LinethRollupSubmissionEventsClient {
  fun findFinalizationAndDataSubmissionV3Events(
    fromL1BlockNumber: BlockParameter,
    finalizationStartBlockNumber: ULong,
  ): SafeFuture<FinalizationAndDataEventsV3?>

  fun findFinalizationAndDataSubmissionV3EventsContainingL2BlockNumber(
    fromL1BlockNumber: BlockParameter,
    l2BlockNumber: ULong,
  ): SafeFuture<FinalizationAndDataEventsV3?>
}
