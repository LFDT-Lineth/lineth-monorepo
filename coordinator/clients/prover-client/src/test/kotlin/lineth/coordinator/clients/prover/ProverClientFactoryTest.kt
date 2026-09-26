package lineth.coordinator.clients.prover

import io.micrometer.core.instrument.MeterRegistry
import io.micrometer.core.instrument.simple.SimpleMeterRegistry
import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import linea.domain.BlockIntervalProofIndex
import linea.domain.BlockIntervals
import linea.domain.CompressionProofIndex
import linea.domain.ExecutionProofIndex
import linea.domain.ProofsToAggregate
import linea.kotlin.ByteArrayExt
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.metrics.micrometer.MicrometerMetricsFacade
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatCode
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.awaitility.Awaitility.await
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import org.junit.jupiter.api.io.TempDir
import java.nio.file.Files
import java.nio.file.Path
import kotlin.random.Random
import kotlin.time.Clock
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds
import kotlin.time.Instant
import kotlin.time.toJavaDuration

@ExtendWith(VertxExtension::class)
class ProverClientFactoryTest {
  private fun buildFileBasedProverConfig(proverDir: Path): FileBasedProverConfig {
    return FileBasedProverConfig(
      requestsDirectory = proverDir.resolve("requests"),
      responsesDirectory = proverDir.resolve("responses"),
      pollingInterval = 100.milliseconds,
      pollingTimeout = 500.milliseconds,
      inprogressProvingSuffixPattern = ".*\\.inprogress\\.prover.*",
      inprogressRequestWritingSuffix = ".inprogress_coordinator_writing",
    )
  }

  private fun buildRiscvProverConfig(proverDir: Path): RiscvProverConfig {
    return RiscvProverConfig(
      l2Execution = FileBasedRiscvProverConfig(
        fileBased = buildFileBasedProverConfig(proverDir.resolve("execution")),
        programId = RiscvProverClientTestFixtures.L2_EXECUTION_PROGRAM_ID,
        provingSystemVersion = RiscvProverClientTestFixtures.PROVING_SYSTEM_VERSION,
        forkName = RiscvProverClientTestFixtures.FORK_NAME,
      ),
      rollup = FileBasedRiscvProverConfig(
        fileBased = buildFileBasedProverConfig(proverDir.resolve("rollup")),
        programId = RiscvProverClientTestFixtures.ROLLUP_PROGRAM_ID,
        provingSystemVersion = RiscvProverClientTestFixtures.PROVING_SYSTEM_VERSION,
        forkName = RiscvProverClientTestFixtures.FORK_NAME,
      ),
      rollupAggregation = FileBasedRiscvProverConfig(
        fileBased = buildFileBasedProverConfig(proverDir.resolve("aggregation")),
        programId = RiscvProverClientTestFixtures.ROLLUP_AGGREGATION_PROGRAM_ID,
        provingSystemVersion = RiscvProverClientTestFixtures.PROVING_SYSTEM_VERSION,
        forkName = RiscvProverClientTestFixtures.FORK_NAME,
      ),
    )
  }

  private fun buildPreRiscvProverConfig(proverDir: Path): PreRiscvProverConfig {
    return PreRiscvProverConfig(
      execution = buildFileBasedProverConfig(proverDir.resolve("execution")),
      blobCompression = buildFileBasedProverConfig(proverDir.resolve("compression")),
      proofAggregation = buildFileBasedProverConfig(proverDir.resolve("aggregation")),
    )
  }

  private fun buildRiscvProversConfig(
    tmpDir: Path,
    switchBlockNumber: Int? = null,
    switchBlockTimestamp: Instant? = null,
    withProverB: Boolean = switchBlockNumber != null || switchBlockTimestamp != null,
  ): ProversConfig {
    return ProversConfig(
      proverSwitch = ProverConfigSwitch(
        current = ProverConfig(
          riscvConfig = buildRiscvProverConfig(tmpDir.resolve("riscv-prover/v1")),
        ),
        next = if (withProverB) {
          ProverConfig(
            riscvConfig = buildRiscvProverConfig(tmpDir.resolve("riscv-prover/v2")),
          )
        } else {
          null
        },
      ),
      switchBlockNumberInclusive = switchBlockNumber?.toULong(),
      switchBlockTimestamp = switchBlockTimestamp,
      enableRequestFilesCleanup = false,
    )
  }

