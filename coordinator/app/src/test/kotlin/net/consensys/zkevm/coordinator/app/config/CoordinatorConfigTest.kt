package net.consensys.zkevm.coordinator.app.config

import com.github.michaelbull.result.getError
import com.sksamuel.hoplite.Masked
import linea.coordinator.config.loadConfigs
import linea.coordinator.config.loadConfigsOrError
import linea.domain.BlockParameter
import net.consensys.linea.blob.BlobCompressorVersion
import net.consensys.linea.ethereum.gaspricing.BoundableFeeCalculator
import net.consensys.linea.ethereum.gaspricing.staticcap.ExtraDataV1UpdaterImpl
import net.consensys.linea.ethereum.gaspricing.staticcap.FeeHistoryFetcherImpl
import net.consensys.linea.ethereum.gaspricing.staticcap.GasPriceUpdaterImpl
import net.consensys.linea.ethereum.gaspricing.staticcap.MinerExtraDataV1CalculatorImpl
import net.consensys.linea.ethereum.gaspricing.staticcap.TransactionCostCalculator
import net.consensys.linea.ethereum.gaspricing.staticcap.VariableFeesCalculator
import net.consensys.linea.jsonrpc.client.RequestRetryConfig
import net.consensys.zkevm.coordinator.app.L2NetworkGasPricingService
import net.consensys.zkevm.coordinator.clients.prover.FileBasedProverConfig
import net.consensys.zkevm.coordinator.clients.prover.ProverConfig
import net.consensys.zkevm.coordinator.clients.prover.ProversConfig
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertDoesNotThrow
import org.junit.jupiter.api.assertThrows
import java.math.BigInteger
import java.net.URI
import java.nio.file.Path
import java.nio.file.Paths
import java.time.Duration
import kotlin.time.Duration.Companion.minutes
import kotlin.time.Duration.Companion.seconds

