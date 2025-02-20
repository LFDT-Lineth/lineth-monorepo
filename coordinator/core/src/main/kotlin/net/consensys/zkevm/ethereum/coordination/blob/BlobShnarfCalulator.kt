package net.consensys.zkevm.ethereum.coordination.blob

import linea.domain.BlockIntervals
import linea.kotlin.decodeHex
import linea.kotlin.encodeHex
import net.consensys.linea.blob.CalculateShnarfResult
import net.consensys.linea.blob.GoNativeBlobShnarfCalculator
import net.consensys.linea.blob.GoNativeShnarfCalculatorFactory
import net.consensys.linea.blob.ShnarfCalculatorVersion
import net.consensys.linea.metrics.LineaMetricsCategory
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.metrics.TimerCapture
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import java.util.Base64
import kotlin.random.Random

/*
 * All hashes, shnarf, x, y values have 32 bytes
 */
data class ShnarfResult(
  val dataHash: ByteArray,
  val snarkHash: ByteArray,
  val expectedX: ByteArray,
  val expectedY: ByteArray,
  val expectedShnarf: ByteArray,
  val commitment: ByteArray,
  val kzgProofContract: ByteArray,
  val kzgProofSideCar: ByteArray
) {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as ShnarfResult

    if (!dataHash.contentEquals(other.dataHash)) return false
    if (!snarkHash.contentEquals(other.snarkHash)) return false
    if (!expectedX.contentEquals(other.expectedX)) return false
    if (!expectedY.contentEquals(other.expectedY)) return false
    if (!expectedShnarf.contentEquals(other.expectedShnarf)) return false
    if (!commitment.contentEquals(other.commitment)) return false
    if (!kzgProofContract.contentEquals(other.kzgProofContract)) return false
    if (!kzgProofSideCar.contentEquals(other.kzgProofSideCar)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = dataHash.contentHashCode()
    result = 31 * result + snarkHash.contentHashCode()
    result = 31 * result + expectedX.contentHashCode()
    result = 31 * result + expectedY.contentHashCode()
    result = 31 * result + expectedShnarf.contentHashCode()
    result = 31 * result + commitment.contentHashCode()
    result = 31 * result + kzgProofContract.contentHashCode()
    result = 31 * result + kzgProofSideCar.contentHashCode()
    return result
  }
}

interface BlobShnarfCalculator {
  fun calculateShnarf(
    compressedData: ByteArray,
    parentStateRootHash: ByteArray,
    finalStateRootHash: ByteArray,
    prevShnarf: ByteArray,
    conflationOrder: BlockIntervals
  ): ShnarfResult
}

class GoBackedBlobShnarfCalculator(
  private val delegate: GoNativeBlobShnarfCalculator,
  private val metricsFacade: MetricsFacade
) : BlobShnarfCalculator {
  constructor(version: ShnarfCalculatorVersion, metricsFacade: MetricsFacade) :
    this(GoNativeShnarfCalculatorFactory.getInstance(version), metricsFacade)

  private val calculateShnarfTimer: TimerCapture<CalculateShnarfResult> = metricsFacade.createSimpleTimer(
    category = LineaMetricsCategory.BLOB,
    name = "shnarf.calculation",
    description = "Time taken to calculate the shnarf hash of the given blob"
  )

  private val log: Logger = LogManager.getLogger(GoBackedBlobShnarfCalculator::class.java)

  @Synchronized
  override fun calculateShnarf(
    compressedData: ByteArray,
    parentStateRootHash: ByteArray,
    finalStateRootHash: ByteArray,
    prevShnarf: ByteArray,
    conflationOrder: BlockIntervals
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
      conflationOrder
    )

    val result = calculateShnarfTimer.captureTime {
      delegate.CalculateShnarf(
        eip4844Enabled = true,
        compressedData = compressedDataB64,
        parentStateRootHash = parentStateRootHash.encodeHex(),
        finalStateRootHash = finalStateRootHash.encodeHex(),
        prevShnarf = prevShnarf.encodeHex(),
        conflationOrderStartingBlockNumber = conflationOrder.startingBlockNumber.toLong(),
        conflationOrderUpperBoundariesLen = conflationOrder.upperBoundaries.size,
        conflationOrderUpperBoundaries = conflationOrder.upperBoundaries.map { it.toLong() }.toLongArray()
      )
    }

    if (result.errorMessage.isNotEmpty()) {
      val errorMessage = "Error while calculating Shnarf. error=${result.errorMessage}"
      throw RuntimeException(errorMessage)
    }

    val domainResult = try {
      ShnarfResult(
        dataHash = result.dataHash.decodeHex(),
        snarkHash = result.snarkHash.decodeHex(),
        expectedX = result.expectedX.decodeHex(),
        expectedY = result.expectedY.decodeHex(),
        expectedShnarf = result.expectedShnarf.decodeHex(),
        commitment = result.commitment.decodeHex(),
        kzgProofContract = result.kzgProofContract.decodeHex(),
        kzgProofSideCar = result.kzgProofSideCar.decodeHex()
      )
    } catch (it: Exception) {
      throw RuntimeException("Error while decoding Shnarf calculation response from Go: ${it.message}")
    }

    return domainResult
  }
}

/**
 * Used for testing purposes while real implementation is not ready
 * Shall be removed very soon
 */
class FakeBlobShnarfCalculator : BlobShnarfCalculator {
  override fun calculateShnarf(
    compressedData: ByteArray,
    parentStateRootHash: ByteArray,
    finalStateRootHash: ByteArray,
    prevShnarf: ByteArray,
    conflationOrder: BlockIntervals
  ): ShnarfResult {
    return ShnarfResult(
      dataHash = Random.nextBytes(32),
      snarkHash = Random.nextBytes(32),
      expectedX = Random.nextBytes(32),
      expectedY = Random.nextBytes(32),
      expectedShnarf = Random.nextBytes(32),
      commitment = Random.nextBytes(48),
      kzgProofContract = Random.nextBytes(48),
      kzgProofSideCar = Random.nextBytes(48)
    )
  }
}