  private fun buildPreRiscvProversConfig(
    tmpDir: Path,
    switchBlockNumber: Int? = null,
    switchBlockTimestamp: Instant? = null,
    withProverB: Boolean = switchBlockNumber != null || switchBlockTimestamp != null,
  ): ProversConfig {
    require(!(switchBlockNumber != null && switchBlockTimestamp != null)) {
      "Only one of switchBlockNumber and switchBlockTimestamp may be set"
    }
    return ProversConfig(
      proverSwitch = ProverConfigSwitch(
        current = ProverConfig(
          preRiscvConfig = buildPreRiscvProverConfig(tmpDir.resolve("prover/v2")),
        ),
        next = if (withProverB) {
          ProverConfig(
            preRiscvConfig = buildPreRiscvProverConfig(tmpDir.resolve("prover/v3")),
          )
        } else {
          null
        },
      ),
      switchBlockNumberInclusive = switchBlockNumber?.toULong(),
      switchBlockTimestamp = switchBlockTimestamp,
      enableRequestFilesCleanup = false,
    )
  }

  /** Builds a [ProversConfig] switching from a pre-RISC-V `current` prover to a RISC-V `next` prover. */
  private fun buildPreRiscvToRiscvSwitchProversConfig(
    tmpDir: Path,
    switchBlockNumberInclusive: ULong,
  ): ProversConfig {
    return ProversConfig(
      proverSwitch = ProverConfigSwitch(
        current = ProverConfig(
          preRiscvConfig = buildPreRiscvProverConfig(tmpDir.resolve("prover-switch/pre-riscv")),
        ),
        next = ProverConfig(
          riscvConfig = buildRiscvProverConfig(tmpDir.resolve("prover-switch/riscv")),
        ),
      ),
      switchBlockNumberInclusive = switchBlockNumberInclusive,
      switchBlockTimestamp = null,
      enableRequestFilesCleanup = false,
    )
  }

  private lateinit var meterRegistry: MeterRegistry
  private lateinit var metricsFacade: MetricsFacade
  private lateinit var preRiscvProverClientFactory: ProverClientFactory
  private lateinit var vertx: Vertx
  private lateinit var testTmpDir: Path

  private val request1 = ProofsToAggregate(
    compressionProofIndexes = listOf(
      CompressionProofIndex(
        startBlockNumber = 1uL,
        endBlockNumber = 9uL,
        hash = Random.nextBytes(32),
        startBlockTimestamp = Instant.fromEpochSeconds(1),
      ),
    ),
    executionProofs = BlockIntervals(startingBlockNumber = 1uL, listOf(9uL)),
    invalidityProofs = emptyList(),
    parentAggregationLastBlockTimestamp = Clock.System.now(),
    parentAggregationLastL1RollingHashMessageNumber = 0uL,
    parentAggregationLastL1RollingHash = ByteArrayExt.random32(),
    parentAggregationLastFtxNumber = 0uL,
    parentAggregationLastFtxRollingHash = ByteArrayExt.random32(),
    startBlockTimestamp = Instant.fromEpochSeconds(1),
  )
  private val request2 = ProofsToAggregate(
    compressionProofIndexes = listOf(
      CompressionProofIndex(
        startBlockNumber = 10uL,
        endBlockNumber = 19uL,
        hash = Random.nextBytes(32),
        startBlockTimestamp = Instant.fromEpochSeconds(10),
      ),
    ),
    invalidityProofs = emptyList(),
    executionProofs = BlockIntervals(startingBlockNumber = 10uL, listOf(19uL)),
    parentAggregationLastBlockTimestamp = Clock.System.now(),
    parentAggregationLastL1RollingHashMessageNumber = 9uL,
    parentAggregationLastL1RollingHash = ByteArrayExt.random32(),
    parentAggregationLastFtxNumber = 0uL,
    parentAggregationLastFtxRollingHash = ByteArrayExt.random32(),
    startBlockTimestamp = Instant.fromEpochSeconds(10),
  )
  private val request3 = ProofsToAggregate(
    compressionProofIndexes = listOf(
      CompressionProofIndex(
        startBlockNumber = 300uL,
        endBlockNumber = 319uL,
        hash = Random.nextBytes(32),
        startBlockTimestamp = Instant.fromEpochSeconds(300),
      ),
    ),
    executionProofs = BlockIntervals(startingBlockNumber = 300uL, listOf(319uL)),
    invalidityProofs = emptyList(),
    parentAggregationLastBlockTimestamp = Clock.System.now(),
    parentAggregationLastL1RollingHashMessageNumber = 299uL,
    parentAggregationLastL1RollingHash = ByteArrayExt.random32(),
    parentAggregationLastFtxNumber = 0uL,
    parentAggregationLastFtxRollingHash = ByteArrayExt.random32(),
    startBlockTimestamp = Instant.fromEpochSeconds(300),
  )

