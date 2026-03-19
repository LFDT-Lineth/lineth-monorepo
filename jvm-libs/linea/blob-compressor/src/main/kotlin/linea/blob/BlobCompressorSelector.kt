package linea.blob

import kotlin.time.Instant

interface BlobCompressorSelector<T> {
  /**
   * Returns a BlobCompressor instance based on the provided selector.
   */
  fun getBlobCompressor(selector: T): BlobCompressor
}

class BlobCompressorSelectorByTimestamp(
  private val blobCompressorVersionSwitchTimestampMap: Map<BlobCompressorVersion, Instant>,
  private val dataLimit: Int,
) : BlobCompressorSelector<Instant> {

  init {
    require(
      blobCompressorVersionSwitchTimestampMap.values.toSet().size
        == blobCompressorVersionSwitchTimestampMap.size,
    ) {
      "Timestamps must be unique across BlobCompressor versions"
    }
  }

  private val blobCompressors = blobCompressorVersionSwitchTimestampMap.mapValues { (blobCompressorVersion, _) ->
    GoBackedBlobCompressor.getInstance(blobCompressorVersion, dataLimit)
  }

  override fun getBlobCompressor(selector: Instant): BlobCompressor {
    val blobCompressorVersion = blobCompressorVersionSwitchTimestampMap.entries
      .filter { selector >= it.value }
      .maxByOrNull { it.value }
      ?.key ?: throw IllegalStateException("No BlobCompressor version found for timestamp $selector")

    return blobCompressors[blobCompressorVersion]
      ?: throw IllegalStateException("BlobCompressor for version $blobCompressorVersion not found")
  }
}
