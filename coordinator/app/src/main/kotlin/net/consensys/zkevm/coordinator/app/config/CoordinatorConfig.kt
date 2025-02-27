package net.consensys.zkevm.coordinator.app.config

import com.sksamuel.hoplite.ConfigAlias
import com.sksamuel.hoplite.Masked
import linea.domain.BlockParameter
import linea.domain.assertIsValidAddress
import linea.kotlin.assertIs32Bytes
import linea.kotlin.decodeHex
import net.consensys.linea.blob.BlobCompressorVersion
import net.consensys.linea.ethereum.gaspricing.dynamiccap.MAX_FEE_HISTORIES_STORAGE_PERIOD
import net.consensys.linea.ethereum.gaspricing.dynamiccap.MAX_FEE_HISTORY_BLOCK_COUNT
import net.consensys.linea.ethereum.gaspricing.dynamiccap.MAX_REWARD_PERCENTILES_SIZE
import net.consensys.linea.ethereum.gaspricing.dynamiccap.TimeOfDayMultipliers
import net.consensys.linea.ethereum.gaspricing.dynamiccap.getAllTimeOfDayKeys
import net.consensys.linea.jsonrpc.client.RequestRetryConfig
import net.consensys.linea.traces.TracesCounters
import net.consensys.linea.traces.TracesCountersV1
import net.consensys.linea.traces.TracesCountersV2
import net.consensys.linea.traces.TracingModuleV1
import net.consensys.linea.traces.TracingModuleV2
import net.consensys.linea.web3j.SmartContractErrors
import net.consensys.zkevm.coordinator.app.L2NetworkGasPricingService
import net.consensys.zkevm.coordinator.clients.prover.ProversConfig
import java.math.BigInteger
import java.net.URL
import java.nio.file.Path
import java.time.Duration
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.minutes
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration
import kotlin.time.toKotlinDuration

data class ApiConfig(
  val observabilityPort: UInt
)

data class ConflationConfig(
  val consistentNumberOfBlocksOnL1ToWait: Int,
  val conflationDeadline: Duration,
  val conflationDeadlineCheckInterval: Duration,
  val conflationDeadlineLastBlockConfirmationDelay: Duration,
  val blocksLimit: Long? = null,
  private var _tracesLimitsV1: TracesCountersV1?,
  private var _tracesLimitsV2: TracesCountersV2?,
  private var _smartContractErrors: SmartContractErrors?,
  val fetchBlocksLimit: Int,
  @ConfigAlias("conflation-target-end-block-numbers")
  private val _conflationTargetEndBlockNumbers: List<Long> = emptyList()
) {

  init {
    require(conflationDeadlineCheckInterval <= conflationDeadline) {
      "Clock ticker interval must be smaller than conflation deadline"
    }
    consistentNumberOfBlocksOnL1ToWait.let {
      require(it > 0) { "consistentNumberOfBlocksOnL1ToWait must be grater than 0" }
    }
    blocksLimit?.let { require(it > 0) { "blocksLimit must be greater than 0" } }

    _smartContractErrors = _smartContractErrors
      ?.let { it.mapKeys { it.key.lowercase() } }
      ?: emptyMap()
  }

  val tracesLimitsV1: TracesCounters
    get() = _tracesLimitsV1 ?: throw IllegalStateException("Traces limits not defined!")

  val tracesLimitsV2: TracesCounters
    get() = _tracesLimitsV2 ?: throw IllegalStateException("Traces limits not defined!")
  val smartContractErrors: SmartContractErrors = _smartContractErrors!!

  val conflationTargetEndBlockNumbers: Set<ULong> = _conflationTargetEndBlockNumbers.map { it.toULong() }.toSet()
}

data class ZkTraces(
  val ethApi: URL,
  val newBlockPollingInterval: Duration
)

interface RetryConfig {
  val maxRetries: Int?
  val timeout: Duration?
  val backoffDelay: Duration
}

/**
 * This class is used to parse the requestRetry config from the toml file.
 * If we use UInt toml parser throws an exception because it does not support SOMETIMES UInt values: ¯\_(ツ)_/¯
 *
 *  kotlin.reflect.jvm.internal.KotlinReflectionInternalError:
 *  This callable does not support a default call: public constructor
 *  RequestRetryConfigTomlFriendly(maxRetries: kotlin.UInt? = ...
 */