class CoordinatorConfigTest {
  companion object {
    private val apiConfig = ApiConfig(9545U)
    private val conflationConfig = ConflationConfig(
      consistentNumberOfBlocksOnL1ToWait = 1,
      conflationDeadline = Duration.parse("PT6S"),
      conflationDeadlineCheckInterval = Duration.parse("PT3S"),
      conflationDeadlineLastBlockConfirmationDelay = Duration.parse("PT2S"),
      blocksLimit = 2,
      _tracesLimitsV1 = expectedTracesCountersV1,
      _tracesLimitsV2 = expectedTracesLimitsV2,
      _smartContractErrors = mapOf(
        // L1 Linea Rollup
        "0f06cd15" to "DataAlreadySubmitted",
        "c01eab56" to "EmptySubmissionData"
      ),
      fetchBlocksLimit = 4000
    )

    private val proversConfig = ProversConfig(
      proverA = ProverConfig(
        execution = FileBasedProverConfig(
          requestsDirectory = Path.of("/data/prover/v2/execution/requests"),
          responsesDirectory = Path.of("/data/prover/v2/execution/responses"),
          pollingInterval = 1.seconds,
          pollingTimeout = 10.minutes,
          inprogressProvingSuffixPattern = ".*\\.inprogress\\.prover.*",
          inprogressRequestWritingSuffix = ".inprogress_coordinator_writing"
        ),
        blobCompression = FileBasedProverConfig(
          requestsDirectory = Path.of("/data/prover/v2/compression/requests"),
          responsesDirectory = Path.of("/data/prover/v2/compression/responses"),
          pollingInterval = 1.seconds,
          pollingTimeout = 10.minutes,
          inprogressProvingSuffixPattern = ".*\\.inprogress\\.prover.*",
          inprogressRequestWritingSuffix = ".inprogress_coordinator_writing"
        ),
        proofAggregation = FileBasedProverConfig(
          requestsDirectory = Path.of("/data/prover/v2/aggregation/requests"),
          responsesDirectory = Path.of("/data/prover/v2/aggregation/responses"),
          pollingInterval = 1.seconds,
          pollingTimeout = 10.minutes,
          inprogressProvingSuffixPattern = ".*\\.inprogress\\.prover.*",
          inprogressRequestWritingSuffix = ".inprogress_coordinator_writing"
        )
      ),
      switchBlockNumberInclusive = null,
      proverB = null
    )

    private val blobCompressionConfig = BlobCompressionConfig(
      blobSizeLimit = 100 * 1024,
      handlerPollingInterval = Duration.parse("PT1S"),
      _batchesLimit = 1
    )

    private val aggregationConfig = AggregationConfig(
      aggregationProofsLimit = 3,
      aggregationDeadline = Duration.parse("PT10S"),
      aggregationCoordinatorPollingInterval = Duration.parse("PT2S"),
      deadlineCheckInterval = Duration.parse("PT8S")
    )

    private val tracesConfig = TracesConfig(
      switchToLineaBesu = false,
      blobCompressorVersion = BlobCompressorVersion.V0_1_0,
      rawExecutionTracesVersion = "0.2.0",
      expectedTracesApiVersion = "0.2.0",
      counters = TracesConfig.FunctionalityEndpoint(
        listOf(
          URI("http://traces-api:8080/").toURL()
        ),
        requestLimitPerEndpoint = 2U,
        requestRetry = RequestRetryConfigTomlFriendly(
          backoffDelay = Duration.parse("PT1S"),
          failuresWarningThreshold = 2
        )
      ),
      conflation = TracesConfig.FunctionalityEndpoint(
        endpoints = listOf(
          URI("http://traces-api:8080/").toURL()
        ),
        requestLimitPerEndpoint = 2U,
        requestRetry = RequestRetryConfigTomlFriendly(
          backoffDelay = Duration.parse("PT1S"),
          failuresWarningThreshold = 2
        )
      ),
      fileManager = TracesConfig.FileManager(
        tracesFileExtension = "json.gz",
        rawTracesDirectory = Path.of("/data/traces/raw"),
        nonCanonicalRawTracesDirectory = Path.of("/data/traces/raw-non-canonical"),
        createNonCanonicalDirectory = true,
        pollingInterval = Duration.parse("PT1S"),
        tracesFileCreationWaitTimeout = Duration.parse("PT2M")
      )
    )

    private val type2StateProofProviderConfig = Type2StateProofProviderConfig(
      endpoints = listOf(URI("http://shomei-frontend:8888/").toURL()),
      requestRetry = RequestRetryConfigTomlFriendly(
        backoffDelay = Duration.parse("PT1S"),
        failuresWarningThreshold = 2
      )
    )
    private val stateManagerConfig = StateManagerClientConfig(
      version = "2.3.0",
      endpoints = listOf(
        URI("http://shomei:8888/").toURL()
      ),
      requestLimitPerEndpoint = 2U,
      requestRetry = RequestRetryConfigTomlFriendly(
        backoffDelay = Duration.parse("PT2S"),
        failuresWarningThreshold = 2
      )
    )

    private val blobSubmissionConfig = BlobSubmissionConfig(
      dbPollingInterval = Duration.parse("PT1S"),
      maxBlobsToReturn = 100,
      maxBlobsToSubmitPerTick = 10,
      priorityFeePerGasUpperBound = 2000000000UL,
      priorityFeePerGasLowerBound = 200000000UL,
      proofSubmissionDelay = Duration.parse("PT1S"),
      targetBlobsToSendPerTransaction = 6,
      useEthEstimateGas = false,
      disabled = false
    )

    private val aggregationFinalizationConfig = AggregationFinalizationConfig(
      dbPollingInterval = Duration.parse("PT1S"),
      maxAggregationsToFinalizePerTick = 1,
      proofSubmissionDelay = Duration.parse("PT1S"),
      useEthEstimateGas = true,
      disabled = false
    )

    private val databaseConfig = DatabaseConfig(
      host = "postgres",
      port = 5432,
      username = "postgres",
      password = Masked("postgres"),
      schema = "linea_coordinator",
      readPoolSize = 10,
      readPipeliningLimit = 10,
      transactionalPoolSize = 10
    )

    private val persistenceRetryConfig = PersistenceRetryConfig(
      maxRetries = null,
      backoffDelay = Duration.parse("PT1S")
    )

    private val l1Config = L1Config(
      zkEvmContractAddress = "0xCf7Ed3AccA5a467e9e704C703E8D87F634fB0Fc9",
      rpcEndpoint = URI("http://l1-el-node:8545").toURL(),
      finalizationPollingInterval = Duration.parse("PT6S"),
      _l1QueryBlockTag = BlockParameter.Tag.LATEST.getTag(),
      gasLimit = 10000000UL,
      feeHistoryBlockCount = 10,
      feeHistoryRewardPercentile = 15.0,
      maxFeePerGasCap = 100000000000UL,
      maxFeePerBlobGasCap = 100000000000UL,
      maxPriorityFeePerGasCap = 20000000000UL,
      gasPriceCapMultiplierForFinalization = 2.0,
      earliestBlock = BigInteger.ZERO,
      sendMessageEventPollingInterval = Duration.parse("PT1S"),
      maxEventScrapingTime = Duration.parse("PT5S"),
      maxMessagesToCollect = 1000U,
      finalizedBlockTag = "latest",
      blockTime = Duration.parse("PT1S"),
      blockRangeLoopLimit = 500U,
      _ethFeeHistoryEndpoint = null,
      _genesisStateRootHash = "0x072ead6777750dc20232d1cee8dc9a395c2d350df4bbaa5096c6f59b214dcecd",
      _genesisShnarfV6 = "0x47452a1b9ebadfe02bdd02f580fa1eba17680d57eec968a591644d05d78ee84f"
    )

    private val l2Config = L2Config(
      messageServiceAddress = "0xe537D669CA013d86EBeF1D64e40fC74CADC91987",
      rpcEndpoint = URI("http://sequencer:8545").toURL(),
      gasLimit = 10000000u,
      maxFeePerGasCap = 100000000000u,
      feeHistoryBlockCount = 4U,
      feeHistoryRewardPercentile = 15.0,
      blocksToFinalization = 0U,
      lastHashSearchWindow = 25U,
      anchoringReceiptPollingInterval = Duration.parse("PT01S"),
      maxReceiptRetries = 120U,
      newBlockPollingInterval = Duration.parse("PT1S")
    )

    private val finalizationSigner = SignerConfig(
      type = SignerConfig.Type.Web3j,
      web3signer = Web3SignerConfig(
        endpoint = "http://web3signer:9000",
        maxPoolSize = 10U,
        keepAlive = true,
        publicKey =
        "ba5734d8f7091719471e7f7ed6b9df170dc70cc661ca05e688601ad984f068b0d67351e5f06073092499336ab0839ef8a521afd334e5" +
          "3807205fa2f08eec74f4"
      ),
      web3j = Web3jConfig(Masked("0x59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d"))
    )

    private val dataSubmissionSigner = SignerConfig(
      type = SignerConfig.Type.Web3j,
      web3signer = Web3SignerConfig(
        endpoint = "http://web3signer:9000",
        maxPoolSize = 10U,
        keepAlive = true,
        publicKey =
        "9d9031e97dd78ff8c15aa86939de9b1e791066a0224e331bc962a2099a7b1f0464b8bbafe1535f2301c72c2cb3535b172da30b02686a" +
          "b0393d348614f157fbdb"
      ),
      web3j = Web3jConfig(Masked("0x5de4111afa1a4b94908f83103eb1f1706367c2e68ca870fc3fb9a804cdab365a"))
    )
    private val l2SignerConfig = SignerConfig(
      type = SignerConfig.Type.Web3j,
      web3signer = Web3SignerConfig(
        endpoint = "http://web3signer:9000",
        maxPoolSize = 10U,
        keepAlive = true,
        publicKey =
        "4a788ad6fa008beed58de6418369717d7492f37d173d70e2c26d9737e2c6eeae929452ef8602a19410844db3e200a0e73f5208fd7625" +
          "9a8766b73953fc3e7023"
      ),
      web3j = Web3jConfig(Masked("0x4d01ae6487860981699236a58b68f807ee5f17b12df5740b85cf4c4653be0f55"))
    )

    private val messageAnchoringServiceConfig = MessageAnchoringServiceConfig(
      pollingInterval = Duration.parse("PT1S"),
      maxMessagesToAnchor = 100U
    )

    private val l2NetworkGasPricingRequestRetryConfig = RequestRetryConfig(
      maxRetries = 3u,
      timeout = 6.seconds,
      backoffDelay = 1.seconds,
      failuresWarningThreshold = 2u
    )

    private val l2NetworkGasPricingServiceConfig = L2NetworkGasPricingService.Config(
      feeHistoryFetcherConfig = FeeHistoryFetcherImpl.Config(
        feeHistoryBlockCount = 50U,
        feeHistoryRewardPercentile = 15.0
      ),
      legacy = L2NetworkGasPricingService.LegacyGasPricingCalculatorConfig(
        legacyGasPricingCalculatorBounds = BoundableFeeCalculator.Config(
          feeUpperBound = 10_000_000_000.0,
          feeLowerBound = 90_000_000.0,
          feeMargin = 0.0
        ),
        transactionCostCalculatorConfig = TransactionCostCalculator.Config(
          sampleTransactionCostMultiplier = 1.0,
          fixedCostWei = 3000000u,
          compressedTxSize = 125,
          expectedGas = 21000
        ),
        naiveGasPricingCalculatorConfig = null
      ),
      jsonRpcGasPriceUpdaterConfig = GasPriceUpdaterImpl.Config(
        gethEndpoints = listOf(
          URI("http://l2-node:8545/").toURL()
        ),
        besuEndPoints = listOf(),
        retryConfig = l2NetworkGasPricingRequestRetryConfig
      ),
      jsonRpcPriceUpdateInterval = 12.seconds,
      extraDataPricingPropagationEnabled = true,
      extraDataUpdateInterval = 12.seconds,
      variableFeesCalculatorConfig = VariableFeesCalculator.Config(
        blobSubmissionExpectedExecutionGas = 213_000u,
        bytesPerDataSubmission = 131072u,
        expectedBlobGas = 131072u,
        margin = 4.0
      ),
      variableFeesCalculatorBounds = BoundableFeeCalculator.Config(
        feeUpperBound = 10_000_000_001.0,
        feeLowerBound = 90_000_001.0,
        feeMargin = 0.0
      ),
      extraDataCalculatorConfig = MinerExtraDataV1CalculatorImpl.Config(
        fixedCostInKWei = 3000u,
        ethGasPriceMultiplier = 1.2
      ),
      extraDataUpdaterConfig = ExtraDataV1UpdaterImpl.Config(
        sequencerEndpoint = URI(/* str = */ "http://sequencer:8545/").toURL(),
        retryConfig = l2NetworkGasPricingRequestRetryConfig
      )
    )

    private val l1DynamicGasPriceCapServiceConfig = L1DynamicGasPriceCapServiceConfig(
      disabled = false,
      gasPriceCapCalculation = L1DynamicGasPriceCapServiceConfig.GasPriceCapCalculation(
        adjustmentConstant = 25U,
        blobAdjustmentConstant = 25U,
        finalizationTargetMaxDelay = Duration.parse("PT30S"),
        gasFeePercentileWindow = Duration.parse("PT1M"),
        gasFeePercentileWindowLeeway = Duration.parse("PT10S"),
        gasFeePercentile = 10.0,
        gasPriceCapsCheckCoefficient = 0.9,
        historicBaseFeePerBlobGasLowerBound = 100_000_000u,
        historicAvgRewardConstant = 100_000_000u,
        timeOfDayMultipliers = expectedTimeOfDayMultipliers
      ),
      feeHistoryFetcher = L1DynamicGasPriceCapServiceConfig.FeeHistoryFetcher(
        fetchInterval = Duration.parse("PT1S"),
        maxBlockCount = 1000U,
        rewardPercentiles = listOf(10.0, 20.0, 30.0, 40.0, 50.0, 60.0, 70.0, 80.0, 90.0, 100.0),
        numOfBlocksBeforeLatest = 4U,
        endpoint = null
      ),
      feeHistoryStorage = L1DynamicGasPriceCapServiceConfig.FeeHistoryStorage(
        storagePeriod = Duration.parse("PT2M")
      )
    )

    private val coordinatorConfig = CoordinatorConfig(
      blobCompression = blobCompressionConfig,
      proofAggregation = aggregationConfig,
      traces = tracesConfig,
      type2StateProofProvider = type2StateProofProviderConfig,
      l1 = l1Config,
      l2 = l2Config,
      finalizationSigner = finalizationSigner,
      dataSubmissionSigner = dataSubmissionSigner,
      blobSubmission = blobSubmissionConfig,
      aggregationFinalization = aggregationFinalizationConfig,
      database = databaseConfig,
      persistenceRetry = persistenceRetryConfig,
      stateManager = stateManagerConfig,
      conflation = conflationConfig,
      api = apiConfig,
      l2Signer = l2SignerConfig,
      messageAnchoringService = messageAnchoringServiceConfig,
      l2NetworkGasPricingService = l2NetworkGasPricingServiceConfig,
      l1DynamicGasPriceCapService = l1DynamicGasPriceCapServiceConfig,
      proversConfig = proversConfig
    )
  }

