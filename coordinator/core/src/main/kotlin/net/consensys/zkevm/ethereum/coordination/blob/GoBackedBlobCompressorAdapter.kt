package net.consensys.zkevm.ethereum.coordination.blob

import linea.blob.BlobCompressor
import linea.blob.BlobCompressorFactory
import linea.blob.BlobCompressorVersion
import linea.metrics.LineaMetricsCategory
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.metrics.Timer
import org.apache.logging.log4j.LogManager

class GoBackedBlobCompressorAdapter private constructor(
  internal val goBackedBlobCompressor: BlobCompressor,
  private val dataLimit: UInt,
  private val metricsFacade: MetricsFacade,
) : BlobCompressor by goBackedBlobCompressor {
  companion object {
    @Volatile
    private var instance: GoBackedBlobCompressorAdapter? = null

    fun getInstance(
      compressorVersion: BlobCompressorVersion,
      dataLimit: UInt,
      metricsFacade: MetricsFacade,
    ): GoBackedBlobCompressorAdapter {
      if (instance == null) {
        synchronized(this) {
          if (instance == null) {
            val goBackedBlobCompressor = BlobCompressorFactory.getInstance(
              compressorVersion = compressorVersion,
              dataLimit = dataLimit.toInt(),
            )
            instance = GoBackedBlobCompressorAdapter(goBackedBlobCompressor, dataLimit, metricsFacade)
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

  private val canAppendBlockTimer: Timer =
    metricsFacade.createTimer(
      category = LineaMetricsCategory.BLOB,
      name = "compressor.canappendblock",
      description = "Time taken to check if block fits in current blob",
    )
  private val appendBlockTimer: Timer =
    metricsFacade.createTimer(
      category = LineaMetricsCategory.BLOB,
      name = "compressor.appendblock",
      description = "Time taken to compress block into current blob",
    )
  private val compressionRatioHistogram =
    metricsFacade.createHistogram(
      category = LineaMetricsCategory.BLOB,
      name = "block.compression.ratio",
      description = "Block compression ratio measured in [0.0,1.0]",
      isRatio = true,
    )
  private val utilizationRatioHistogram =
    metricsFacade.createHistogram(
      category = LineaMetricsCategory.BLOB,
      name = "data.utilization.ratio",
      description = "Data utilization ratio of a blob measured in [0.0,1.0]",
      isRatio = true,
    )

  private val log = LogManager.getLogger(GoBackedBlobCompressorAdapter::class.java)

  override fun canAppendBlock(blockRLPEncoded: ByteArray): Boolean {
    return canAppendBlockTimer.captureTime {
      goBackedBlobCompressor.canAppendBlock(blockRLPEncoded)
    }
  }

  override fun appendBlock(blockRLPEncoded: ByteArray): BlobCompressor.AppendResult {
    val appendResult =
      appendBlockTimer.captureTime {
        goBackedBlobCompressor.appendBlock(blockRLPEncoded)
      }
    val compressionSizeBefore = appendResult.compressedSizeBefore
    val compressedSizeAfter = appendResult.compressedSizeAfter
    val compressionRatio =
      (1.0 - (compressedSizeAfter - compressionSizeBefore).toDouble() / blockRLPEncoded.size)
        .also { compressionRatioHistogram.record(it) }

    log.trace(
      "block compressed: blockRlpSize={} compressionDataBefore={} compressionDataAfter={} compressionRatio={}",
      blockRLPEncoded.size,
      compressionSizeBefore,
      compressedSizeAfter,
      compressionRatio,
    )

    return appendResult
  }

  override fun getCompressedData(): ByteArray {
    val compressedData = goBackedBlobCompressor.getCompressedData()
    utilizationRatioHistogram.record(compressedData.size.toDouble() / dataLimit.toInt())
    return compressedData
  }
}