data class RequestRetryConfigTomlFriendly(
  override val maxRetries: Int? = null,
  override val timeout: Duration? = null,
  override val backoffDelay: Duration,
  val failuresWarningThreshold: Int = 0
) : RetryConfig {
  init {
    maxRetries?.also {
      require(maxRetries > 0) { "maxRetries must be greater than zero. value=$maxRetries" }
    }
    timeout?.also {
      require(timeout.toKotlinDuration() > 0.milliseconds) { "timeout must be >= 1ms. value=$timeout" }
    }
  }

  internal val asDomain = RequestRetryConfig(
    maxRetries = maxRetries?.toUInt(),
    timeout = timeout?.toKotlinDuration(),
    backoffDelay = backoffDelay.toKotlinDuration(),
    failuresWarningThreshold = failuresWarningThreshold.toUInt()
  )
}

data class PersistenceRetryConfig(
  override val maxRetries: Int? = null,
  override val backoffDelay: Duration = 1.seconds.toJavaDuration(),
  override val timeout: Duration? = 10.minutes.toJavaDuration()
) : RetryConfig

internal interface RequestRetryConfigurable {
  val requestRetry: RequestRetryConfigTomlFriendly
  val requestRetryConfig: RequestRetryConfig
    get() = requestRetry.asDomain
}

data class BlobCompressionConfig(
  val blobSizeLimit: Int,
  @ConfigAlias("batches-limit")
  private val _batchesLimit: Int? = null,
  val handlerPollingInterval: Duration
) {
  init {
    _batchesLimit?.also {
      require(it > 0) { "batchesLimit=$_batchesLimit must be greater than 0" }
    }
  }

  val batchesLimit: UInt?
    get() = _batchesLimit?.toUInt()
}

data class AggregationConfig(
  val aggregationProofsLimit: Int,
  val aggregationDeadline: Duration,
  val aggregationCoordinatorPollingInterval: Duration,
  val deadlineCheckInterval: Duration,
  val aggregationSizeMultipleOf: Int = 1,
  @ConfigAlias("target-end-blocks")
  private val _targetEndBlocks: List<Long> = emptyList()
) {
  val targetEndBlocks: List<ULong> = _targetEndBlocks.map { it.toULong() }

  init {
    require(aggregationSizeMultipleOf > 0) { "aggregationSizeMultipleOf should be greater than 0" }
  }
}

data class TracesConfig(
  val rawExecutionTracesVersion: String,
  val expectedTracesApiVersion: String,
  val counters: FunctionalityEndpoint,
  val conflation: FunctionalityEndpoint,
  val fileManager: FileManager,
  val switchToLineaBesu: Boolean = false,
  val blobCompressorVersion: BlobCompressorVersion,
  val expectedTracesApiVersionV2: String? = null,
  val countersV2: FunctionalityEndpoint? = null,
  val conflationV2: FunctionalityEndpoint? = null
) {
  init {
    if (switchToLineaBesu) {
      require(expectedTracesApiVersionV2 != null) {
        "expectedTracesApiVersionV2 is required when switching to linea besu for tracing"
      }
      require(countersV2 != null) { "countersV2 is required when switching to linea besu for tracing" }
      require(conflationV2 != null) { "conflationV2 is required when switching to linea besu for tracing" }
    }
  }

  data class FunctionalityEndpoint(
    val endpoints: List<URL>,
    val requestLimitPerEndpoint: UInt,
    override val requestRetry: RequestRetryConfigTomlFriendly
  ) : RequestRetryConfigurable {
    init {
      require(requestLimitPerEndpoint > 0u) { "requestLimitPerEndpoint must be greater than 0" }
    }
  }

  data class FileManager(
    val tracesFileExtension: String,
    val rawTracesDirectory: Path,
    val nonCanonicalRawTracesDirectory: Path,
    val createNonCanonicalDirectory: Boolean,
    val pollingInterval: Duration,
    val tracesFileCreationWaitTimeout: Duration
  )
}

