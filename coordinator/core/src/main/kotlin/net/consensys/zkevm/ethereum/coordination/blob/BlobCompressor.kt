package net.consensys.zkevm.ethereum.coordination.blob

import linea.blob.BlobCompressorVersion
import linea.blob.GoNativeBlobCompressor
import linea.blob.GoNativeBlobCompressorFactory
import linea.kotlin.encodeHex
import net.consensys.linea.metrics.LineaMetricsCategory
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.metrics.Timer
import org.apache.logging.log4j.LogManager
import kotlin.random.Random

class BlobCompressionException(message: String) : RuntimeException(message)

interface BlobCompressor {

  /**
   * @Throws(BlobCompressionException::class) when blockRLPEncoded is invalid
   */
  fun canAppendBlock(blockRLPEncoded: ByteArray): Boolean

  /**
   * @Throws(BlobCompressionException::class) when blockRLPEncoded is invalid
   */
  fun appendBlock(blockRLPEncoded: ByteArray): AppendResult

  fun startNewBatch()
  fun getCompressedData(): ByteArray
  fun reset()

  data class AppendResult(
    // returns false if last chunk would go over dataLimit. Does  not append last block.
    val blockAppended: Boolean,
    val compressedSizeBefore: Int,
    // even when block is not appended, compressedSizeAfter should as if it was appended
    val compressedSizeAfter: Int,
  )
}

class GoBackedBlobCompressor private constructor(
  internal val goNativeBlobCompressor: GoNativeBlobCompressor,
  private val dataLimit: UInt,
  private val metricsFacade: MetricsFacade,
) : BlobCompressor {

  companion object {
    @Volatile
    private var instance: GoBackedBlobCompressor? = null

    fun getInstance(
      compressorVersion: BlobCompressorVersion,
      dataLimit: UInt,
      metricsFacade: MetricsFacade,
    ): GoBackedBlobCompressor {
      if (instance == null) {
        synchronized(this) {
          if (instance == null) {
            val goNativeBlobCompressor = GoNativeBlobCompressorFactory.getInstance(compressorVersion)
            val initialized = goNativeBlobCompressor.Init(
              dataLimit.toInt(),
              GoNativeBlobCompressorFactory.dictionaryPath.toString(),
            )
            if (!initialized) {
              throw InstantiationException(goNativeBlobCompressor.Error())
            }
            instance = GoBackedBlobCompressor(goNativeBlobCompressor, dataLimit, metricsFacade)
          } else {
            throw IllegalStateException("Compressor singleton instance already created")
          }
        }
      } else {
        throw IllegalStateException("Compressor singleton instance already created")
      }
      return instance!!
    }
  }

  private val canAppendBlockTimer: Timer = metricsFacade.createTimer(
    category = LineaMetricsCategory.BLOB,
    name = "compressor.canappendblock",
    description = "Time taken to check if block fits in current blob",
  )
  private val appendBlockTimer: Timer = metricsFacade.createTimer(
    category = LineaMetricsCategory.BLOB,
    name = "compressor.appendblock",
    description = "Time taken to compress block into current blob",
  )
  private val compressionRatioHistogram = metricsFacade.createHistogram(
    category = LineaMetricsCategory.BLOB,
    name = "block.compression.ratio",
    description = "Block compression ratio measured in [0.0,1.0]",
    isRatio = true,
  )
  private val utilizationRatioHistogram = metricsFacade.createHistogram(
    category = LineaMetricsCategory.BLOB,
    name = "data.utilization.ratio",
    description = "Data utilization ratio of a blob measured in [0.0,1.0]",
    isRatio = true,
  )

  private val log = LogManager.getLogger(GoBackedBlobCompressor::class.java)

  override fun canAppendBlock(blockRLPEncoded: ByteArray): Boolean {
    return canAppendBlockTimer.captureTime {
      goNativeBlobCompressor.CanWrite(blockRLPEncoded, blockRLPEncoded.size)
    }
  }

  override fun appendBlock(blockRLPEncoded: ByteArray): BlobCompressor.AppendResult {
    val compressionSizeBefore = goNativeBlobCompressor.Len()
    val appended = appendBlockTimer.captureTime {
      goNativeBlobCompressor.Write(blockRLPEncoded, blockRLPEncoded.size)
    }
    val compressedSizeAfter = goNativeBlobCompressor.Len()
    val compressionRatio = (1.0 - (compressedSizeAfter - compressionSizeBefore).toDouble() / blockRLPEncoded.size)
      .also { compressionRatioHistogram.record(it) }

    log.trace(
      "block compressed: blockRlpSize={} compressionDataBefore={} compressionDataAfter={} compressionRatio={}",
      blockRLPEncoded.size,
      compressionSizeBefore,
      compressedSizeAfter,
      compressionRatio,
    )
    val error = goNativeBlobCompressor.Error()
    if (error != null) {
      log.error("Failure while writing the following RLP encoded block: {}", blockRLPEncoded.encodeHex())
      throw BlobCompressionException(error)
    }
    return BlobCompressor.AppendResult(appended, compressionSizeBefore, compressedSizeAfter)
  }

  override fun startNewBatch() {
    goNativeBlobCompressor.StartNewBatch()
  }

  override fun getCompressedData(): ByteArray {
    val compressedData = ByteArray(goNativeBlobCompressor.Len())
    goNativeBlobCompressor.Bytes(compressedData)
    utilizationRatioHistogram.record(goNativeBlobCompressor.Len().toDouble() / dataLimit.toInt())
    return compressedData
  }

  override fun reset() {
    goNativeBlobCompressor.Reset()
  }
}

class FakeBlobCompressor(
  private val dataLimit: Int,
  // 1 means no compression
  // 0.5 means compressed data is 50% smaller than original
  // 1.1 means compressed data is 10% bigger than original
  private val fakeCompressionRatio: Double = 1.0,
) : BlobCompressor {
  val log = LogManager.getLogger(FakeBlobCompressor::class.java)

  init {
    require(dataLimit > 0) { "dataLimit must be greater than 0" }
    require(fakeCompressionRatio > 0.0) { "fakeCompressionRatio must be greater than 0" }
  }

  private var compressedData = byteArrayOf()

  override fun canAppendBlock(blockRLPEncoded: ByteArray): Boolean {
    return compressedData.size + blockRLPEncoded.size * fakeCompressionRatio <= dataLimit
  }

  override fun appendBlock(blockRLPEncoded: ByteArray): BlobCompressor.AppendResult {
    return buildAppendResult(blockRLPEncoded)
  }

  private fun buildAppendResult(blockRLPEncoded: ByteArray): BlobCompressor.AppendResult {
    val compressedBlock = Random.nextBytes((blockRLPEncoded.size * fakeCompressionRatio).toInt())
    val compressedSizeBefore = compressedData.size
    val compressedSizeAfter = compressedData.size + compressedBlock.size
    return if (compressedSizeAfter > dataLimit) {
      BlobCompressor.AppendResult(
        false,
        compressedSizeBefore,
        compressedSizeAfter,
      )
    } else {
      compressedData += compressedBlock
      BlobCompressor.AppendResult(true, compressedSizeBefore, compressedSizeAfter)
    }
  }

  override fun startNewBatch() {
    // we don't care.
  }

  override fun getCompressedData(): ByteArray {
    return compressedData
  }

  override fun reset() {
    log.trace("resetting to empty state")
    compressedData = byteArrayOf()
  }
}