  @BeforeEach
  fun beforeEach(vertx: Vertx, @TempDir tmpDir: Path) {
    this.vertx = vertx
    this.testTmpDir = tmpDir
    meterRegistry = SimpleMeterRegistry()
    metricsFacade = MicrometerMetricsFacade(registry = meterRegistry, "linea")
    preRiscvProverClientFactory =
      DefaultProverClientFactory(
        vertx = vertx,
        config = buildPreRiscvProversConfig(testTmpDir, switchBlockNumber = 200),
        chainId = 59144UL,
        l2MessageServiceAddress = "0xa",
        metricsFacade = metricsFacade,
      )
  }

  // --- RISC-V l2-execution prover client ---

  @Test
  fun `l2ExecutionProverClient should build L2 execution prover client`() {
    val factory = DefaultProverClientFactory(
      vertx = vertx,
      config = buildRiscvProversConfig(testTmpDir),
      l2MessageServiceAddress = "0x508Ca82Df566dCD1B0DE8296e70a96332cD644ec",
      chainId = 59144UL,
      metricsFacade = metricsFacade,
    )

    val client = factory.l2ExecutionProverClient()
    assertThat(client).isNotNull
  }

  @Test
  fun `l2ExecutionProverClient should fail when l2MessageServiceAddress is empty`() {
    val factory = DefaultProverClientFactory(
      vertx = vertx,
      config = buildRiscvProversConfig(testTmpDir),
      l2MessageServiceAddress = "",
      chainId = 59144UL,
      metricsFacade = metricsFacade,
    )

    assertThatThrownBy { factory.l2ExecutionProverClient() }
      .isInstanceOf(IllegalArgumentException::class.java)
      .hasMessage("l2MessageServiceAddress must be configured for the RISC-V execution prover")
  }

  // --- current/next prover switching (ABProverClientRouter) ---

  private val switchBlockNumberInclusive = 2_000_000UL
  private val currentProgramId = "0xfedcba1"
  private val currentProvingSystemVersion = "0xabcdef1"
  private val nextProgramId = "0xfedcba4"
  private val nextProvingSystemVersion = "0xabcdef2"

  private fun l2ExecutionRequestAt(blockNumber: ULong) =
    RiscvProverClientTestFixtures.l2ExecutionProofRequestV1(
      executions = listOf(RiscvProverClientTestFixtures.executionInfo(blockNumber)),
    )

  private fun l2ExecutionClientConfig(
    fileBased: FileBasedProverConfig,
    programId: String = currentProgramId,
    provingSystemVersion: String = currentProvingSystemVersion,
  ): FileBasedRiscvProverConfig = FileBasedRiscvProverConfig(
    fileBased = fileBased,
    programId = programId,
    provingSystemVersion = provingSystemVersion,
    forkName = RiscvProverClientTestFixtures.FORK_NAME,
  )

  private fun requestFilePath(config: FileBasedProverConfig, proofIndex: BlockIntervalProofIndex): Path =
    config.requestsDirectory.resolve(L2ExecutionProofFileNameProvider.getFileName(proofIndex))

  private fun requestDtoFromFile(
    config: FileBasedProverConfig,
    proofIndex: BlockIntervalProofIndex,
  ): L2ExecutionProofRequestDto =
    RiscvProverClientTestFixtures.jsonMapper.readValue(
      requestFilePath(config, proofIndex).toFile(),
      L2ExecutionProofRequestDto::class.java,
    )

