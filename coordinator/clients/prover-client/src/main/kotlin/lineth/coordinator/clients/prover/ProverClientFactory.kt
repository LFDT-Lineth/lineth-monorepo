package lineth.coordinator.clients.prover

import io.vertx.core.Vertx
import linea.clients.BlobCompressionProverClientV2
import linea.clients.ExecutionProverClientV2
import linea.clients.InvalidityProverClientV1
import linea.clients.L2ExecutionProverClientV1
import linea.clients.ProofAggregationProverClientV2
import linea.clients.ProverFileNameProvider
import linea.clients.ProverProofTransport
import linea.clients.RollupAggregationProverClientV1
import linea.clients.RollupProverClientV1
import linea.domain.BlockIntervalProofIndex
import lineth.coordinator.clients.prover.serialization.JsonSerialization
import lineth.fileio.FileReader
import lineth.fileio.FileWriter
import lineth.metrics.LineaMetricsCategory
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.metrics.micrometer.GaugeAggregator
import org.apache.logging.log4j.Logger

/**
 * Builds the prover clients a conflation app needs.
 *
 * Implementors that do not delegate to [DefaultProverClientFactory] must register the
 * `prover.waiting` gauges themselves (see [ProverClientFactorySupport.registerWaitingGauges]) —
 * those gauges are per-instance state, so an implementation that neither delegates nor registers
 * them silently reports nothing.
 */
interface ProverClientFactory {
  fun preRiscvExecutionProverClient(): ExecutionProverClientV2

  fun preRiscvBlobCompressionProverClient(
    log: Logger = PreRiscvBlobCompressionProverClient.LOG,
  ): BlobCompressionProverClientV2

  fun preRiscvProofAggregationProverClient(
    log: Logger = PreRiscvProofAggregationClient.LOG,
  ): ProofAggregationProverClientV2

  fun preRiscvInvalidityProverClient(): InvalidityProverClientV1

  /**
   * RISC-V l2-execution prover client.
   */
  fun l2ExecutionProverClient(): L2ExecutionProverClientV1

  /**
   * RISC-V rollup prover client: recursively verifies the execution proofs of the conflations it
   * covers.
   */
  fun rollupProverClient(): RollupProverClientV1

  /** RISC-V rollup-aggregation prover client: aggregates a span of rollup proofs. */
  fun rollupAggregationProverClient(): RollupAggregationProverClientV1
}

/**
 * Builds a [ProverClientFactory] for one prover configuration.
 */
fun interface ProverClientFactoryBuilder {
  fun build(
    vertx: Vertx,
    config: ProversConfig,
    l2MessageServiceAddress: String,
    chainId: ULong,
    metricsFacade: MetricsFacade,
  ): ProverClientFactory

  companion object {
    /** The built-in, file-based factory. */
    val FILE_BASED = ProverClientFactoryBuilder {
        vertx,
        config,
        l2MessageServiceAddress,
        chainId,
        metricsFacade,
      ->
      DefaultProverClientFactory(vertx, config, chainId, l2MessageServiceAddress, metricsFacade)
    }
  }
}

/**
 * The seven `prover.waiting` gauges, one per proof category, shared by every [ProverClientFactory]
 * implementation.
 *
 * Extracted so an implementation that does not delegate to [DefaultProverClientFactory] can still
 * report them: each [GaugeAggregator] accumulates the clients registered against *this* instance,
 * so the registration cannot be inherited by construction alone. Registering the same gauge name
 * twice against one Micrometer registry silently discards the second supplier, so create one of
 * these per factory instance and no more.
 */
class ProverClientFactorySupport(metricsFacade: MetricsFacade) {
  val executionWaitingResponses = GaugeAggregator()
  val blobWaitingResponses = GaugeAggregator()
  val aggregationWaitingResponses = GaugeAggregator()
  val invalidityWaitingResponses = GaugeAggregator()
  val l2ExecutionWaitingResponses = GaugeAggregator()
  val rollupWaitingResponses = GaugeAggregator()
  val rollupAggregationWaitingResponses = GaugeAggregator()

  init {
    registerWaitingGauges(metricsFacade)
  }