data class StateManagerClientConfig(
  val version: String,
  val endpoints: List<URL>,
  val requestLimitPerEndpoint: UInt,
  override val requestRetry: RequestRetryConfigTomlFriendly
) : RequestRetryConfigurable {
  init {
    require(requestLimitPerEndpoint > 0u) { "requestLimitPerEndpoint must be greater than 0" }
  }
}

data class BlobSubmissionConfig(
  val dbPollingInterval: Duration,
  val maxBlobsToReturn: Int,
  val proofSubmissionDelay: Duration,
  val priorityFeePerGasUpperBound: ULong,
  val priorityFeePerGasLowerBound: ULong,
  val maxBlobsToSubmitPerTick: Int = maxBlobsToReturn,
  // defaults to 6, not supported atm, preparatory work
  val targetBlobsToSendPerTransaction: Int = 6,
  val useEthEstimateGas: Boolean = false,
  override var disabled: Boolean = false
) : FeatureToggleable {
  init {
    require(maxBlobsToReturn > 0) { "maxBlobsToReturn must be greater than 0" }
    require(maxBlobsToSubmitPerTick >= 0) { "submissionLimit must be greater or equal to 0" }
    require(targetBlobsToSendPerTransaction in 1..6) {
      "targetBlobsToSendPerTransaction must be between 1 and 6, value=$targetBlobsToSendPerTransaction"
    }
  }
}

data class AggregationFinalizationConfig(
  val dbPollingInterval: Duration,
  val maxAggregationsToFinalizePerTick: Int,
  val proofSubmissionDelay: Duration,
  val useEthEstimateGas: Boolean = false,
  override var disabled: Boolean = false
) : FeatureToggleable {
  init {
    require(maxAggregationsToFinalizePerTick > 0) {
      "maxAggregationsToFinalizePerIteration must be greater than 0"
    }
  }
}

data class DatabaseConfig(
  val host: String,
  val port: Int,
  val username: String,
  val password: Masked,
  val schema: String,
  val readPoolSize: Int,
  val readPipeliningLimit: Int,
  val transactionalPoolSize: Int
)

data class L1Config(
  val zkEvmContractAddress: String,
  val rpcEndpoint: URL,
  val finalizationPollingInterval: Duration,
  @ConfigAlias("l1-query-block-tag")
  private val _l1QueryBlockTag: String = BlockParameter.Tag.FINALIZED.name,
  val gasLimit: ULong,
  val feeHistoryBlockCount: Int,
  val feeHistoryRewardPercentile: Double,
  val maxFeePerGasCap: ULong,
  val maxFeePerBlobGasCap: ULong,
  val maxPriorityFeePerGasCap: ULong,
  val gasPriceCapMultiplierForFinalization: Double,
  val earliestBlock: BigInteger,
  val sendMessageEventPollingInterval: Duration,
  val maxEventScrapingTime: Duration,
  val maxMessagesToCollect: UInt,
  val finalizedBlockTag: String,
  val blockRangeLoopLimit: UInt = 0U,
  val blockTime: Duration = Duration.parse("PT12S"),
  @ConfigAlias("eth-fee-history-endpoint") private val _ethFeeHistoryEndpoint: URL?,
  @ConfigAlias("genesis-state-root-hash") private val _genesisStateRootHash: String,
  @ConfigAlias("genesis-shnarf-v6") private val _genesisShnarfV6: String
) {
  val ethFeeHistoryEndpoint: URL
    get() = _ethFeeHistoryEndpoint ?: rpcEndpoint

  val genesisStateRootHash: ByteArray
    get() = _genesisStateRootHash.decodeHex().assertIs32Bytes("genesisStateRootHash")
  val genesisShnarfV6: ByteArray
    get() = _genesisShnarfV6.decodeHex().assertIs32Bytes("genesisShnarfV6")

  val l1QueryBlockTag: BlockParameter.Tag
    get() = BlockParameter.Tag.fromString(_l1QueryBlockTag)

  init {
    require(gasPriceCapMultiplierForFinalization > 0.0) {
      "gas price cap multiplier for finalization must be greater than 0.0." +
        " Value=$gasPriceCapMultiplierForFinalization"
    }
    // just to ensure that the tag is valid right at config parsing time
    BlockParameter.Tag.fromString(_l1QueryBlockTag)
  }
}

