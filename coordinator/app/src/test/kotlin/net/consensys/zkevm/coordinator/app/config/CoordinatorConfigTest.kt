package net.consensys.zkevm.coordinator.app.config

import com.github.michaelbull.result.get
import com.github.michaelbull.result.getError
import com.github.michaelbull.result.onFailure
import com.github.michaelbull.result.onSuccess
import com.sksamuel.hoplite.Masked
import net.consensys.linea.BlockParameter
import net.consensys.linea.blob.BlobCompressorVersion
import net.consensys.linea.ethereum.gaspricing.BoundableFeeCalculator
import net.consensys.linea.ethereum.gaspricing.staticcap.ExtraDataV1UpdaterImpl
import net.consensys.linea.ethereum.gaspricing.staticcap.FeeHistoryFetcherImpl
import net.consensys.linea.ethereum.gaspricing.staticcap.GasPriceUpdaterImpl
import net.consensys.linea.ethereum.gaspricing.staticcap.MinerExtraDataV1CalculatorImpl
import net.consensys.linea.ethereum.gaspricing.staticcap.TransactionCostCalculator
import net.consensys.linea.ethereum.gaspricing.staticcap.VariableFeesCalculator
import net.consensys.linea.jsonrpc.client.RequestRetryConfig
import net.consensys.linea.traces.TracesCountersV1
import net.consensys.linea.traces.TracesCountersV2
import net.consensys.linea.traces.TracingModuleV1
import net.consensys.linea.traces.TracingModuleV2
import net.consensys.linea.web3j.SmartContractErrors
import net.consensys.zkevm.coordinator.app.CoordinatorAppCli
import net.consensys.zkevm.coordinator.app.L2NetworkGasPricingService
import net.consensys.zkevm.coordinator.clients.prover.FileBasedProverConfig
import net.consensys.zkevm.coordinator.clients.prover.ProverConfig
import net.consensys.zkevm.coordinator.clients.prover.ProversConfig
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Assertions.fail
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertDoesNotThrow
import org.junit.jupiter.api.assertThrows
import java.io.File
import java.math.BigInteger
import java.net.URI
import java.nio.file.Path
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
      _tracesLimitsV1 = TracesCountersV1(
        mapOf(
          TracingModuleV1.ADD to 524288U,
          TracingModuleV1.BIN to 262144U,
          TracingModuleV1.BIN_RT to 262144U,
          TracingModuleV1.EC_DATA to 4096U,
          TracingModuleV1.EXT to 131072U,
          TracingModuleV1.HUB to 2097152U,
          TracingModuleV1.INSTRUCTION_DECODER to 512U,
          TracingModuleV1.MMIO to 131072U,
          TracingModuleV1.MMU to 131072U,
          TracingModuleV1.MMU_ID to 131072U,
          TracingModuleV1.MOD to 131072U,
          TracingModuleV1.MUL to 65536U,
          TracingModuleV1.MXP to 524288U,
          TracingModuleV1.PHONEY_RLP to 32768U,
          TracingModuleV1.PUB_HASH to 32768U,
          TracingModuleV1.PUB_HASH_INFO to 32768U,
          TracingModuleV1.PUB_LOG to 16384U,
          TracingModuleV1.PUB_LOG_INFO to 16384U,
          TracingModuleV1.RLP to 512U,
          TracingModuleV1.ROM to 4194304U,
          TracingModuleV1.SHF to 65536U,
          TracingModuleV1.SHF_RT to 4096U,
          TracingModuleV1.TX_RLP to 131072U,
          TracingModuleV1.WCP to 262144U,
          TracingModuleV1.BLOCK_TX to 200U,
          TracingModuleV1.BLOCK_L2L1LOGS to 16U,
          TracingModuleV1.BLOCK_KECCAK to 8192U,
          TracingModuleV1.PRECOMPILE_ECRECOVER to 10000U,
          TracingModuleV1.PRECOMPILE_SHA2 to 10000U,
          TracingModuleV1.PRECOMPILE_RIPEMD to 10000U,
          TracingModuleV1.PRECOMPILE_IDENTITY to 10000U,
          TracingModuleV1.PRECOMPILE_MODEXP to 10000U,
          TracingModuleV1.PRECOMPILE_ECADD to 10000U,
          TracingModuleV1.PRECOMPILE_ECMUL to 10000U,
          TracingModuleV1.PRECOMPILE_ECPAIRING to 10000U,
          TracingModuleV1.PRECOMPILE_BLAKE2F to 512U
        )
      ),
      _tracesLimitsV2 = TracesCountersV2(
        mapOf(
          TracingModuleV2.ADD to 524288u,
          TracingModuleV2.BIN to 262144u,
          TracingModuleV2.BLAKE_MODEXP_DATA to 16384u,
          TracingModuleV2.BLOCK_DATA to 1024u,
          TracingModuleV2.BLOCK_HASH to 512u,
          TracingModuleV2.EC_DATA to 262144u,
          TracingModuleV2.EUC to 65536u,
          TracingModuleV2.EXP to 8192u,
          TracingModuleV2.EXT to 1048576u,
          TracingModuleV2.GAS to 65536u,
          TracingModuleV2.HUB to 2097152u,
          TracingModuleV2.LOG_DATA to 65536u,
          TracingModuleV2.LOG_INFO to 4096u,
          TracingModuleV2.MMIO to 4194304u,
          TracingModuleV2.MMU to 4194304u,
          TracingModuleV2.MOD to 131072u,
          TracingModuleV2.MUL to 65536u,
          TracingModuleV2.MXP to 524288u,
          TracingModuleV2.OOB to 262144u,
          TracingModuleV2.RLP_ADDR to 4096u,
          TracingModuleV2.RLP_TXN to 131072u,
          TracingModuleV2.RLP_TXN_RCPT to 65536u,
          TracingModuleV2.ROM to 4194304u,
          TracingModuleV2.ROM_LEX to 1024u,
          TracingModuleV2.SHAKIRA_DATA to 32768u,
          TracingModuleV2.SHF to 65536u,
          TracingModuleV2.STP to 16384u,
          TracingModuleV2.TRM to 32768u,
          TracingModuleV2.TXN_DATA to 8192u,
          TracingModuleV2.WCP to 262144u,
          TracingModuleV2.BIN_REFERENCE_TABLE to 4294967295u,
          TracingModuleV2.SHF_REFERENCE_TABLE to 4294967295u,
          TracingModuleV2.INSTRUCTION_DECODER to 4294967295u,
          TracingModuleV2.PRECOMPILE_ECRECOVER_EFFECTIVE_CALLS to 128u,
          TracingModuleV2.PRECOMPILE_SHA2_BLOCKS to 671u,
          TracingModuleV2.PRECOMPILE_RIPEMD_BLOCKS to 671u,
          TracingModuleV2.PRECOMPILE_MODEXP_EFFECTIVE_CALLS to 4u,
          TracingModuleV2.PRECOMPILE_ECADD_EFFECTIVE_CALLS to 16384u,
          TracingModuleV2.PRECOMPILE_ECMUL_EFFECTIVE_CALLS to 32u,
          TracingModuleV2.PRECOMPILE_ECPAIRING_FINAL_EXPONENTIATIONS to 16u,
          TracingModuleV2.PRECOMPILE_ECPAIRING_G2_MEMBERSHIP_CALLS to 64u,
          TracingModuleV2.PRECOMPILE_ECPAIRING_MILLER_LOOPS to 64u,
          TracingModuleV2.PRECOMPILE_BLAKE_EFFECTIVE_CALLS to 600u,
          TracingModuleV2.PRECOMPILE_BLAKE_ROUNDS to 600u,
          TracingModuleV2.BLOCK_KECCAK to 8192u,
          TracingModuleV2.BLOCK_L1_SIZE to 1000000u,
          TracingModuleV2.BLOCK_L2_L1_LOGS to 16u,
          TracingModuleV2.BLOCK_TRANSACTIONS to 200u
        )
      ),
      _smartContractErrors = mapOf(
        // L1 Linea Rollup
        "0f06cd15" to "DataAlreadySubmitted",
        "c01eab56" to "EmptySubmissionData",
        "abefa5e8" to "DataStartingBlockDoesNotMatch",
        "5548c6b3" to "DataParentHasEmptyShnarf",
        "36459fa0" to "L1RollingHashDoesNotExistOnL1",
        "cbbd7953" to "FirstBlockGreaterThanFinalBlock",
        "a386ed70" to "FirstBlockLessThanOrEqualToLastFinalizedBlock",
        "70614405" to "FinalBlockNumberLessThanOrEqualToLastFinalizedBlock",
        "2898482a" to "FinalBlockStateEqualsZeroHash",
        "bf81c6e0" to "FinalizationInTheFuture",
        "0c256592" to "MissingMessageNumberForRollingHash",
        "5228f4c8" to "MissingRollingHashForMessageNumber",
        "729eebce" to "FirstByteIsNotZero",
        "6426c6c5" to "BytesLengthNotMultipleOf32",
        "68dcad5f" to "PointEvaluationResponseInvalid",
        "f75db381" to "PrecompileReturnDataLengthWrong",
        "a71194af" to "PointEvaluationFailed",
        "2f22b98a" to "LastFinalizedShnarfWrong",
        "7dc2487d" to "SnarkHashIsZeroHash",
        "bc5aad11" to "FinalizationStateIncorrect",
        "b1504a5f" to "BlobSubmissionDataIsMissing",
        "c0e41e1d" to "EmptyBlobDataAtIndex",
        "fb4cd6ef" to "FinalBlockDoesNotMatchShnarfFinalBlock",
        "2526F108" to "ShnarfAndFinalBlockNumberLengthsMismatched",
        "d3664fb3" to "FinalShnarfWrong",
        "4e686675" to "L2MerkleRootDoesNotExist",
        "e5d14425" to "L2MerkleRootAlreadyAnchored",
        "0c91d776" to "BytesLengthNotMultipleOfTwo",
        "ead4c30e" to "StartingRootHashDoesNotMatch",
        "7907d79b" to "ProofIsEmpty",
        "69ed70ab" to "InvalidProofType",
        "09bde339" to "InvalidProof",
        "db246dde" to "IsPaused",
        "b015579f" to "IsNotPaused",
        "3b174434" to "MessageHashesListLengthHigherThanOneHundred",
        "ca389c44" to "InvalidProofOrProofVerificationRanOutOfGas",
        "42ab979d" to "ParentBlobNotSubmitted",
        "edeae83c" to "FinalBlobNotSubmitted",
        // L2 Message Service
        "6446cc9c" to "MessageHashesListLengthIsZero",
        "d39e75f9" to "L1MessageNumberSynchronizationWrong",
        "7557a60a" to "L1RollingHashSynchronizationWrong",
        "36a4bb94" to "FinalRollingHashIsZero"
      ),
      fetchBlocksLimit = 4000
    )

    private val zkTracesConfig = ZkTraces(
      URI("http://traces-node:8545").toURL(),
      Duration.parse("PT1S")
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
      disabled = true
    )

    private val aggregationFinalizationConfig = AggregationFinalizationConfig(
      dbPollingInterval = Duration.parse("PT1S"),
      maxAggregationsToFinalizePerTick = 1,
      proofSubmissionDelay = Duration.parse("PT1S"),
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
      _genesisShnarfV5 = "0x47452a1b9ebadfe02bdd02f580fa1eba17680d57eec968a591644d05d78ee84f"
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
      maxReceiptRetries = 120U
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
      jsonRpcPricingPropagationEnabled = true,
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
        timeOfDayMultipliers = mapOf(
          "SUNDAY_0" to 1.7489178377946066,
          "SUNDAY_1" to 1.7494632175198737,
          "SUNDAY_2" to 1.75,
          "SUNDAY_3" to 1.733166295438555,
          "SUNDAY_4" to 1.6993775444542885,
          "SUNDAY_5" to 1.6350086618091364,
          "SUNDAY_6" to 1.5627740860151331,
          "SUNDAY_7" to 1.4831149222064164,
          "SUNDAY_8" to 1.4101476768256929,
          "SUNDAY_9" to 1.370085278922007,
          "SUNDAY_10" to 1.3516015544068651,
          "SUNDAY_11" to 1.3482404546676368,
          "SUNDAY_12" to 1.3580905751578942,
          "SUNDAY_13" to 1.3775497419563296,
          "SUNDAY_14" to 1.3700255667542938,
          "SUNDAY_15" to 1.2642948506461285,
          "SUNDAY_16" to 1.2794806131912935,
          "SUNDAY_17" to 1.2750892256476676,
          "SUNDAY_18" to 1.2919720208955585,
          "SUNDAY_19" to 1.317984990098603,
          "SUNDAY_20" to 1.4433501639513178,
          "SUNDAY_21" to 1.4705921238901998,
          "SUNDAY_22" to 1.515043370430801,
          "SUNDAY_23" to 1.5556742617266397,
          "MONDAY_0" to 1.5381562278760164,
          "MONDAY_1" to 1.5423761828433993,
          "MONDAY_2" to 1.539015963719092,
          "MONDAY_3" to 1.487676153648977,
          "MONDAY_4" to 1.430973985132037,
          "MONDAY_5" to 1.4656765439056292,
          "MONDAY_6" to 1.4484298622828233,
          "MONDAY_7" to 1.4459076216659752,
          "MONDAY_8" to 1.4899061835032241,
          "MONDAY_9" to 1.5249733712852067,
          "MONDAY_10" to 1.511367489481033,
          "MONDAY_11" to 1.4225695658047797,
          "MONDAY_12" to 1.2887291896624584,
          "MONDAY_13" to 1.1460926897291355,
          "MONDAY_14" to 1.0004897955233254,
          "MONDAY_15" to 0.8694664537368378,
          "MONDAY_16" to 0.8270273375962802,
          "MONDAY_17" to 0.7868289022833883,
          "MONDAY_18" to 0.7780303121746551,
          "MONDAY_19" to 0.7756215256634205,
          "MONDAY_20" to 0.7984895728860915,
          "MONDAY_21" to 0.8918589268832423,
          "MONDAY_22" to 0.9967716668541272,
          "MONDAY_23" to 1.0973334887144106,
          "TUESDAY_0" to 1.2233064209957951,
          "TUESDAY_1" to 1.3238883432855082,
          "TUESDAY_2" to 1.3874518307497257,
          "TUESDAY_3" to 1.463621147171298,
          "TUESDAY_4" to 1.4975989065490154,
          "TUESDAY_5" to 1.481679186141442,
          "TUESDAY_6" to 1.452778387763161,
          "TUESDAY_7" to 1.3414858185569951,
          "TUESDAY_8" to 1.2869454637983988,
          "TUESDAY_9" to 1.249347290389873,
          "TUESDAY_10" to 1.196488297386161,
          "TUESDAY_11" to 1.1136140507034202,
          "TUESDAY_12" to 0.9867528660797885,
          "TUESDAY_13" to 0.8018989158195754,
          "TUESDAY_14" to 0.6173048748109258,
          "TUESDAY_15" to 0.46718586671750373,
          "TUESDAY_16" to 0.4103633833041902,
          "TUESDAY_17" to 0.4871260756989506,
          "TUESDAY_18" to 0.5667378483016126,
          "TUESDAY_19" to 0.6464203510900723,
          "TUESDAY_20" to 0.7780268325299871,
          "TUESDAY_21" to 0.8995921101255763,
          "TUESDAY_22" to 1.0077600114996088,
          "TUESDAY_23" to 1.1109769960680498,
          "WEDNESDAY_0" to 1.2097668746150059,
          "WEDNESDAY_1" to 1.2631002319009361,
          "WEDNESDAY_2" to 1.2912775191940549,
          "WEDNESDAY_3" to 1.3229785939630059,
          "WEDNESDAY_4" to 1.3428607301494424,
          "WEDNESDAY_5" to 1.3750788517823973,
          "WEDNESDAY_6" to 1.3752344527256497,
          "WEDNESDAY_7" to 1.3505490078766218,
          "WEDNESDAY_8" to 1.2598503219367945,
          "WEDNESDAY_9" to 1.2051668977452374,
          "WEDNESDAY_10" to 1.0320896222195326,
          "WEDNESDAY_11" to 0.8900138031631949,
          "WEDNESDAY_12" to 0.6341155208698448,
          "WEDNESDAY_13" to 0.48337590254714624,
          "WEDNESDAY_14" to 0.2903189399226416,
          "WEDNESDAY_15" to 0.25,
          "WEDNESDAY_16" to 0.25711039485046006,
          "WEDNESDAY_17" to 0.37307641907591793,
          "WEDNESDAY_18" to 0.45280799454961196,
          "WEDNESDAY_19" to 0.5631397823847637,
          "WEDNESDAY_20" to 0.6285005244224133,
          "WEDNESDAY_21" to 0.6671897537279405,
          "WEDNESDAY_22" to 0.7268406397452634,
          "WEDNESDAY_23" to 0.8068904097486369,
          "THURSDAY_0" to 0.9021601102971811,
          "THURSDAY_1" to 1.023741688964238,
          "THURSDAY_2" to 1.1340689935096755,
          "THURSDAY_3" to 1.2530130345819006,
          "THURSDAY_4" to 1.3163421664973542,
          "THURSDAY_5" to 1.3536343767230727,
          "THURSDAY_6" to 1.3432290485306728,
          "THURSDAY_7" to 1.2864983218982178,
          "THURSDAY_8" to 1.2320488534113174,
          "THURSDAY_9" to 1.1984530721079034,
          "THURSDAY_10" to 1.0877338251341975,
          "THURSDAY_11" to 0.9999324929016475,
          "THURSDAY_12" to 0.87536726762619,
          "THURSDAY_13" to 0.6560822412167919,
          "THURSDAY_14" to 0.44836474861432074,
          "THURSDAY_15" to 0.36145134935025247,
          "THURSDAY_16" to 0.2695997829759713,
          "THURSDAY_17" to 0.2898426312618241,
          "THURSDAY_18" to 0.3970093434340387,
          "THURSDAY_19" to 0.5193273246848977,
          "THURSDAY_20" to 0.6426415257034419,
          "THURSDAY_21" to 0.800685718218497,
          "THURSDAY_22" to 0.9215516833839711,
          "THURSDAY_23" to 1.053701659160912,
          "FRIDAY_0" to 1.149649788723893,
          "FRIDAY_1" to 1.2046315447861193,
          "FRIDAY_2" to 1.2724031281576726,
          "FRIDAY_3" to 1.3525693456352732,
          "FRIDAY_4" to 1.3746126314960814,
          "FRIDAY_5" to 1.3744591862592468,
          "FRIDAY_6" to 1.3297812543035683,
          "FRIDAY_7" to 1.2762064429631657,
          "FRIDAY_8" to 1.235662409263294,
          "FRIDAY_9" to 1.2171558028785991,
          "FRIDAY_10" to 1.182722399785398,
          "FRIDAY_11" to 1.137345538963285,
          "FRIDAY_12" to 0.9999308422620752,
          "FRIDAY_13" to 0.8055000309055653,
          "FRIDAY_14" to 0.5667135273493851,
          "FRIDAY_15" to 0.4081529603000651,
          "FRIDAY_16" to 0.3987031354907009,
          "FRIDAY_17" to 0.5030075499003412,
          "FRIDAY_18" to 0.6518159532641841,
          "FRIDAY_19" to 0.8733483414970974,
          "FRIDAY_20" to 1.0496224913080463,
          "FRIDAY_21" to 1.1820684558591705,
          "FRIDAY_22" to 1.2561688567574458,
          "FRIDAY_23" to 1.3204704912328773,
          "SATURDAY_0" to 1.3832230236620218,
          "SATURDAY_1" to 1.4632908341022142,
          "SATURDAY_2" to 1.5019230781315296,
          "SATURDAY_3" to 1.5437332506007084,
          "SATURDAY_4" to 1.5934153179751855,
          "SATURDAY_5" to 1.6245578072557723,
          "SATURDAY_6" to 1.6294919789890665,
          "SATURDAY_7" to 1.6027665451672717,
          "SATURDAY_8" to 1.6068061069158674,
          "SATURDAY_9" to 1.624257927970777,
          "SATURDAY_10" to 1.5996112411089,
          "SATURDAY_11" to 1.5659672993092648,
          "SATURDAY_12" to 1.5333537902522736,
          "SATURDAY_13" to 1.445292929996356,
          "SATURDAY_14" to 1.2966021477035259,
          "SATURDAY_15" to 1.250999408961155,
          "SATURDAY_16" to 1.2535364828163025,
          "SATURDAY_17" to 1.2736456128871074,
          "SATURDAY_18" to 1.3348268054897328,
          "SATURDAY_19" to 1.4571388900094875,
          "SATURDAY_20" to 1.5073787902995706,
          "SATURDAY_21" to 1.5605139580010123,
          "SATURDAY_22" to 1.5885303316932382,
          "SATURDAY_23" to 1.6169891066719597
        )
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
      zkTraces = zkTracesConfig,
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
  fun parsesValidConfig() {
    val smartContractErrorConfig =
      CoordinatorAppCli.loadConfigsOrError<SmartContractErrorCodesConfig>(
        listOf(File("../../config/common/smart-contract-errors.toml"))
      )
    val timeOfDayMultipliers =
      CoordinatorAppCli.loadConfigsOrError<GasPriceCapTimeOfDayMultipliersConfig>(
        listOf(File("../../config/common/gas-price-cap-time-of-day-multipliers.toml"))
      )
    val tracesLimitsConfigs =
      CoordinatorAppCli.loadConfigsOrError<TracesLimitsV1ConfigFile>(
        listOf(File("../../config/common/traces-limits-v1.toml"))
      )
    val tracesLimitsV2Configs =
      CoordinatorAppCli.loadConfigsOrError<TracesLimitsV2ConfigFile>(
        listOf(File("../../config/common/traces-limits-v2.toml"))
      )
    CoordinatorAppCli.loadConfigsOrError<CoordinatorConfigTomlDto>(
      listOf(File("../../config/coordinator/coordinator-docker.config.toml"))
    )
      .onFailure { error: String -> fail(error) }
      .onSuccess { config: CoordinatorConfigTomlDto ->
        val configs = config.copy(
          conflation = config.conflation.copy(
            _tracesLimitsV1 = tracesLimitsConfigs.get()?.tracesLimits?.let { TracesCountersV1(it) },
            _tracesLimitsV2 = tracesLimitsV2Configs.get()?.tracesLimits?.let { TracesCountersV2(it) },
            _smartContractErrors = smartContractErrorConfig.get()!!.smartContractErrors
          ),
          l1DynamicGasPriceCapService = config.l1DynamicGasPriceCapService.copy(
            gasPriceCapCalculation = config.l1DynamicGasPriceCapService.gasPriceCapCalculation.copy(
              timeOfDayMultipliers = timeOfDayMultipliers.get()?.gasPriceCapTimeOfDayMultipliers
            )
          )
        )
        assertEquals(coordinatorConfig, configs.reified())
        assertEquals(coordinatorConfig.l1.rpcEndpoint, coordinatorConfig.l1.ethFeeHistoryEndpoint)
      }
  }

  @Test
  fun parsesValidWeb3SignerConfigOverride() {
    val smartContractErrorCodes: SmartContractErrors =
      CoordinatorAppCli.loadConfigsOrError<SmartContractErrorCodesConfig>(
        listOf(File("../../config/common/smart-contract-errors.toml"))
      ).get()!!.smartContractErrors
    val timeOfDayMultipliers =
      CoordinatorAppCli.loadConfigsOrError<GasPriceCapTimeOfDayMultipliersConfig>(
        listOf(File("../../config/common/gas-price-cap-time-of-day-multipliers.toml"))
      )
    val tracesLimitsConfigs =
      CoordinatorAppCli.loadConfigsOrError<TracesLimitsV1ConfigFile>(
        listOf(File("../../config/common/traces-limits-v1.toml"))
      )
    val tracesLimitsV2Configs =
      CoordinatorAppCli.loadConfigsOrError<TracesLimitsV2ConfigFile>(
        listOf(File("../../config/common/traces-limits-v2.toml"))
      )

    CoordinatorAppCli.loadConfigsOrError<CoordinatorConfigTomlDto>(
      listOf(
        File("../../config/coordinator/coordinator-docker.config.toml"),
        File("../../config/coordinator/coordinator-docker-web3signer-override.config.toml")
      )
    )
      .onFailure { error: String -> fail(error) }
      .onSuccess {
        val configs = it.copy(
          conflation = it.conflation.copy(
            _tracesLimitsV1 = tracesLimitsConfigs.get()?.tracesLimits?.let { TracesCountersV1(it) },
            _tracesLimitsV2 = tracesLimitsV2Configs.get()?.tracesLimits?.let { TracesCountersV2(it) },
            _smartContractErrors = smartContractErrorCodes
          ),
          l1DynamicGasPriceCapService = it.l1DynamicGasPriceCapService.copy(
            gasPriceCapCalculation = it.l1DynamicGasPriceCapService.gasPriceCapCalculation.copy(
              timeOfDayMultipliers = timeOfDayMultipliers.get()?.gasPriceCapTimeOfDayMultipliers
            )
          )
        )

        val expectedConfig =
          coordinatorConfig.copy(
            finalizationSigner = finalizationSigner.copy(type = SignerConfig.Type.Web3Signer),
            dataSubmissionSigner = dataSubmissionSigner.copy(type = SignerConfig.Type.Web3Signer),
            l2Signer = l2SignerConfig.copy(type = SignerConfig.Type.Web3Signer)
          )

        assertEquals(expectedConfig, configs.reified())
      }
  }

  @Test
  fun parsesValidTracesV2ConfigOverride() {
    val smartContractErrorCodes: SmartContractErrors =
      CoordinatorAppCli.loadConfigsOrError<SmartContractErrorCodesConfig>(
        listOf(File("../../config/common/smart-contract-errors.toml"))
      ).get()!!.smartContractErrors
    val timeOfDayMultipliers =
      CoordinatorAppCli.loadConfigsOrError<GasPriceCapTimeOfDayMultipliersConfig>(
        listOf(File("../../config/common/gas-price-cap-time-of-day-multipliers.toml"))
      )
    val tracesLimitsConfigs =
      CoordinatorAppCli.loadConfigsOrError<TracesLimitsV1ConfigFile>(
        listOf(File("../../config/common/traces-limits-v1.toml"))
      )
    val tracesLimitsV2Configs =
      CoordinatorAppCli.loadConfigsOrError<TracesLimitsV2ConfigFile>(
        listOf(File("../../config/common/traces-limits-v2.toml"))
      )

    CoordinatorAppCli.loadConfigsOrError<CoordinatorConfigTomlDto>(
      listOf(
        File("../../config/coordinator/coordinator-docker.config.toml"),
        File("../../config/coordinator/coordinator-docker-traces-v2-override.config.toml")
      )
    )
      .onFailure { error: String -> fail(error) }
      .onSuccess {
        val configs = it.copy(
          conflation = it.conflation.copy(
            _tracesLimitsV1 = tracesLimitsConfigs.get()?.tracesLimits?.let { TracesCountersV1(it) },
            _tracesLimitsV2 = tracesLimitsV2Configs.get()?.tracesLimits?.let { TracesCountersV2(it) },
            _smartContractErrors = smartContractErrorCodes
          ),
          l1DynamicGasPriceCapService = it.l1DynamicGasPriceCapService.copy(
            gasPriceCapCalculation = it.l1DynamicGasPriceCapService.gasPriceCapCalculation.copy(
              timeOfDayMultipliers = timeOfDayMultipliers.get()?.gasPriceCapTimeOfDayMultipliers
            )
          )
        )

        val expectedConfig =
          coordinatorConfig.copy(
            zkTraces = zkTracesConfig.copy(ethApi = URI("http://traces-node-v2:8545").toURL()),
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

        assertEquals(expectedConfig, configs.reified())
      }
  }

  @Test
  fun invalidConfigReturnsErrorResult() {
    val configs =
      CoordinatorAppCli.loadConfigsOrError<TestConfig>(
        listOf(
          File("../../config/coordinator/coordinator-docker.config.toml"),
          File("../../config/coordinator/coordinator-docker-web3signer-override.config.toml")
        )
      )

    assertThat(configs.getError()).contains("'extraField': Missing from config")
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
  fun testValidAggregationAndConflationByTargetBlockNumberWhenL2InclusiveBlockNumberToStopAndFlushAggregationSpecified
  () {
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
