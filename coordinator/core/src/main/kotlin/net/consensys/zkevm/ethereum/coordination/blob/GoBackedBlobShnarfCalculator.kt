package net.consensys.zkevm.ethereum.coordination.blob

import linea.blob.GoNativeBlobShnarfCalculator
import linea.blob.GoNativeShnarfCalculatorFactory
import linea.blob.ShnarfCalculatorVersion
import linea.domain.BlobShnarfCalculator
import linea.domain.BlockIntervals
import linea.domain.ShnarfResult
import linea.kotlin.decodeHex
import linea.kotlin.encodeHex
import linea.metrics.LineaMetricsCategory
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.metrics.Timer
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import java.util.Base64

class GoBackedBlobShnarfCalculator(
  private val delegate: GoNativeBlobShnarfCalculator,
  private val metricsFacade: MetricsFacade,
) : BlobShnarfCalculator {
  constructor(version: ShnarfCalculatorVersion, metricsFacade: MetricsFacade) :
    this(GoNativeShnarfCalculatorFactory.getInstance(version), metricsFacade)

  private val calculateShnarfTimer: Timer =
    metricsFacade.createTimer(
      category = LineaMetricsCategory.BLOB,
      name = "shnarf.calculation",
      description = "Time taken to calculate the shnarf hash of the given blob",
    )

  private val log: Logger = LogManager.getLogger(GoBackedBlobShnarfCalculator::class.java)

  @Synchronized
  override fun calculateShnarf(
    compressedData: ByteArray,
    parentStateRootHash: ByteArray,
    finalStateRootHash: ByteArray,
    prevShnarf: ByteArray,
    conflationOrder: BlockIntervals,
  ): ShnarfResult {
    val compressedDataB64 = Base64.getEncoder().encodeToString(compressedData)
    log.trace(
      "calculateShnarf: " +
        "compressedDataHex={} " +
        "compressedDataBaseB64={} " +
        "parentStateRootHash={} " +
        "finalStateRootHash={} " +
        "prevShnarf={} " +
        "conflationOrder={}",
      compressedData.encodeHex(),
      compressedDataB64,
      parentStateRootHash.encodeHex(),
      finalStateRootHash.encodeHex(),
      prevShnarf.encodeHex(),
      conflationOrder,
    )

    val result =
      calculateShnarfTimer.captureTime {
        delegate.CalculateShnarf(
          eip4844Enabled = true,
          compressedData = compressedDataB64,
          parentStateRootHash = parentStateRootHash.encodeHex(),
          finalStateRootHash = finalStateRootHash.encodeHex(),
          prevShnarf = prevShnarf.encodeHex(),
          conflationOrderStartingBlockNumber = conflationOrder.startingBlockNumber.toLong(),
          conflationOrderUpperBoundariesLen = conflationOrder.upperBoundaries.size,
          conflationOrderUpperBoundaries = conflationOrder.upperBoundaries.map { it.toLong() }.toLongArray(),
        )
      }

    if (result.errorMessage.isNotEmpty()) {
      val errorMessage = "Error while calculating Shnarf. error=${result.errorMessage}"
      throw RuntimeException(errorMessage)
    }

    val domainResult =
      try {
        ShnarfResult(
          dataHash = result.dataHash.decodeHex(),
          snarkHash = result.snarkHash.decodeHex(),
          expectedX = result.expectedX.decodeHex(),
          expectedY = result.expectedY.decodeHex(),
          expectedShnarf = result.expectedShnarf.decodeHex(),
          commitment = result.commitment.decodeHex(),
          kzgProofContract = result.kzgProofContract.decodeHex(),
          kzgProofSideCar = result.kzgProofSideCar.decodeHex(),
        )
      } catch (e: Exception) {
        throw RuntimeException("Error while decoding Shnarf calculation response from Go: ${e.message}")
      }

    return domainResult
  }
}
