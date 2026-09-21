package lineth.coordinator.clients.prover

import com.github.tomakehurst.wiremock.WireMockServer
import com.github.tomakehurst.wiremock.client.WireMock
import com.github.tomakehurst.wiremock.core.WireMockConfiguration
import io.micrometer.core.instrument.MeterRegistry
import io.micrometer.core.instrument.simple.SimpleMeterRegistry
import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import linea.domain.BlockIntervalProofIndex
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.metrics.micrometer.MicrometerMetricsFacade
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import org.junit.jupiter.api.io.TempDir
import java.net.URI
import java.nio.file.Path
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds
import kotlin.time.Instant

@ExtendWith(VertxExtension::class)
class ProverClientFactoryTest {
  private fun buildProversConfig(
    tmpDir: Path,
    switchBlockNumber: Int? = null,
    switchBlockTimestamp: Instant? = null,
    withProverB: Boolean = switchBlockNumber != null || switchBlockTimestamp != null,
  ): ProversConfig<ProverConfig> {
    fun buildFileBasedProverConfig(
      proverDir: Path,
    ): FileBasedProverConfig {
      return FileBasedProverConfig(
        requestsDirectory = proverDir.resolve("requests"),
        responsesDirectory = proverDir.resolve("responses"),
        pollingInterval = 100.milliseconds,
        pollingTimeout = 500.milliseconds,
        inprogressProvingSuffixPattern = ".*\\.inprogress\\.prover.*",
        inprogressRequestWritingSuffix = ".inprogress_coordinator_writing",
      )
    }

    fun buildProverConfig(proverDir: Path): ProverConfig {
      return ProverConfig(
        l2Execution = ProverClientConfig(
          fileBased = buildFileBasedProverConfig(proverDir.resolve("execution")),
          restfulBased = null,
          programId = RiscvProverClientTestFixtures.L2_EXECUTION_PROGRAM_ID,
          provingSystemVersion = RiscvProverClientTestFixtures.PROVING_SYSTEM_VERSION,
          forkName = RiscvProverClientTestFixtures.FORK_NAME,
        ),
        rollup = ProverClientConfig(
          fileBased = buildFileBasedProverConfig(proverDir.resolve("rollup")),
          restfulBased = null,
          programId = RiscvProverClientTestFixtures.ROLLUP_PROGRAM_ID,
          provingSystemVersion = RiscvProverClientTestFixtures.PROVING_SYSTEM_VERSION,
          forkName = RiscvProverClientTestFixtures.FORK_NAME,
        ),
        rollupAggregation = ProverClientConfig(
          fileBased = buildFileBasedProverConfig(proverDir.resolve("aggregation")),
          restfulBased = null,
          programId = RiscvProverClientTestFixtures.ROLLUP_AGGREGATION_PROGRAM_ID,
          provingSystemVersion = RiscvProverClientTestFixtures.PROVING_SYSTEM_VERSION,
          forkName = RiscvProverClientTestFixtures.FORK_NAME,
        ),
      )
    }

    return ProversConfig(
      proverSwitch = ProverConfigSwitch(
        current = buildProverConfig(tmpDir.resolve("riscv-prover/v1")),
        next = if (withProverB) {
          buildProverConfig(tmpDir.resolve("riscv-prover/v2"))
        } else {
          null
        },
      ),
      switchBlockNumberInclusive = switchBlockNumber?.toULong(),
      switchBlockTimestamp = switchBlockTimestamp,
      enableRequestFilesCleanup = false,
    )
  }

  private lateinit var meterRegistry: MeterRegistry
  private lateinit var metricsFacade: MetricsFacade
  private lateinit var proverClientFactory: ProverClientFactory
  private lateinit var vertx: Vertx
  private lateinit var testTmpDir: Path

  @BeforeEach
  fun beforeEach(vertx: Vertx, @TempDir tmpDir: Path) {
    this.vertx = vertx
    this.testTmpDir = tmpDir
    meterRegistry = SimpleMeterRegistry()
    metricsFacade = MicrometerMetricsFacade(registry = meterRegistry, "linea")
    proverClientFactory =
      ProverClientFactory(
        vertx = vertx,
        config = buildProversConfig(testTmpDir, switchBlockNumber = 200),
        chainId = 59144L,
        metricsFacade = metricsFacade,
      )
  }