  private fun registerWaitingGauges(metricsFacade: MetricsFacade) {
    metricsFacade.createGauge(
      category = LineaMetricsCategory.BATCH,
      name = "prover.waiting",
      description = "Number of execution proof waiting responses",
      measurementSupplier = executionWaitingResponses,
    )
    metricsFacade.createGauge(
      category = LineaMetricsCategory.BLOB,
      name = "prover.waiting",
      description = "Number of blob compression proof waiting responses",
      measurementSupplier = blobWaitingResponses,
    )
    metricsFacade.createGauge(
      category = LineaMetricsCategory.AGGREGATION,
      name = "prover.waiting",
      description = "Number of aggregation proof waiting responses",
      measurementSupplier = aggregationWaitingResponses,
    )
    metricsFacade.createGauge(
      category = LineaMetricsCategory.FORCED_TRANSACTION,
      name = "prover.waiting",
      description = "Number of invalidity proof waiting responses",
      measurementSupplier = invalidityWaitingResponses,
    )
    metricsFacade.createGauge(
      category = LineaMetricsCategory.RISCV_L2_EXECUTION,
      name = "prover.waiting",
      description = "Number of RISC-V l2-execution proof waiting responses",
      measurementSupplier = l2ExecutionWaitingResponses,
    )
    metricsFacade.createGauge(
      category = LineaMetricsCategory.RISCV_ROLLUP,
      name = "prover.waiting",
      description = "Number of RISC-V rollup proof waiting responses",
      measurementSupplier = rollupWaitingResponses,
    )
    metricsFacade.createGauge(
      category = LineaMetricsCategory.RISCV_ROLLUP_AGGREGATION,
      name = "prover.waiting",
      description = "Number of RISC-V rollup-aggregation proof waiting responses",
      measurementSupplier = rollupAggregationWaitingResponses,
    )
  }
}