data class L2Config(
  val messageServiceAddress: String,
  val rpcEndpoint: URL,
  val gasLimit: ULong,
  val maxFeePerGasCap: ULong,
  val feeHistoryBlockCount: UInt,
  val feeHistoryRewardPercentile: Double,
  val blocksToFinalization: UInt,
  val lastHashSearchWindow: UInt,
  val anchoringReceiptPollingInterval: Duration,
  val maxReceiptRetries: UInt
) {
  init {
    messageServiceAddress.assertIsValidAddress("messageServiceAddress")
  }
}

data class SignerConfig(
  val type: Type,
  val web3signer: Web3SignerConfig?,
  val web3j: Web3jConfig?
) {
  enum class Type {
    Web3j,
    Web3Signer
  }

  init {
    when (type) {
      Type.Web3Signer -> {
        if (web3signer == null) throw IllegalStateException("Signer $type configuration is null.")
      }

      Type.Web3j -> {
        if (web3j == null) throw IllegalStateException("Signer $type configuration is null.")
      }
    }
  }
}

data class Web3jConfig(
  val privateKey: Masked
)

data class Web3SignerConfig(
  val endpoint: String,
  val maxPoolSize: UInt,
  val keepAlive: Boolean,
  val publicKey: String
)

interface FeatureToggleable {
  val disabled: Boolean
  val enabled: Boolean
    get() = !disabled
}

data class MessageAnchoringServiceConfig(
  val pollingInterval: Duration,
  val maxMessagesToAnchor: UInt,
  override var disabled: Boolean = false
) : FeatureToggleable {
  init {
    require(maxMessagesToAnchor > 0u) { "maxMessagesToAnchor must be greater than 0" }
  }
}