  private data class TestConfig(val extraField: String)

  @Test
  fun `should keep local stack testing configs uptodate with the code`() {
    // Just assert that Files have been loaded and parsed correctly
    // This is to prevent Code changes in coordinator and forgetting to update config files used in the local stack
    loadConfigs(
      coordinatorConfigFiles = listOf(
        Path.of("../../config/coordinator/coordinator-docker.config.toml"),
        Path.of("../../config/coordinator/coordinator-docker-traces-v2-override.config.toml"),
        Path.of("../../config/coordinator/coordinator-docker-web3signer-override.config.toml"),
        Path.of("../../config/coordinator/coordinator-local-dev.config.overrides.toml"),
        Path.of("../../config/coordinator/coordinator-local-dev.config-traces-v2.overrides.toml")
      ),
      tracesLimitsFileV1 = Path.of("../../config/common/traces-limits-v1.toml"),
      tracesLimitsFileV2 = Path.of("../../config/common/traces-limits-v2.toml"),
      gasPriceCapTimeOfDayMultipliersFile = Path.of("../../config/common/gas-price-cap-time-of-day-multipliers.toml"),
      smartContractErrorsFile = Path.of("../../config/common/smart-contract-errors.toml")
    )
  }

  private fun pathToResource(resource: String): Path {
    return Paths.get(
      this::class.java.classLoader.getResource(resource)?.toURI()
        ?: error("Resource not found: $resource")
    )
  }