class DefaultProverClientFactory(
  private val vertx: Vertx,
  private val config: ProversConfig,
  private val chainId: ULong,
  private val l2MessageServiceAddress: String,
  metricsFacade: MetricsFacade,
  private val support: ProverClientFactorySupport = ProverClientFactorySupport(metricsFacade),
) : ProverClientFactory {
  private fun requireRiscvConfig(): ProversConfig {
    require(
      config.proverSwitch.current.riscvConfig != null ||
        config.proverSwitch.next?.riscvConfig != null,
    ) {
      "RISC-V prover config must be configured in either current or next"
    }
    return config
  }

  private fun requirePreRiscvConfig(): ProversConfig {
    require(config.proverSwitch.current.preRiscvConfig != null) {
      "Pre RISC-V prover config must be configured in current"
    }
    return config
  }

  override fun l2ExecutionProverClient(): L2ExecutionProverClientV1 {
    val config = requireRiscvConfig()
    return ABProverClientRouter.create(
      proverAConfig = config.proverSwitch.current.riscvConfig?.l2Execution,
      proverBConfig = config.proverSwitch.next?.riscvConfig?.l2Execution,
      switchBlockNumberInclusive = config.switchBlockNumberInclusive,
      switchBlockTimestamp = config.switchBlockTimestamp,
    ) { proverConfig ->
      buildFileBasedL2ExecutionProverClient(proverConfig)
        .also { support.l2ExecutionWaitingResponses.addReporter(it) }
    }
  }

  override fun rollupProverClient(): RollupProverClientV1 {
    val config = requireRiscvConfig()
    return ABProverClientRouter.create(
      proverAConfig = config.proverSwitch.current.riscvConfig,
      proverBConfig = config.proverSwitch.next?.riscvConfig,
      switchBlockNumberInclusive = config.switchBlockNumberInclusive,
      switchBlockTimestamp = config.switchBlockTimestamp,
    ) { proverConfig ->
      buildFileBasedRollupProverClient(
        proverConfig = proverConfig.rollup,
        l2ExecutionProverConfig = proverConfig.l2Execution,
      )
        .also { support.rollupWaitingResponses.addReporter(it) }
    }
  }

  override fun rollupAggregationProverClient(): RollupAggregationProverClientV1 {
    val config = requireRiscvConfig()
    return ABProverClientRouter.create(
      proverAConfig = config.proverSwitch.current.riscvConfig,
      proverBConfig = config.proverSwitch.next?.riscvConfig,
      switchBlockNumberInclusive = config.switchBlockNumberInclusive,
      switchBlockTimestamp = config.switchBlockTimestamp,
    ) { proverConfig ->
      buildFileBasedRollupAggregationProverClient(
        proverConfig = proverConfig.rollupAggregation,
        rollupProverConfig = proverConfig.rollup,
      )
        .also { support.rollupAggregationWaitingResponses.addReporter(it) }
    }
  }

  override fun preRiscvExecutionProverClient(): ExecutionProverClientV2 {
    val preRiscvConfig = requirePreRiscvConfig()
    return ABProverClientRouter.create(
      proverAConfig = preRiscvConfig.proverSwitch.current.preRiscvConfig?.execution,
      proverBConfig = preRiscvConfig.proverSwitch.next?.preRiscvConfig?.execution,
      switchBlockNumberInclusive = preRiscvConfig.switchBlockNumberInclusive,
      switchBlockTimestamp = preRiscvConfig.switchBlockTimestamp,
    ) { proverConfig ->
      PreRiscvExecutionProverClient(
        config = proverConfig,
        vertx = vertx,
        enableRequestFilesCleanup = preRiscvConfig.enableRequestFilesCleanup,
      ).also { support.executionWaitingResponses.addReporter(it) }
    }
  }

  override fun preRiscvBlobCompressionProverClient(
    log: Logger,
  ): BlobCompressionProverClientV2 {
    val preRiscvConfig = requirePreRiscvConfig()
    return ABProverClientRouter.create(
      proverAConfig = preRiscvConfig.proverSwitch.current.preRiscvConfig!!.blobCompression,
      proverBConfig = preRiscvConfig.proverSwitch.next?.preRiscvConfig?.blobCompression,
      switchBlockNumberInclusive = preRiscvConfig.switchBlockNumberInclusive,
      switchBlockTimestamp = preRiscvConfig.switchBlockTimestamp,
    ) { proverConfig ->
      PreRiscvBlobCompressionProverClient(
        config = proverConfig,
        vertx = vertx,
        enableRequestFilesCleanup = preRiscvConfig.enableRequestFilesCleanup,
        log = log,
      )
        .also { support.blobWaitingResponses.addReporter(it) }
    }
  }

  override fun preRiscvProofAggregationProverClient(
    log: Logger,
  ): ProofAggregationProverClientV2 {
    val preRiscvConfig = requirePreRiscvConfig()
    return ABProverClientRouter.create(
      proverAConfig = preRiscvConfig.proverSwitch.current.preRiscvConfig!!,
      proverBConfig = preRiscvConfig.proverSwitch.next?.preRiscvConfig,
      switchBlockNumberInclusive = preRiscvConfig.switchBlockNumberInclusive,
      switchBlockTimestamp = preRiscvConfig.switchBlockTimestamp,
    ) { proverConfig ->
      PreRiscvProofAggregationClient(
        config = proverConfig.proofAggregation,
        invalidityProverConfig = proverConfig.invalidity,
        vertx = vertx,
        enableRequestFilesCleanup = preRiscvConfig.enableRequestFilesCleanup,
        log = log,
      )
        .also { support.aggregationWaitingResponses.addReporter(it) }
    }
  }

  override fun preRiscvInvalidityProverClient(): InvalidityProverClientV1 {
    val preRiscvConfig = requirePreRiscvConfig()
    if (preRiscvConfig.proverSwitch.current.preRiscvConfig!!.invalidity == null) {
      throw IllegalStateException("Invalidity prover config is not configured")
    }

    return ABProverClientRouter.create(
      proverAConfig = preRiscvConfig.proverSwitch.current.preRiscvConfig.invalidity,
      proverBConfig = preRiscvConfig.proverSwitch.next?.preRiscvConfig?.invalidity,
      switchBlockNumberInclusive = preRiscvConfig.switchBlockNumberInclusive,
      switchBlockTimestamp = preRiscvConfig.switchBlockTimestamp,
    ) { proverConfig ->
      PreRiscvInvalidityProverClient(
        config = proverConfig,
        vertx = vertx,
        enableRequestFilesCleanup = preRiscvConfig.enableRequestFilesCleanup,
      )
        .also { support.invalidityWaitingResponses.addReporter(it) }
    }
  }

  private fun <RequestDto : Any, ResponseDto> fileBasedTransport(
    proverConfig: FileBasedRiscvProverConfig,
    requestFileNameProvider: ProverFileNameProvider<BlockIntervalProofIndex>,
    responseFileNameProvider: ProverFileNameProvider<BlockIntervalProofIndex>,
    responseDtoClass: Class<ResponseDto>,
  ): ProverProofTransport<RequestDto, ResponseDto, BlockIntervalProofIndex> {
    return FileBasedProverProofTransport<
      RequestDto,
      ResponseDto,
      BlockIntervalProofIndex,
      >(
      config = proverConfig.fileBased,
      vertx = vertx,
      fileWriter = FileWriter(vertx, JsonSerialization.proofResponseMapperV1),
      fileReader = FileReader(
        vertx,
        JsonSerialization.proofResponseMapperV1,
        responseDtoClass,
      ),
      requestFileNameProvider = requestFileNameProvider,
      responseFileNameProvider = responseFileNameProvider,
      enableRequestFilesCleanup = requireRiscvConfig().enableRequestFilesCleanup,
    )
  }

  private fun buildL2ExecutionProofTransport(proverConfig: FileBasedRiscvProverConfig) =
    fileBasedTransport<L2ExecutionProofRequestDto, L2ExecutionProofResponseDto>(
      proverConfig = proverConfig,
      requestFileNameProvider = L2ExecutionProofFileNameProvider,
      responseFileNameProvider = L2ExecutionProofFileNameProvider,
      responseDtoClass = L2ExecutionProofResponseDto::class.java,
    )

  private fun <RequestDto : Any> buildRollupProofTransport(proverConfig: FileBasedRiscvProverConfig) =
    fileBasedTransport<RequestDto, RollupProofResponseDto>(
      proverConfig = proverConfig,
      requestFileNameProvider = RollupProofFileNameProvider,
      responseFileNameProvider = RollupProofFileNameProvider,
      responseDtoClass = RollupProofResponseDto::class.java,
    )

  private fun <RequestDto : Any> buildRollupAggregationProofTransport(proverConfig: FileBasedRiscvProverConfig) =
    fileBasedTransport<RequestDto, RollupAggregationProofResponseDto>(
      proverConfig = proverConfig,
      requestFileNameProvider = RollupAggregationProofFileNameProvider,
      responseFileNameProvider = RollupAggregationProofFileNameProvider,
      responseDtoClass = RollupAggregationProofResponseDto::class.java,
    )

  private fun buildFileBasedL2ExecutionProverClient(proverConfig: FileBasedRiscvProverConfig): L2ExecutionProverClient {
    require(l2MessageServiceAddress.isNotEmpty()) {
      "l2MessageServiceAddress must be configured for the RISC-V execution prover"
    }

    return L2ExecutionProverClient(
      transport = buildL2ExecutionProofTransport(proverConfig),
      programId = proverConfig.programId,
      provingSystemVersion = proverConfig.provingSystemVersion,
      l2MessageServiceAddress = l2MessageServiceAddress,
      forkName = proverConfig.forkName,
    )
  }

  private fun buildFileBasedRollupProverClient(
    proverConfig: FileBasedRiscvProverConfig,
    l2ExecutionProverConfig: FileBasedRiscvProverConfig,
  ): FileBasedRollupProverClient {
    return FileBasedRollupProverClient(
      transport = buildRollupProofTransport(proverConfig),
      l2ExecutionProofTransport = buildL2ExecutionProofTransport(l2ExecutionProverConfig),
      programId = proverConfig.programId,
      provingSystemVersion = proverConfig.provingSystemVersion,
      chainId = chainId.toLong(),
    )
  }

  private fun buildFileBasedRollupAggregationProverClient(
    proverConfig: FileBasedRiscvProverConfig,
    rollupProverConfig: FileBasedRiscvProverConfig,
  ): FileBasedRollupAggregationProverClient {
    return FileBasedRollupAggregationProverClient(
      transport = buildRollupAggregationProofTransport(proverConfig),
      rollupProofTransport = buildRollupProofTransport(rollupProverConfig),
      programId = proverConfig.programId,
      provingSystemVersion = proverConfig.provingSystemVersion,
    )
  }
}
