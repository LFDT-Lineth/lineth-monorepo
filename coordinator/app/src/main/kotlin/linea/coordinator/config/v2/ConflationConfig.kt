package linea.coordinator.config.v2

import linea.blob.BlobCompressorVersion
import linea.domain.RetryConfig
import net.consensys.linea.traces.TracesCountersV2
import java.net.URL
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds

data class ConflationConfig(
  override val disabled: Boolean = false,
  val blocksLimit: UInt? = null,
  val forceStopConflationAtBlockInclusive: ULong? = null,
  val blocksPollingInterval: Duration = 1.seconds,
  val conflationDeadline: Duration? = null, // disabled by default
  val conflationDeadlineCheckInterval: Duration = 10.seconds,
  // 24 second without blocks must elapse before conflation deadline is considered expired
  val conflationDeadlineLastBlockConfirmationDelay: Duration = 24.seconds,
  val consistentNumberOfBlocksOnL1ToWait: UInt = 32u, // 1 epoch
  val l2FetchBlocksLimit: UInt = UInt.MAX_VALUE,
  val l2Endpoint: URL,
  val l2RequestRetries: RetryConfig = RetryConfig.endlessRetry(
    backoffDelay = 1.seconds,
    failuresWarningThreshold = 3u,
  ),
  val l2GetLogsEndpoint: URL,
  val blobCompression: BlobCompression = BlobCompression(),
  val proofAggregation: ProofAggregation = ProofAggregation(),
  val tracesLimitsV2: TracesCountersV2,
) : FeatureToggle {
  data class BlobCompression(
    val blobSizeLimit: UInt = 102400u,
    val handlerPollingInterval: Duration = 1.seconds,
    val batchesLimit: UInt? = null,
    val blobCompressorVersion: BlobCompressorVersion = BlobCompressorVersion.V1_2,
  )

  data class ProofAggregation(
    val proofsLimit: UInt = 300u,
    val deadline: Duration = Duration.INFINITE,
    val deadlineCheckInterval: Duration = 30.seconds,
    val coordinatorPollingInterval: Duration = 3.seconds,
    val targetEndBlocks: List<ULong>? = null,
    val aggregationSizeMultipleOf: UInt = 1u,
  )
}