  @Test
  fun `should parse and consolidate configs`() {
    val configs = loadConfigs(
      coordinatorConfigFiles = listOf(pathToResource("configs/coordinator.config.toml")),
      tracesLimitsFileV1 = pathToResource("configs/traces-limits-v1.toml"),
      tracesLimitsFileV2 = pathToResource("configs/traces-limits-v2.toml"),
      gasPriceCapTimeOfDayMultipliersFile = pathToResource("configs/gas-price-cap-time-of-day-multipliers.toml"),
      smartContractErrorsFile = pathToResource("configs/smart-contract-errors.toml")
    )

    assertEquals(coordinatorConfig, configs)
    assertEquals(coordinatorConfig.l1.rpcEndpoint, coordinatorConfig.l1.ethFeeHistoryEndpoint)
  }

  @Test
  fun parsesValidWeb3SignerConfigOverride() {
    val config = loadConfigs(
      coordinatorConfigFiles = listOf(
        pathToResource("configs/coordinator.config.toml"),
        pathToResource("configs/coordinator-web3signer-override.config.toml")
      ),
      tracesLimitsFileV1 = pathToResource("configs/traces-limits-v1.toml"),
      tracesLimitsFileV2 = pathToResource("configs/traces-limits-v2.toml"),
      gasPriceCapTimeOfDayMultipliersFile = pathToResource("configs/gas-price-cap-time-of-day-multipliers.toml"),
      smartContractErrorsFile = pathToResource("configs/smart-contract-errors.toml")
    )

    val expectedConfig =
      coordinatorConfig.copy(
        finalizationSigner = finalizationSigner.copy(type = SignerConfig.Type.Web3Signer),
        dataSubmissionSigner = dataSubmissionSigner.copy(type = SignerConfig.Type.Web3Signer),
        l2Signer = l2SignerConfig.copy(type = SignerConfig.Type.Web3Signer)
      )

    assertThat(config).isEqualTo(expectedConfig)
  }