  @Test
  fun `l2ExecutionProverClient should build L2 execution prover client`() {
    val factory = ProverClientFactory(
      vertx = vertx,
      config = buildProversConfig(testTmpDir),
      l2MessageServiceAddress = "0x508Ca82Df566dCD1B0DE8296e70a96332cD644ec",
      chainId = 59144L,
      metricsFacade = metricsFacade,
    )

    val client = factory.l2ExecutionProverClient()
    assertThat(client).isNotNull
  }

  @Test
  fun `l2ExecutionProverClient should fail when l2MessageServiceAddress is not configured`() {
    val factory = ProverClientFactory(
      vertx = vertx,
      config = buildProversConfig(testTmpDir),
      l2MessageServiceAddress = null,
      chainId = 59144L,
      metricsFacade = metricsFacade,
    )

    assertThatThrownBy { factory.l2ExecutionProverClient() }
      .isInstanceOf(IllegalArgumentException::class.java)
      .hasMessage("l2MessageServiceAddress must be configured for the RISC-V execution prover")
  }

  @Test
  fun `l2ExecutionProverClient should fail when l2MessageServiceAddress is empty`() {
    val factory = ProverClientFactory(
      vertx = vertx,
      config = buildProversConfig(testTmpDir),
      l2MessageServiceAddress = "",
      chainId = 59144L,
      metricsFacade = metricsFacade,
    )

    assertThatThrownBy { factory.l2ExecutionProverClient() }
      .isInstanceOf(IllegalArgumentException::class.java)
      .hasMessage("l2MessageServiceAddress must be configured for the RISC-V execution prover")
  }

  @Test
  fun `should fail with clear error when block number switch has no prover B`() {
    val factory =
      ProverClientFactory(
        vertx = vertx,
        config = buildProversConfig(testTmpDir, switchBlockNumber = 200, withProverB = false),
        chainId = 59144L,
        metricsFacade = metricsFacade,
      )

    assertThatThrownBy { factory.rollupProverClient() }
      .isInstanceOf(IllegalArgumentException::class.java)
      .hasMessage("proverBConfig must be provided when switchBlockNumberInclusive is set")
  }

  @Test
  fun `should fail with clear error when timestamp switch has no prover B`() {
    val factory =
      ProverClientFactory(
        vertx = vertx,
        config = buildProversConfig(
          testTmpDir,
          switchBlockTimestamp = Instant.fromEpochSeconds(50),
          withProverB = false,
        ),
        chainId = 59144L,
        metricsFacade = metricsFacade,
      )

    assertThatThrownBy { factory.rollupAggregationProverClient() }
      .isInstanceOf(IllegalArgumentException::class.java)
      .hasMessage("proverBConfig must be provided when switchBlockTimestamp is set")
  }

  // --- current/next prover switching (ABProverClientRouter) ---

  private val switchBlockNumberInclusive = 2_000_000UL
  private val l2ExecutionJobsPathPattern = "/api/v1/jobs/59144/execution/.*"
  private val currentProgramId = "0xfedcba1"
  private val currentProvingSystemVersion = "0xabcdef1"
  private val nextProgramId = "0xfedcba4"
  private val nextProvingSystemVersion = "0xabcdef2"

  private fun l2ExecutionRequestAt(blockNumber: ULong) =
    RiscvProverClientTestFixtures.l2ExecutionProofRequestV1(
      executions = listOf(RiscvProverClientTestFixtures.executionInfo(blockNumber)),
    )

  private fun l2ExecutionClientConfig(
    fileBased: FileBasedProverConfig? = null,
    restfulBased: RestfulBasedProverConfig? = null,
    programId: String = currentProgramId,
    provingSystemVersion: String = currentProvingSystemVersion,
  ): ProverClientConfig = ProverClientConfig(
    fileBased = fileBased,
    restfulBased = restfulBased,
    programId = programId,
    provingSystemVersion = provingSystemVersion,
    forkName = RiscvProverClientTestFixtures.FORK_NAME,
  )

