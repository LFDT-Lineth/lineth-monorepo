package lineth.coordinator.config.v2.toml

import lineth.coordinator.config.v2.ConflationConfig
import lineth.blob.BlobCompressorVersion
import lineth.config.docs.ConfigDoc
import lineth.config.docs.ConfigSection
import lineth.traces.TracesCountersV4
import lineth.traces.TracesCountersV5
import java.net.URL
import java.nio.file.Path
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds
import kotlin.time.Instant

data class ConflationToml(
  @param:ConfigDoc(description = "Whether conflation is disabled.", default = "false")
  val disabled: Boolean = false,
  @param:ConfigDoc(
    description = "Maximum number of L2 blocks in a single conflation. Omit for no block-count limit.",
    example = "2",
  )
  val blocksLimit: UInt? = null,
  @param:ConfigDoc(
    description = "Inclusive L2 block number at which to stop conflating. Omit to conflate indefinitely.",
    example = "100000000",
  )
  val forceStopConflationAtBlockInclusive: ULong? = null,
  @param:ConfigDoc(
    description = "Inclusive block timestamp at which to stop conflating. Omit to conflate indefinitely.",
    example = "2024-01-01T00:00:00Z",
  )
  val forceStopConflationAtBlockTimestampInclusive: Instant? = null,
  @param:ConfigDoc(
    description = "Maximum time to wait before conflating available blocks even if limits are not reached. " +
      "Omit to disable deadline-based conflation.",
    example = "PT6S",
  )
  val conflationDeadline: Duration? = null,
  @param:ConfigDoc(
    description = "How often to check whether the conflation deadline has elapsed.",
    default = "PT30S",
  )
  val conflationDeadlineCheckInterval: Duration = 30.seconds,
  @param:ConfigDoc(
    description = "Delay after the last block before confirming it, to absorb short reorgs.",
    default = "PT30S",
  )
  val conflationDeadlineLastBlockConfirmationDelay: Duration = 30.seconds,
  @param:ConfigDoc(
    description = "Number of L1 blocks to wait for consistency (finality safety margin); defaults to one epoch.",
    default = "32",
  )
  val consistentNumberOfBlocksOnL1ToWait: UInt = 32u, // 1 epoch
  @param:ConfigDoc(
    description = "Interval between polls for new L2 blocks.",
    default = "PT1S",
  )
  val newBlocksPollingInterval: Duration = 1.seconds,
  @param:ConfigDoc(
    description = "Maximum number of L2 blocks to fetch that are unproven and held in memory. Omit for no limit.",
    example = "4000",
  )
  val l2FetchBlocksLimit: UInt? = null,
  @param:ConfigDoc(
    description = "L2 endpoint used for conflation. Falls back to defaults.l2-endpoint.",
    example = "http://sequencer:8545",
  )
  val l2Endpoint: URL? = null,
  @param:ConfigSection("Retry policy for L2 conflation requests; falls back to defaults.l2-request-retries.")
  val l2RequestRetries: RequestRetriesToml? = null,
  @param:ConfigDoc(
    description = "L2 endpoint used for eth_getLogs during conflation. Falls back to l2-endpoint/defaults.",
    example = "http://sequencer:8545",
  )
  val l2LogsEndpoint: URL? = null,
  @param:ConfigSection("Blob compression settings.")
  val blobCompression: BlobCompressionToml = BlobCompressionToml(),
  @param:ConfigSection("Proof aggregation settings.")
  val proofAggregation: ProofAggregationToml = ProofAggregationToml(),
  @param:ConfigDoc(
    description = "Directory used to persist conflation backtesting data. Omit to disable backtesting output.",
    example = "/data/conflation-backtesting",
  )
  val backtestingDirectory: Path? = null,
  @param:ConfigDoc(
    description = "RISC-V starting block timestamp (inclusive).",
    example = "2027-01-01T00:00:00Z",
  )
  val riscvStartingBlockTimestampInclusive: Instant? = null,
) {
  init {
    require(proofAggregation.proofsLimit >= (blobCompression.batchesLimit ?: 1u) + 1u) {
      "Aggregation proofsLimit must be greater than or equal to Blobs batchesLimit + 1 or " +
        "greater than or equal to 2 if batchesLimit is not set."
    }
  }

  data class BlobCompressionToml(
    @param:ConfigDoc(
      description = "Maximum compressed blob size in bytes before a blob is finalized.",
      default = "102400",
    )
    val blobSizeLimit: UInt = 102400u,
    @param:ConfigDoc(
      description = "Interval between blob compression handler runs.",
      default = "PT1S",
    )
    val handlerPollingInterval: Duration = 1.seconds,
    @param:ConfigDoc(
      description = "Maximum number of batches per blob. Omit for no explicit limit.",
      example = "1",
    )
    val batchesLimit: UInt? = null,
    @param:ConfigDoc(
      description = "Blob compressor version to use.",
      default = "V3",
    )
    val blobCompressorVersion: BlobCompressorVersion = BlobCompressorVersion.V3,
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
    @param:ConfigDoc(
      description = "Maximum number of proofs in a single aggregation.",
      default = "300",
    )
    val proofsLimit: UInt = 300u,
    @param:ConfigDoc(
      description = "Maximum number of blobs in a single aggregation. Omit for no explicit limit.",
      example = "6",
    )
    val blobsLimit: UInt? = null,
    @param:ConfigDoc(
      description = "Maximum time to wait before aggregating available proofs. Omit to disable deadline-based " +
        "aggregation (wait indefinitely).",
      example = "PT1M",
    )
    val deadline: Duration? = null,
    @param:ConfigDoc(
      description = "How often to check whether the aggregation deadline has elapsed.",
      default = "PT30S",
    )
    val deadlineCheckInterval: Duration = 30.seconds,
    @param:ConfigDoc(
      description = "Interval between coordinator polls for new proofs to aggregate.",
      default = "PT3S",
    )
    val coordinatorPollingInterval: Duration = 3.seconds,
    @param:ConfigDoc(
      description = "L2 block numbers that must end an aggregation (e.g. hard-fork boundaries). " +
        "Omit for none.",
      example = "[]",
    )
    val targetEndBlocks: List<ULong>? = null,
    @param:ConfigDoc(
      description = "Aggregations are only closed when their size is a multiple of this value.",
      default = "1",
    )
    val aggregationSizeMultipleOf: UInt = 1u,
    @param:ConfigDoc(
      description = "Timestamps that force an aggregation boundary (e.g. timestamp-based hard forks).",
      default = "[]",
    )
    val timestampBasedHardForks: List<Instant> = emptyList(),
    @param:ConfigDoc(
      description = "Whether to wait for a period of no L2 activity before triggering an aggregation.",
      default = "false",
    )
    val waitForNoL2ActivityToTriggerAggregation: Boolean = false,
    @param:ConfigDoc(
      description = "Whether to wait for the target block to be finalized on L1 before aggregating.",
      default = "false",
    )
    val waitTargetBlockL1Finalization: Boolean = false,
    @param:ConfigDoc(
      description = "Whether to wait for the API to resume after the target block before aggregating.",
      default = "false",
    )
    val waitApiResumeAfterTargetBlock: Boolean = false,
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
        waitTargetBlockL1Finalization = this.waitTargetBlockL1Finalization,
        waitApiResumeAfterTargetBlock = this.waitApiResumeAfterTargetBlock,
      )
    }
  }

  fun reified(
    defaults: DefaultsToml,
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
      tracesLimits = tracesCountersLimitsV4
        ?: requireNotNull(tracesCountersLimitsV5) {
          "either tracesCountersLimitsV4 or tracesCountersLimitsV5 must be set"
        },
      backtestingDirectory = backtestingDirectory,
      riscvStartingBlockTimestampInclusive = this.riscvStartingBlockTimestampInclusive,
    )
  }
}