  @Test
  fun parsesValidTracesV2ConfigOverride() {
    val config = loadConfigs(
      coordinatorConfigFiles = listOf(
        pathToResource("configs/coordinator.config.toml"),
        pathToResource("configs/coordinator-traces-v2-override.config.toml")
      ),
      tracesLimitsFileV1 = pathToResource("configs/traces-limits-v1.toml"),
      tracesLimitsFileV2 = pathToResource("configs/traces-limits-v2.toml"),
      gasPriceCapTimeOfDayMultipliersFile = pathToResource("configs/gas-price-cap-time-of-day-multipliers.toml"),
      smartContractErrorsFile = pathToResource("configs/smart-contract-errors.toml")
    )

    val expectedConfig =
      coordinatorConfig.copy(
        l2NetworkGasPricingService = l2NetworkGasPricingServiceConfig.copy(
          legacy =
          l2NetworkGasPricingServiceConfig.legacy.copy(
            transactionCostCalculatorConfig =
            l2NetworkGasPricingServiceConfig.legacy.transactionCostCalculatorConfig?.copy(
              compressedTxSize = 350,
              expectedGas = 29400
            )
          )
        ),
        traces = tracesConfig.copy(
          switchToLineaBesu = true,
          blobCompressorVersion = BlobCompressorVersion.V1_0_1,
          expectedTracesApiVersionV2 = "v0.8.0-rc8",
          conflationV2 = tracesConfig.conflation.copy(
            endpoints = listOf(URI("http://traces-node-v2:8545/").toURL()),
            requestLimitPerEndpoint = 1U
          ),
          countersV2 = TracesConfig.FunctionalityEndpoint(
            listOf(
              URI("http://traces-node-v2:8545/").toURL()
            ),
            requestLimitPerEndpoint = 1U,
            requestRetry = RequestRetryConfigTomlFriendly(
              backoffDelay = Duration.parse("PT1S"),
              failuresWarningThreshold = 2
            )
          )
        ),
        proversConfig = proversConfig.copy(
          proverA = proversConfig.proverA.copy(
            execution = proversConfig.proverA.execution.copy(
              requestsDirectory = Path.of("/data/prover/v3/execution/requests"),
              responsesDirectory = Path.of("/data/prover/v3/execution/responses")
            ),
            blobCompression = proversConfig.proverA.blobCompression.copy(
              requestsDirectory = Path.of("/data/prover/v3/compression/requests"),
              responsesDirectory = Path.of("/data/prover/v3/compression/responses")
            ),
            proofAggregation = proversConfig.proverA.proofAggregation.copy(
              requestsDirectory = Path.of("/data/prover/v3/aggregation/requests"),
              responsesDirectory = Path.of("/data/prover/v3/aggregation/responses")
            )
          )
        )
      )

    assertThat(config).isEqualTo(expectedConfig)
  }