  private fun restfulProverConfig(wiremock: WireMockServer): RestfulBasedProverConfig = RestfulBasedProverConfig(
    endpoint = URI("http://localhost:${wiremock.port()}/").toURL(),
    restfulApiBasePath = "/api",
    restfulApiVersion = "v1",
    pollingInterval = 50.milliseconds,
    pollingTimeout = 2.seconds,
  )

  private fun startProverWiremock(): WireMockServer {
    val wiremock = WireMockServer(WireMockConfiguration.options().dynamicPort())
    wiremock.start()
    wiremock.stubFor(WireMock.get(WireMock.urlPathMatching(l2ExecutionJobsPathPattern)).willReturn(WireMock.notFound()))
    wiremock.stubFor(WireMock.post(WireMock.urlPathMatching(l2ExecutionJobsPathPattern)).willReturn(WireMock.ok()))
    return wiremock
  }

  private fun postedCount(wiremock: WireMockServer): Int =
    wiremock.findAll(WireMock.postRequestedFor(WireMock.urlPathMatching(l2ExecutionJobsPathPattern))).size

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

  /** Parses the `proof_request` body of the [index]-th (0-based) request posted to [wiremock]. */
  private fun requestDtoFromWiremock(wiremock: WireMockServer, index: Int = 0): L2ExecutionProofRequestDto {
    val postedRequest = wiremock.findAll(
      WireMock.postRequestedFor(WireMock.urlPathMatching(l2ExecutionJobsPathPattern)),
    )[index]
    val body = RiscvProverClientTestFixtures.jsonMapper.readTree(postedRequest.bodyAsString)
    return RiscvProverClientTestFixtures.jsonMapper.treeToValue(
      body.get("proof_request"),
      L2ExecutionProofRequestDto::class.java,
    )
  }