  /** Builds a [ProversConfig] switching l2-execution (and dummy file-based rollup/aggregation) at [switchBlockNumberInclusive]. */
  private fun buildSwitchProversConfig(
    currentL2Execution: FileBasedRiscvProverConfig,
    nextL2Execution: FileBasedRiscvProverConfig,
    tmpDir: Path,
  ): ProversConfig {
    fun proverConfig(dirSuffix: String, l2Execution: FileBasedRiscvProverConfig) = RiscvProverConfig(
      l2Execution = l2Execution,
      rollup = l2ExecutionClientConfig(
        fileBased = RiscvProverClientTestFixtures.fileBasedProverConfig(tmpDir.resolve("$dirSuffix/rollup")),
      ).copy(programId = RiscvProverClientTestFixtures.ROLLUP_PROGRAM_ID),
      rollupAggregation = l2ExecutionClientConfig(
        fileBased = RiscvProverClientTestFixtures.fileBasedProverConfig(tmpDir.resolve("$dirSuffix/aggregation")),
      ).copy(programId = RiscvProverClientTestFixtures.ROLLUP_AGGREGATION_PROGRAM_ID),
    )
    return ProversConfig(
      proverSwitch = ProverConfigSwitch(
        current = ProverConfig(
          riscvConfig = proverConfig("v1", currentL2Execution),
        ),
        next = ProverConfig(
          riscvConfig = proverConfig("v2", nextL2Execution),
        ),
      ),
      switchBlockNumberInclusive = switchBlockNumberInclusive,
      switchBlockTimestamp = null,
      enableRequestFilesCleanup = false,
    )
  }

  private fun buildL2ExecutionClient(proversConfig: ProversConfig) =
    DefaultProverClientFactory(
      vertx = vertx,
      config = proversConfig,
      l2MessageServiceAddress = RiscvProverClientTestFixtures.L2_MESSAGE_SERVICE_ADDRESS,
      chainId = 59144UL,
      metricsFacade = metricsFacade,
    ).l2ExecutionProverClient()

  @Test
  fun `should switch from current to next prover at switchBlockNumberInclusive when both are file-based`() {
    val currentFileConfig = RiscvProverClientTestFixtures.fileBasedProverConfig(
      testTmpDir.resolve("v1/execution"),
    )
    val nextFileConfig = RiscvProverClientTestFixtures.fileBasedProverConfig(
      testTmpDir.resolve("v2/execution"),
    )

    val client = buildL2ExecutionClient(
      buildSwitchProversConfig(
        currentL2Execution = l2ExecutionClientConfig(
          fileBased = currentFileConfig,
          programId = currentProgramId,
          provingSystemVersion = currentProvingSystemVersion,
        ),
        nextL2Execution = l2ExecutionClientConfig(
          fileBased = nextFileConfig,
          programId = nextProgramId,
          provingSystemVersion = nextProvingSystemVersion,
        ),
        tmpDir = testTmpDir,
      ),
    )

    val beforeSwitchProofIndex = client.createProofRequest(
      l2ExecutionRequestAt(switchBlockNumberInclusive - 1UL),
    ).get()
    assertThat(requestFilePath(currentFileConfig, beforeSwitchProofIndex)).exists()
    assertThat(requestFilePath(nextFileConfig, beforeSwitchProofIndex)).doesNotExist()
    val beforeSwitchDto = requestDtoFromFile(currentFileConfig, beforeSwitchProofIndex)
    assertThat(beforeSwitchDto.programId).isEqualTo(currentProgramId)
    assertThat(beforeSwitchDto.provingSystemVersion).isEqualTo(currentProvingSystemVersion)

    val atSwitchProofIndex =
      client.createProofRequest(l2ExecutionRequestAt(switchBlockNumberInclusive)).get()
    assertThat(requestFilePath(nextFileConfig, atSwitchProofIndex)).exists()
    assertThat(requestFilePath(currentFileConfig, atSwitchProofIndex)).doesNotExist()
    val atSwitchDto = requestDtoFromFile(nextFileConfig, atSwitchProofIndex)
    assertThat(atSwitchDto.programId).isEqualTo(nextProgramId)
    assertThat(atSwitchDto.provingSystemVersion).isEqualTo(nextProvingSystemVersion)
  }

  // --- current (pre-RISC-V) to next (RISC-V) prover-type switching ---