data class L1DynamicGasPriceCapServiceConfig(
  val gasPriceCapCalculation: GasPriceCapCalculation,
  val feeHistoryFetcher: FeeHistoryFetcher,
  val feeHistoryStorage: FeeHistoryStorage,
  override var disabled: Boolean = false
) : FeatureToggleable {
  data class GasPriceCapCalculation(
    val adjustmentConstant: UInt,
    val blobAdjustmentConstant: UInt,
    val finalizationTargetMaxDelay: Duration,
    val gasFeePercentileWindow: Duration,
    val gasFeePercentileWindowLeeway: Duration,
    val gasFeePercentile: Double,
    val gasPriceCapsCheckCoefficient: Double,
    val historicBaseFeePerBlobGasLowerBound: ULong,
    val historicAvgRewardConstant: ULong?,
    val timeOfDayMultipliers: TimeOfDayMultipliers?
  ) {
    init {
      timeOfDayMultipliers?.also {
        val allTimeOfDayKeys = getAllTimeOfDayKeys()
        if (allTimeOfDayKeys != it.keys) {
          val missingKeys = allTimeOfDayKeys - it.keys
          val extraKeys = it.keys - allTimeOfDayKeys
          val errorMessage =
            "Invalid time of day multipliers: missing keys: " +
              "${missingKeys.joinToString(",", "[", "]")}, " +
              "unsupported keys=${extraKeys.joinToString(",", "[", "]")}"
          throw IllegalStateException(errorMessage)
        }

        it.entries.forEach { timeOfDayMultiplier ->
          require(timeOfDayMultiplier.value > 0.0) {
            throw IllegalStateException(
              "Each multiplier in timeOfDayMultipliers must be greater than 0.0." +
                " Key=${timeOfDayMultiplier.key} Value=${timeOfDayMultiplier.value}"
            )
          }
        }

        require(gasFeePercentile in 0.0..100.0) {
          "gasFeePercentile must be within 0.0 and 100.0." +
            " Value=$gasFeePercentile"
        }

        require(gasPriceCapsCheckCoefficient > 0.0) {
          "gasPriceCapsCheckCoefficient must be greater than 0.0." +
            " Value=$gasPriceCapsCheckCoefficient"
        }
      }
    }
  }

  data class FeeHistoryStorage(
    val storagePeriod: Duration
  ) {
    init {
      require(storagePeriod <= MAX_FEE_HISTORIES_STORAGE_PERIOD.toJavaDuration()) {
        "storagePeriod must be at most $MAX_FEE_HISTORIES_STORAGE_PERIOD days"
      }
    }
  }

  data class FeeHistoryFetcher(
    val fetchInterval: Duration,
    val maxBlockCount: UInt,
    val rewardPercentiles: List<Double>,
    val numOfBlocksBeforeLatest: UInt = 4U,
    val endpoint: URL?
  ) {
    init {
      require(
        maxBlockCount > 0U &&
          maxBlockCount <= MAX_FEE_HISTORY_BLOCK_COUNT
      ) {
        "maxBlockCount must be greater than 0 and " +
          "less than or equal to $MAX_FEE_HISTORY_BLOCK_COUNT"
      }

      require(
        rewardPercentiles.isNotEmpty() &&
          rewardPercentiles.size <= MAX_REWARD_PERCENTILES_SIZE
      ) {
        "rewardPercentiles must be a non-empty list with " +
          "maximum length as $MAX_REWARD_PERCENTILES_SIZE."
      }

      require(rewardPercentiles.zipWithNext().all { it.first < it.second }) {
        "rewardPercentiles must contain monotonically-increasing values"
      }

      rewardPercentiles.forEach { percentile ->
        require(percentile in 0.0..100.0) {
          "Each percentile in rewardPercentiles must be within 0.0 and 100.0." +
            " Value=$percentile"
        }
      }
    }
  }

  init {
    require(feeHistoryStorage.storagePeriod >= gasPriceCapCalculation.gasFeePercentileWindow) {
      "storagePeriod must be at least same length as" +
        " gasFeePercentileWindow=${gasPriceCapCalculation.gasFeePercentileWindow}." +
        " Value=${feeHistoryStorage.storagePeriod}"
    }

    require(
      gasPriceCapCalculation.gasFeePercentileWindow
        >= gasPriceCapCalculation.gasFeePercentileWindowLeeway
    ) {
      "gasFeePercentileWindow must be at least same length as" +
        " gasFeePercentileWindowLeeway=${gasPriceCapCalculation.gasFeePercentileWindowLeeway}." +
        " Value=${gasPriceCapCalculation.gasFeePercentileWindow}"
    }

    require(feeHistoryFetcher.rewardPercentiles.contains(gasPriceCapCalculation.gasFeePercentile)) {
      "rewardPercentiles must contain the given" +
        " gasFeePercentile=${gasPriceCapCalculation.gasFeePercentile}." +
        " Value=${feeHistoryFetcher.rewardPercentiles}"
    }
  }
}

data class SmartContractErrorCodesConfig(val smartContractErrors: SmartContractErrors)

data class GasPriceCapTimeOfDayMultipliersConfig(val gasPriceCapTimeOfDayMultipliers: TimeOfDayMultipliers)

data class Type2StateProofProviderConfig(
  val endpoints: List<URL>,
  override val requestRetry: RequestRetryConfigTomlFriendly
) : RequestRetryConfigurable

data class TracesLimitsV1ConfigFile(val tracesLimits: Map<TracingModuleV1, UInt>)
data class TracesLimitsV2ConfigFile(val tracesLimits: Map<TracingModuleV2, UInt>)

