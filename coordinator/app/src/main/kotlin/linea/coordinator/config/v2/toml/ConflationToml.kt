package linea.coordinator.config.v2.toml

import kotlinx.datetime.Instant
import linea.blob.BlobCompressorVersion
import linea.coordinator.config.v2.ConflationConfig
import net.consensys.linea.traces.TracesCountersV2
import net.consensys.linea.traces.TracesCountersV4
import net.consensys.linea.traces.TracesCountersV5
import java.net.URL
import java.nio.file.Path
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds

data class ConflationToml(
  val disabled: Boolean = false,
  val blocksLimit: UInt? = null,
  val forceStopConflationAtBlockInclusive: ULong? = null,
  val forceStopConflationAtBlockTimestampInclusive: Instant? = null,
  val conflationDeadline: Duration? = null,
  val conflationDeadlineCheckInterval: Duration = 30.seconds,
  val conflationDeadlineLastBlockConfirmationDelay: Duration = 30.seconds,
  val consistentNumberOfBlocksOnL1ToWait: UInt = 32u, // 1 epoch
  val newBlocksPollingInterval: Duration = 1.seconds,
  val l2FetchBlocksLimit: UInt? = null,
  val l2Endpoint: URL? = null,
  val l2RequestRetries: RequestRetriesToml? = null,
  val l2LogsEndpoint: URL? = null,
  val blobCompression: BlobCompressionToml = BlobCompressionToml(),
  val proofAggregation: ProofAggregationToml = ProofAggregationToml(),
  val backtestingDirectory: Path? = null,
) {
  init {
    require(proofAggregation.proofsLimit >= (blobCompression.batchesLimit ?: 1u) + 1u) {
      "Aggregation proofsLimit must be greater than or equal to Blobs batchesLimit + 1 or " +
        "greater than or equal to 2 if batchesLimit is not set."
    }
  }

  data class BlobCompressionToml(
    val blobSizeLimit: UInt = 102400u,
    val handlerPollingInterval: Duration = 1.seconds,
    val batchesLimit: UInt? = null,
    val blobCompressorVersion: BlobCompressorVersion = BlobCompressorVersion.V1_2,
  ) {
    fun reified(): ConflationConfig.BlobCompression {
      return ConflationConfig.BlobCompression(
        blobSizeLimit = this.blobSizeLimit,
        handlerPollingInterval = this.handlerPollingInterval,
        batchesLimit = this.batchesLimit,
        blobCompressorVersion = this.blobCompressorVersion,
      )
    }
  }

  data class ProofAggregationToml(
    val proofsLimit: UInt = 300u,
    val blobsLimit: UInt? = null,
    val deadline: Duration? = null,
    val deadlineCheckInterval: Duration = 30.seconds,
    val coordinatorPollingInterval: Duration = 3.seconds,
    val targetEndBlocks: List<ULong>? = null,
    val aggregationSizeMultipleOf: UInt = 1u,
    val timestampBasedHardForks: List<Instant> = emptyList(),
    val waitForNoL2ActivityToTriggerAggregation: Boolean = false,
  ) {
    init {
      require(timestampBasedHardForks.toSet().size == timestampBasedHardForks.size) {
        "Timestamps list contains duplicates! Probably a misconfiguration!"
      }
    }

    fun reified(): ConflationConfig.ProofAggregation {
      return ConflationConfig.ProofAggregation(
        proofsLimit = this.proofsLimit,
        blobsLimit = this.blobsLimit,
        deadline = this.deadline ?: Duration.INFINITE,
        deadlineCheckInterval = this.deadlineCheckInterval,
        coordinatorPollingInterval = this.coordinatorPollingInterval,
        targetEndBlocks = this.targetEndBlocks,
        aggregationSizeMultipleOf = this.aggregationSizeMultipleOf,
        timestampBasedHardForks = timestampBasedHardForks,
        waitForNoL2ActivityToTriggerAggregation = this.waitForNoL2ActivityToTriggerAggregation,
      )
    }
  }

  fun reified(
    defaults: DefaultsToml,
    tracesCountersLimitsV2: TracesCountersV2?,
    tracesCountersLimitsV4: TracesCountersV4?,
    tracesCountersLimitsV5: TracesCountersV5?,
  ): ConflationConfig {
    return ConflationConfig(
      disabled = this.disabled,
      blocksLimit = this.blocksLimit,
      forceStopConflationAtBlockInclusive = this.forceStopConflationAtBlockInclusive,
      forceStopConflationAtBlockTimestampInclusive = this.forceStopConflationAtBlockTimestampInclusive,
      blocksPollingInterval = this.newBlocksPollingInterval,
      conflationDeadline = this.conflationDeadline,
      conflationDeadlineCheckInterval = this.conflationDeadlineCheckInterval,
      conflationDeadlineLastBlockConfirmationDelay = this.conflationDeadlineLastBlockConfirmationDelay,
      consistentNumberOfBlocksOnL1ToWait = this.consistentNumberOfBlocksOnL1ToWait,
      l2FetchBlocksLimit = this.l2FetchBlocksLimit ?: UInt.MAX_VALUE,
      l2Endpoint = this.l2Endpoint
        ?: defaults.l2Endpoint
        ?: throw AssertionError("l2Endpoint config missing"),
      l2RequestRetries = this.l2RequestRetries?.asDomain
        ?: defaults.l2RequestRetries.asDomain,
      l2GetLogsEndpoint = this.l2LogsEndpoint
        ?: this.l2Endpoint
        ?: defaults.l2Endpoint
        ?: throw AssertionError("please set l2GetLogsEndpoint or l2Endpoint config"),
      blobCompression = this.blobCompression.reified(),
      proofAggregation = this.proofAggregation.reified(),
      tracesLimits = tracesCountersLimitsV2 ?: tracesCountersLimitsV4 ?: tracesCountersLimitsV5!!,
      backtestingDirectory = backtestingDirectory,
    )
  }
}
