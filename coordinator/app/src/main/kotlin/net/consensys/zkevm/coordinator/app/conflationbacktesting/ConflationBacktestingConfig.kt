package net.consensys.zkevm.coordinator.app.conflationbacktesting

import linea.blob.BlobCompressorVersion
import java.net.URL

data class ConflationBacktestingConfig(
  val startBlockNumber: ULong,
  val endBlockNumber: ULong,
  val blobCompressorVersion: BlobCompressorVersion,
  val batchesFixedSize: UInt? = null,
  val parentBlobShnarf: String? = null,
  val tracesApi: TracesApiConfig,
  val shomeiApi: ShomeiApiConfig,
) {
  fun jobId(): String {
    return "$startBlockNumber-$endBlockNumber-${this.hashCode()}"
  }
}

data class TracesApiConfig(
  val endpoint: URL,
  val version: String,
  val requestLimitPerEndpoint: UInt = 10u,
)

data class ShomeiApiConfig(
  val endpoint: URL,
  val version: String,
  val requestLimitPerEndpoint: UInt = 10u,
)