  @Test
  fun `should switch from pre-riscv to riscv prover at switchBlockNumberInclusive`() {
    val switchBlockNumberInclusive = 500UL
    val factory = DefaultProverClientFactory(
      vertx = vertx,
      config = buildPreRiscvToRiscvSwitchProversConfig(testTmpDir, switchBlockNumberInclusive),
      l2MessageServiceAddress = RiscvProverClientTestFixtures.L2_MESSAGE_SERVICE_ADDRESS,
      chainId = 59144UL,
      metricsFacade = metricsFacade,
    )

    val preRiscvClient = factory.preRiscvExecutionProverClient()
    val riscvClient = factory.l2ExecutionProverClient()

    fun executionProofIndexAt(blockNumber: ULong) = ExecutionProofIndex(
      startBlockNumber = blockNumber,
      endBlockNumber = blockNumber,
      startBlockTimestamp = Instant.fromEpochSeconds(blockNumber.toLong()),
    )

    fun blockIntervalProofIndexAt(blockNumber: ULong) =
      RiscvProverClientTestFixtures.blockIntervalProofIndex(blockNumber, blockNumber)

    // before the switch: pre-riscv prover is non-null (usable), riscv prover is null (unusable)
    assertThatCode {
      preRiscvClient.isProofAlreadyDone(executionProofIndexAt(switchBlockNumberInclusive - 1UL))
    }.doesNotThrowAnyException()

    assertThatThrownBy {
      riscvClient.isProofAlreadyDone(blockIntervalProofIndexAt(switchBlockNumberInclusive - 1UL))
    }
      .isInstanceOf(IllegalArgumentException::class.java)
      .hasMessageContaining("proverA should not be null")

    // after the switch: pre-riscv prover is null (unusable), riscv prover is non-null (usable)
    assertThatThrownBy {
      preRiscvClient.isProofAlreadyDone(executionProofIndexAt(switchBlockNumberInclusive))
    }
      .isInstanceOf(IllegalArgumentException::class.java)
      .hasMessageContaining("proverB should not be null")

    assertThatCode {
      riscvClient.isProofAlreadyDone(blockIntervalProofIndexAt(switchBlockNumberInclusive))
    }.doesNotThrowAnyException()
  }

  // --- pre-RISC-V prover client ---

  @Test
  fun `should create a prover with routing when switch is defined`() {
    val proverClient = preRiscvProverClientFactory.preRiscvProofAggregationProverClient()
    assertThat(proverClient).isInstanceOf(ABProverClientRouter::class.java)

    // swallow timeout exception because responses are not available
    runCatching { proverClient.requestProof(request1).get() }
    runCatching { proverClient.requestProof(request2).get() }
    runCatching { proverClient.requestProof(request3).get() }

    await()
      .atMost(5.seconds.toJavaDuration())
      .untilAsserted {
        Files.list(testTmpDir.resolve("prover/v2/aggregation/requests")).use {
          assertThat(it.count()).isEqualTo(2)
        }
        Files.list(testTmpDir.resolve("prover/v3/aggregation/requests")).use {
          assertThat(it.count()).isEqualTo(1)
        }
      }
  }

  @Test
  fun `should create a prover with routing when switchBlockTimestamp is defined`() {
    val factory =
      DefaultProverClientFactory(
        vertx = vertx,
        config = buildPreRiscvProversConfig(testTmpDir, switchBlockTimestamp = Instant.fromEpochSeconds(50)),
        chainId = 59144UL,
        l2MessageServiceAddress = "0xa",
        metricsFacade = metricsFacade,
      )
    val proverClient = factory.preRiscvProofAggregationProverClient()
    assertThat(proverClient).isInstanceOf(ABProverClientRouter::class.java)

    runCatching { proverClient.requestProof(request1).get() }
    runCatching { proverClient.requestProof(request2).get() }
    runCatching { proverClient.requestProof(request3).get() }

    await()
      .atMost(5.seconds.toJavaDuration())
      .untilAsserted {
        Files.list(testTmpDir.resolve("prover/v2/aggregation/requests")).use {
          assertThat(it.count()).isEqualTo(2)
        }
        Files.list(testTmpDir.resolve("prover/v3/aggregation/requests")).use {
          assertThat(it.count()).isEqualTo(1)
        }
      }
  }

  @Test
  fun `should create metrics gauge and aggregate them`() {
    val proverClientI1 = preRiscvProverClientFactory.preRiscvProofAggregationProverClient()
    val proverClientI2 = preRiscvProverClientFactory.preRiscvProofAggregationProverClient()

    runCatching { proverClientI1.requestProof(request1).get() }
    runCatching { proverClientI2.requestProof(request2).get() }
    runCatching { proverClientI1.requestProof(request3).get() }

    assertThat(meterRegistry.find("linea.batch.prover.waiting").gauge()).isNotNull
    assertThat(meterRegistry.find("linea.blob.prover.waiting").gauge()).isNotNull
    assertThat(meterRegistry.find("linea.aggregation.prover.waiting").gauge()).isNotNull

    assertThat(meterRegistry.find("linea.batch.prover.waiting").gauge()!!.value()).isEqualTo(0.0)
    assertThat(meterRegistry.find("linea.blob.prover.waiting").gauge()!!.value()).isEqualTo(0.0)
    assertThat(meterRegistry.find("linea.aggregation.prover.waiting").gauge()!!.value()).isEqualTo(3.0)
  }
}