  /** Builds a [ProversConfig] switching l2-execution (and dummy file-based rollup/aggregation) at [switchBlockNumberInclusive]. */
  private fun buildSwitchProversConfig(
    currentL2Execution: ProverClientConfig,
    nextL2Execution: ProverClientConfig,
    tmpDir: Path,
  ): ProversConfig<ProverConfig> {
    fun proverConfig(dirSuffix: String, l2Execution: ProverClientConfig) = ProverConfig(
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
        current = proverConfig("v1", currentL2Execution),
        next = proverConfig("v2", nextL2Execution),
      ),
      switchBlockNumberInclusive = switchBlockNumberInclusive,
      switchBlockTimestamp = null,
      enableRequestFilesCleanup = false,
    )
  }

  private fun buildL2ExecutionClient(proversConfig: ProversConfig<ProverConfig>) =
    ProverClientFactory(
      vertx = vertx,
      config = proversConfig,
      l2MessageServiceAddress = RiscvProverClientTestFixtures.L2_MESSAGE_SERVICE_ADDRESS,
      chainId = RiscvProverClientTestFixtures.CHAIN_ID,
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

  @Test
  fun `should switch from current to next prover at switchBlockNumberInclusive when both are restful-based`() {
    val currentWiremock = startProverWiremock()
    val nextWiremock = startProverWiremock()
    try {
      val client = buildL2ExecutionClient(
        buildSwitchProversConfig(
          currentL2Execution = l2ExecutionClientConfig(
            restfulBased = restfulProverConfig(currentWiremock),
            programId = currentProgramId,
            provingSystemVersion = currentProvingSystemVersion,
          ),
          nextL2Execution = l2ExecutionClientConfig(
            restfulBased = restfulProverConfig(nextWiremock),
            programId = nextProgramId,
            provingSystemVersion = nextProvingSystemVersion,
          ),
          tmpDir = testTmpDir,
        ),
      )

      client.createProofRequest(l2ExecutionRequestAt(switchBlockNumberInclusive - 1UL)).get()
      assertThat(postedCount(currentWiremock)).isEqualTo(1)
      assertThat(postedCount(nextWiremock)).isEqualTo(0)
      val beforeSwitchDto = requestDtoFromWiremock(currentWiremock)
      assertThat(beforeSwitchDto.programId).isEqualTo(currentProgramId)
      assertThat(beforeSwitchDto.provingSystemVersion).isEqualTo(currentProvingSystemVersion)

      client.createProofRequest(l2ExecutionRequestAt(switchBlockNumberInclusive)).get()
      assertThat(postedCount(nextWiremock)).isEqualTo(1)
      assertThat(postedCount(currentWiremock)).isEqualTo(1)
      val atSwitchDto = requestDtoFromWiremock(nextWiremock)
      assertThat(atSwitchDto.programId).isEqualTo(nextProgramId)
      assertThat(atSwitchDto.provingSystemVersion).isEqualTo(nextProvingSystemVersion)
    } finally {
      currentWiremock.stop()
      nextWiremock.stop()
    }
  }

  @Test
  fun `should switch from current file-based prover to next restful-based prover at switchBlockNumberInclusive`() {
    val currentFileConfig = RiscvProverClientTestFixtures.fileBasedProverConfig(
      testTmpDir.resolve("v1/execution"),
    )
    val nextWiremock = startProverWiremock()
    try {
      val client = buildL2ExecutionClient(
        buildSwitchProversConfig(
          currentL2Execution = l2ExecutionClientConfig(
            fileBased = currentFileConfig,
            programId = currentProgramId,
            provingSystemVersion = currentProvingSystemVersion,
          ),
          nextL2Execution = l2ExecutionClientConfig(
            restfulBased = restfulProverConfig(nextWiremock),
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
      assertThat(postedCount(nextWiremock)).isEqualTo(0)
      val beforeSwitchDto = requestDtoFromFile(currentFileConfig, beforeSwitchProofIndex)
      assertThat(beforeSwitchDto.programId).isEqualTo(currentProgramId)
      assertThat(beforeSwitchDto.provingSystemVersion).isEqualTo(currentProvingSystemVersion)

      client.createProofRequest(l2ExecutionRequestAt(switchBlockNumberInclusive)).get()
      assertThat(postedCount(nextWiremock)).isEqualTo(1)
      val atSwitchDto = requestDtoFromWiremock(nextWiremock)
      assertThat(atSwitchDto.programId).isEqualTo(nextProgramId)
      assertThat(atSwitchDto.provingSystemVersion).isEqualTo(nextProvingSystemVersion)
    } finally {
      nextWiremock.stop()
    }
  }

  @Test
  fun `should switch from current restful-based prover to next file-based prover at switchBlockNumberInclusive`() {
    val currentWiremock = startProverWiremock()
    val nextFileConfig = RiscvProverClientTestFixtures.fileBasedProverConfig(
      testTmpDir.resolve("v2/execution"),
    )
    try {
      val client = buildL2ExecutionClient(
        buildSwitchProversConfig(
          currentL2Execution = l2ExecutionClientConfig(
            restfulBased = restfulProverConfig(currentWiremock),
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

      client.createProofRequest(l2ExecutionRequestAt(switchBlockNumberInclusive - 1UL)).get()
      assertThat(postedCount(currentWiremock)).isEqualTo(1)
      val beforeSwitchDto = requestDtoFromWiremock(currentWiremock)
      assertThat(beforeSwitchDto.programId).isEqualTo(currentProgramId)
      assertThat(beforeSwitchDto.provingSystemVersion).isEqualTo(currentProvingSystemVersion)

      val atSwitchProofIndex =
        client.createProofRequest(l2ExecutionRequestAt(switchBlockNumberInclusive)).get()
      assertThat(requestFilePath(nextFileConfig, atSwitchProofIndex)).exists()
      assertThat(postedCount(currentWiremock)).isEqualTo(1)
      val atSwitchDto = requestDtoFromFile(nextFileConfig, atSwitchProofIndex)
      assertThat(atSwitchDto.programId).isEqualTo(nextProgramId)
      assertThat(atSwitchDto.provingSystemVersion).isEqualTo(nextProvingSystemVersion)
    } finally {
      currentWiremock.stop()
    }
  }
}