//
// CoordinatorConfigTomlDto class to parse from toml
// CoordinatorConfig class with reified configs
// separation between Toml representation and domain representation
// otherwise it's hard to test the configuration is loaded properly
data class CoordinatorConfigTomlDto(
  val l2InclusiveBlockNumberToStopAndFlushAggregation: ULong? = null,
  val zkTraces: ZkTraces,
  val blobCompression: BlobCompressionConfig,
  val proofAggregation: AggregationConfig,
  val traces: TracesConfig,
  val type2StateProofProvider: Type2StateProofProviderConfig,
  val l1: L1Config,
  val l2: L2Config,
  val finalizationSigner: SignerConfig,
  val dataSubmissionSigner: SignerConfig,
  val blobSubmission: BlobSubmissionConfig,
  val aggregationFinalization: AggregationFinalizationConfig,
  val database: DatabaseConfig,
  val persistenceRetry: PersistenceRetryConfig,
  val stateManager: StateManagerClientConfig,
  val conflation: ConflationConfig,
  val api: ApiConfig,
  val l2Signer: SignerConfig,
  val messageAnchoringService: MessageAnchoringServiceConfig,
  val l2NetworkGasPricing: L2NetworkGasPricingTomlDto,
  val l1DynamicGasPriceCapService: L1DynamicGasPriceCapServiceConfig,
  val testL1Disabled: Boolean = false,
  val prover: ProverConfigTomlDto
) {
  fun reified(): CoordinatorConfig = CoordinatorConfig(
    l2InclusiveBlockNumberToStopAndFlushAggregation = l2InclusiveBlockNumberToStopAndFlushAggregation,
    zkTraces = zkTraces,
    blobCompression = blobCompression,
    proofAggregation = proofAggregation,
    traces = traces,
    type2StateProofProvider = type2StateProofProvider,
    l1 = l1,
    l2 = l2,
    finalizationSigner = finalizationSigner,
    dataSubmissionSigner = dataSubmissionSigner,
    blobSubmission = blobSubmission,
    aggregationFinalization = aggregationFinalization,
    database = database,
    persistenceRetry = persistenceRetry,
    stateManager = stateManager,
    conflation = conflation,
    api = api,
    l2Signer = l2Signer,
    messageAnchoringService = messageAnchoringService,
    l2NetworkGasPricingService = if (!testL1Disabled) l2NetworkGasPricing.reified() else null,
    l1DynamicGasPriceCapService = l1DynamicGasPriceCapService,
    testL1Disabled = testL1Disabled,
    proversConfig = prover.reified()
  )
}

data class CoordinatorConfig(
  val l2InclusiveBlockNumberToStopAndFlushAggregation: ULong? = null,
  val zkTraces: ZkTraces,
  val blobCompression: BlobCompressionConfig,
  val proofAggregation: AggregationConfig,
  val traces: TracesConfig,
  val type2StateProofProvider: Type2StateProofProviderConfig,
  val l1: L1Config,
  val l2: L2Config,
  val finalizationSigner: SignerConfig,
  val dataSubmissionSigner: SignerConfig,
  val blobSubmission: BlobSubmissionConfig,
  val aggregationFinalization: AggregationFinalizationConfig,
  val database: DatabaseConfig,
  val persistenceRetry: PersistenceRetryConfig,
  val stateManager: StateManagerClientConfig,
  val conflation: ConflationConfig,
  val api: ApiConfig,
  val l2Signer: SignerConfig,
  val messageAnchoringService: MessageAnchoringServiceConfig,
  val l2NetworkGasPricingService: L2NetworkGasPricingService.Config?,
  val l1DynamicGasPriceCapService: L1DynamicGasPriceCapServiceConfig,
  val testL1Disabled: Boolean = false,
  val proversConfig: ProversConfig
) {
  init {
    if (l2InclusiveBlockNumberToStopAndFlushAggregation != null) {
      require(proofAggregation.targetEndBlocks.contains(l2InclusiveBlockNumberToStopAndFlushAggregation)) {
        "proofAggregation.targetEndBlocks should contain the l2InclusiveBlockNumberToStopAndFlushAggregation"
      }
      require(conflation.conflationTargetEndBlockNumbers.contains(l2InclusiveBlockNumberToStopAndFlushAggregation)) {
        "conflation.conflationTargetEndBlockNumbers should contain the l2InclusiveBlockNumberToStopAndFlushAggregation"
      }
    }

    require(
      blobCompression.batchesLimit == null ||
        blobCompression.batchesLimit!! < proofAggregation.aggregationProofsLimit.toUInt()
    ) {
      "[blob-compression].batchesLimit=${blobCompression.batchesLimit} must be less than " +
        "[proof-aggregation].aggregationProofsLimit=${proofAggregation.aggregationProofsLimit}"
    }

    if (testL1Disabled) {
      messageAnchoringService.disabled = true
      l1DynamicGasPriceCapService.disabled = true
    }
  }
}
