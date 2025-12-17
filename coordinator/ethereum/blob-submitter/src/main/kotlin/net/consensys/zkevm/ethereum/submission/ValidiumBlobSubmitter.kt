package net.consensys.zkevm.ethereum.submission

import kotlinx.datetime.Clock
import linea.domain.BlockIntervalData
import linea.domain.toBlockIntervalsString
import net.consensys.zkevm.coordinator.clients.smartcontract.LineaValidiumSmartContractClient
import net.consensys.zkevm.domain.BlobRecord
import net.consensys.zkevm.domain.BlobSubmittedEvent
import net.consensys.zkevm.ethereum.gaspricing.GasPriceCapProvider
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.function.Consumer

class ValidiumBlobSubmitter(
  private val contract: LineaValidiumSmartContractClient,
  private val gasPriceCapProvider: GasPriceCapProvider?,
  private val blobSubmittedEventConsumer: Consumer<BlobSubmittedEvent> = Consumer<BlobSubmittedEvent> { },
  private val clock: Clock = Clock.System,
) : BlobSubmitter {
  private val log: Logger = LogManager.getLogger(this::class.java)

  override fun submitBlobs(blobsChunks: List<List<BlobRecord>>): SafeFuture<List<String>> {
    return blobsChunks
      .fold(SafeFuture.completedFuture(emptyList())) { chainOfFutures, blobs ->
        val newChainOfFutures = chainOfFutures
          .thenCompose { listOfTxHashes ->
            submitBlobsInSingleTx(blobs)
              .thenApply { txHash -> listOfTxHashes + txHash }
          }
        newChainOfFutures
      }
  }

  private fun submitBlobsInSingleTx(blobs: List<BlobRecord>): SafeFuture<String> {
    return (
      gasPriceCapProvider?.getGasPriceCaps(blobs.first().startBlockNumber.toLong())
        ?: SafeFuture.completedFuture(null)
      )
      .thenCompose { gasPriceCaps ->
        val nonce = contract.currentNonce()
        log.debug(
          "submitting shnarf data for blobs: blobs={} nonce={} gasPriceCaps={}",
          blobs.toBlockIntervalsString(),
          nonce,
          gasPriceCaps,
        )
        contract.acceptShnarfData(blobs, gasPriceCaps)
          .whenException { th ->
            logSubmissionError(
              log,
              blobs.toBlockIntervalsString(),
              th,
              isEthCall = false,
            )
          }
          .thenPeek { transactionHash ->
            log.info(
              "shnarf data for blobs submitted: blobs={} transactionHash={}, nonce={} gasPriceCaps={}",
              blobs.toBlockIntervalsString(),
              transactionHash,
              nonce,
              gasPriceCaps,
            )
            val blobSubmittedEvent = BlobSubmittedEvent(
              blobs = blobs.map { BlockIntervalData(it.startBlockNumber, it.endBlockNumber) },
              endBlockTime = blobs.last().endBlockTime,
              lastShnarf = blobs.last().expectedShnarf,
              submissionTimestamp = clock.now(),
              transactionHash = transactionHash.toByteArray(),
            )
            blobSubmittedEventConsumer.accept(blobSubmittedEvent)
          }
      }
  }

  override fun submitBlobCall(blobRecords: List<BlobRecord>): SafeFuture<*> {
    return (
      gasPriceCapProvider?.getGasPriceCapsWithCoefficient(blobRecords.first().startBlockNumber.toLong())
        ?: SafeFuture.completedFuture(null)
      )
      .thenCompose { gasPriceCaps ->
        val nonce = contract.currentNonce()
        log.debug(
          "eth_call submitting shnarf data for blobs: blobs={} nonce={} gasPriceCaps={}",
          blobRecords.toBlockIntervalsString(),
          nonce,
          gasPriceCaps,
        )
        contract.acceptShnarfDataEthCall(blobRecords, gasPriceCaps)
          .whenException { th ->
            logSubmissionError(log, blobRecords.toBlockIntervalsString(), th, isEthCall = true)
          }
          .thenPeek { _ ->
            log.debug(
              "eth_call shnarf data for blobs submission passed: blob={} nonce={} gasPriceCaps={}",
              blobRecords.toBlockIntervalsString(),
              nonce,
              gasPriceCaps,
            )
          }
      }
  }
}
