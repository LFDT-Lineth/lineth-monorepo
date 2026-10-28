package lineth.finalization

import linea.contract.l1.FinalizationDataV9
import linea.contract.l1.LinethFinalizationSmartContractClient
import linea.domain.CommonDomainFunctions
import linea.domain.FinalizationSubmittedEvent
import lineth.gaspricing.GasPriceCapProviderV2
import lineth.submission.logSubmissionError
import org.apache.logging.log4j.LogManager
import org.web3j.tx.exceptions.ContractCallException
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.function.Consumer
import kotlin.time.Clock
import kotlin.time.Instant

interface AggregationSubmitterV2 {
  /**
   * Validates if the aggregation proof is valid and submits it to the Lineth contract.
   * If eth_call is successful, the aggregation is submitted to the Lineth contract
   * and the transaction hash is returned.
   */
  fun submitAggregationAfterEthCall(
    startBlockNumber: ULong,
    endBlockNumber: ULong,
    startBlockTimestamp: Instant,
    finalizationData: FinalizationDataV9,
  ): SafeFuture<String?>
}

class AggregationSubmitterV2Impl(
  private val lineaSmartContractClient: LinethFinalizationSmartContractClient,
  private val gasPriceCapProvider: GasPriceCapProviderV2?,
  private val aggregationSubmittedEventConsumer: Consumer<FinalizationSubmittedEvent> =
    Consumer<FinalizationSubmittedEvent> { },
  private val clock: Clock = Clock.System,
) : AggregationSubmitterV2 {
  private val log = LogManager.getLogger(this::class.java)

  override fun submitAggregationAfterEthCall(
    startBlockNumber: ULong,
    endBlockNumber: ULong,
    startBlockTimestamp: Instant,
    finalizationData: FinalizationDataV9,
  ): SafeFuture<String?> {
    val blockInterval = CommonDomainFunctions.blockIntervalString(startBlockNumber, endBlockNumber)
    log.debug("submitting aggregation={}", blockInterval)
    return (
      gasPriceCapProvider?.getGasPriceCaps(startBlockTimestamp)
        ?: SafeFuture.completedFuture(null)
      ).thenCompose { gasPriceCaps ->
      val nonce = lineaSmartContractClient.currentNonce()
      lineaSmartContractClient.finalizeBlocksV9(
        data = finalizationData,
        gasPriceCaps = gasPriceCaps,
        preflightWithEthCall = true,
      ).thenPeek { transactionHash ->
        log.info(
          "submitted aggregation={} transactionHash={} nonce={} gasPriceCaps={}",
          blockInterval,
          transactionHash,
          nonce,
          gasPriceCaps,
        )
        val aggregationSubmittedEvent = FinalizationSubmittedEvent(
          endBlockNumber = finalizationData.endBlockNumber,
          endBlockTimestamp = finalizationData.finalTimestamp,
          submissionTimestamp = clock.now(),
        )
        aggregationSubmittedEventConsumer.accept(aggregationSubmittedEvent)
      }
    }.whenException { th ->
      logAggregationSubmissionError(blockInterval, th)
    }
  }

  private fun logAggregationSubmissionError(
    intervalString: String,
    error: Throwable,
    isEthCall: Boolean = error.cause is ContractCallException,
  ) {
    logSubmissionError(
      log,
      "{} for aggregation finalization failed: aggregation={} errorMessage={}",
      intervalString,
      error,
      isEthCall,
    )
  }
}