  @Test
  fun invalidConfigReturnsErrorResult() {
    val configsResult = loadConfigsOrError<TestConfig>(
      configFiles = listOf(pathToResource("configs/coordinator.config.toml"))
    )

    assertThat(configsResult.getError()).contains("'extraField': Missing from config")
  }

  @Test
  fun testInvalidAggregationByTargetBlockNumberWhenL2InclusiveBlockNumberToStopAndFlushAggregationSpecified() {
    val aggregationConfigWithoutTargetBlockNumber = aggregationConfig.copy(
      _targetEndBlocks = emptyList()
    )
    val conflationConfigWithTargetBlockNumber = conflationConfig.copy(
      _conflationTargetEndBlockNumbers = listOf(100L)
    )

    val exception = assertThrows<IllegalArgumentException> {
      coordinatorConfig.copy(
        l2InclusiveBlockNumberToStopAndFlushAggregation = 100uL,
        proofAggregation = aggregationConfigWithoutTargetBlockNumber,
        conflation = conflationConfigWithTargetBlockNumber
      )
    }
    assertThat(exception.message)
      .isEqualTo("proofAggregation.targetEndBlocks should contain the l2InclusiveBlockNumberToStopAndFlushAggregation")
  }

  @Test
  fun testInvalidConflationByTargetBlockNumberWhenL2InclusiveBlockNumberToStopAndFlushAggregationSpecified() {
    val aggregationConfigWithTargetBlockNumber = aggregationConfig.copy(
      _targetEndBlocks = listOf(100L)
    )
    val conflationConfigWithoutTargetBlockNumber = conflationConfig.copy(
      _conflationTargetEndBlockNumbers = emptyList()
    )

    val exception = assertThrows<IllegalArgumentException> {
      coordinatorConfig.copy(
        l2InclusiveBlockNumberToStopAndFlushAggregation = 100uL,
        proofAggregation = aggregationConfigWithTargetBlockNumber,
        conflation = conflationConfigWithoutTargetBlockNumber
      )
    }
    assertThat(exception.message)
      .isEqualTo(
        "conflation.conflationTargetEndBlockNumbers should contain the " +
          "l2InclusiveBlockNumberToStopAndFlushAggregation"
      )
  }

  @Test
  fun testValidAggrAndConflationByTargetBlockNumberWhenL2InclusiveBlockNumberToStopAndFlushAggregationSpecified() {
    val aggregationConfigWithoutSwithBlockNumber = aggregationConfig.copy(
      _targetEndBlocks = listOf(10L, 100L)
    )
    val conflationConfigWithTargetBlockNumber = conflationConfig.copy(
      _conflationTargetEndBlockNumbers = listOf(100L)
    )

    assertDoesNotThrow {
      coordinatorConfig.copy(
        l2InclusiveBlockNumberToStopAndFlushAggregation = 100uL,
        proofAggregation = aggregationConfigWithoutSwithBlockNumber,
        conflation = conflationConfigWithTargetBlockNumber
      )
    }
  }
}
