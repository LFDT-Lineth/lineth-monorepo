package net.consensys.zkevm.ethereum.finalization

import kotlinx.datetime.Clock
import net.consensys.zkevm.coordinator.clients.smartcontract.LineaRollupSmartContractClient
import net.consensys.zkevm.domain.BlobRecord
import net.consensys.zkevm.domain.FinalizationSubmittedEvent
import net.consensys.zkevm.domain.ProofToFinalize
import net.consensys.zkevm.ethereum.gaspricing.GasPriceCapProvider
import net.consensys.zkevm.ethereum.submission.logSubmissionError
import org.apache.logging.log4j.LogManager
import org.web3j.tx.exceptions.ContractCallException
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.function.Consumer

interface AggregationSubmitter {
  /**
   * Validates if the aggregation proof is valid and submits it to the Linea contract.
   * If eth_call is successful, the aggregation is submitted to the Linea contract
   * and the transaction hash is returned.
   */
  fun submitAggregationAfterEthCall(
    aggregationProof: ProofToFinalize,
    aggregationEndBlob: BlobRecord,
    parentShnarf: ByteArray,
    parentL1RollingHash: ByteArray,
    parentL1RollingHashMessageNumber: Long,
  ): SafeFuture<String?>
}

class AggregationSubmitterImpl(
  private val lineaRollup: LineaRollupSmartContractClient,
  private val gasPriceCapProvider: GasPriceCapProvider?,
  private val aggregationSubmittedEventConsumer: Consumer<FinalizationSubmittedEvent> =
    Consumer<FinalizationSubmittedEvent> { },
  private val clock: Clock = Clock.System,
) : AggregationSubmitter {
  private val log = LogManager.getLogger(this::class.java)

  override fun submitAggregationAfterEthCall(
    aggregationProof: ProofToFinalize,
    aggregationEndBlob: BlobRecord,
    parentShnarf: ByteArray,
    parentL1RollingHash: ByteArray,
    parentL1RollingHashMessageNumber: Long,
  ): SafeFuture<String?> {
    log.debug("submitting aggregation={}", aggregationProof.intervalString())
    return (
      gasPriceCapProvider?.getGasPriceCaps(aggregationProof.firstBlockNumber)
        ?: SafeFuture.completedFuture(null)
      ).thenCompose { gasPriceCaps ->
      val nonce = lineaRollup.currentNonce()
      lineaRollup.finalizeBlocksAfterEthCall(
        aggregation = aggregationProof,
        aggregationLastBlob = aggregationEndBlob,
        parentL1RollingHash = parentL1RollingHash,
        parentL1RollingHashMessageNumber = parentL1RollingHashMessageNumber,
        gasPriceCaps = gasPriceCaps,
      ).thenPeek { transactionHash ->
        log.info(
          "submitted aggregation={} transactionHash={} nonce={} gasPriceCaps={}",
          aggregationProof.intervalString(),
          transactionHash,
          nonce,
          gasPriceCaps,
        )
        val aggregationSubmittedEvent = FinalizationSubmittedEvent(
          aggregationProof = aggregationProof,
          parentShnarf = parentShnarf,
          parentL1RollingHash = parentL1RollingHash,
          parentL1RollingHashMessageNumber = parentL1RollingHashMessageNumber,
          submissionTimestamp = clock.now(),
          transactionHash = transactionHash.toByteArray(),
        )
        aggregationSubmittedEventConsumer.accept(aggregationSubmittedEvent)
      }
    }.whenException { th ->
      logAggregationSubmissionError(aggregationProof.intervalString(), th)
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
