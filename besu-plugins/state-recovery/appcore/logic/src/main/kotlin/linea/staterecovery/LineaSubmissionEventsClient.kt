package linea.staterecovery

import linea.contract.events.DataFinalizedV3
import linea.contract.events.DataSubmittedV3
import linea.domain.BlockParameter
import linea.domain.EthLogEvent
import tech.pegasys.teku.infrastructure.async.SafeFuture

data class FinalizationAndDataEventsV3(
  val dataSubmittedEvents: List<EthLogEvent<DataSubmittedV3>>,
  val dataFinalizedEvent: EthLogEvent<DataFinalizedV3>,
)

interface LineaRollupSubmissionEventsClient {
  fun findFinalizationAndDataSubmissionV3Events(
    fromL1BlockNumber: BlockParameter,
    finalizationStartBlockNumber: ULong,
  ): SafeFuture<FinalizationAndDataEventsV3?>

  fun findFinalizationAndDataSubmissionV3EventsContainingL2BlockNumber(
    fromL1BlockNumber: BlockParameter,
    l2BlockNumber: ULong,
  ): SafeFuture<FinalizationAndDataEventsV3?>
}
