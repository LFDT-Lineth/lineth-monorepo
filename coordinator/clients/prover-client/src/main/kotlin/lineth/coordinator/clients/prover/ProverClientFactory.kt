package lineth.coordinator.clients.prover

import io.vertx.core.Vertx
import linea.clients.BlobCompressionProverClientV2
import linea.clients.ExecutionProverClientV2
import linea.clients.InvalidityProverClientV1
import linea.clients.L2ExecutionProverClientV1
import linea.clients.ProofAggregationProverClientV2
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

  fun executionProverClient(): L2ExecutionProverClientV1

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
      DefaultProverClientFactory(vertx, config, l2MessageServiceAddress, chainId, metricsFacade)
    }
  }
}

/**
 * The four `prover.waiting` gauges, one per proof category, shared by every [ProverClientFactory]
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
  }
}

/**
 * The built-in [ProverClientFactory]: every client writes its request to, and reads its response
 * from, the local filesystem (see [FileBasedProverProofTransport]).
 */
class DefaultProverClientFactory(
  private val vertx: Vertx,
  private val config: ProversConfig,
  private val l2MessageServiceAddress: String,
  private val chainId: ULong,
  metricsFacade: MetricsFacade,
  private val support: ProverClientFactorySupport = ProverClientFactorySupport(metricsFacade),
) : ProverClientFactory {

  override fun preRiscvExecutionProverClient(): ExecutionProverClientV2 {
    return ABProverClientRouter.create(
      proverAConfig = config.proverA.execution,
      proverBConfig = config.proverB?.execution,
      switchBlockNumberInclusive = config.switchBlockNumberInclusive,
      switchBlockTimestamp = config.switchBlockTimestamp,
    ) { proverConfig ->
      PreRiscvExecutionProverClient(
        config = proverConfig,
        vertx = vertx,
        enableRequestFilesCleanup = config.enableRequestFilesCleanup,
      ).also { support.executionWaitingResponses.addReporter(it) }
    }
  }

  override fun preRiscvBlobCompressionProverClient(
    log: Logger,
  ): BlobCompressionProverClientV2 {
    return ABProverClientRouter.create(
      proverAConfig = requireNotNull(config.proverA.blobCompression) {
        "proverA.blobCompression must be configured to use blobCompressionProverClient"
      },
      proverBConfig = config.proverB?.blobCompression,
      switchBlockNumberInclusive = config.switchBlockNumberInclusive,
      switchBlockTimestamp = config.switchBlockTimestamp,
    ) { proverConfig ->
      PreRiscvBlobCompressionProverClient(
        config = proverConfig,
        vertx = vertx,
        enableRequestFilesCleanup = config.enableRequestFilesCleanup,
        log = log,
      )
        .also { support.blobWaitingResponses.addReporter(it) }
    }
  }

  override fun preRiscvProofAggregationProverClient(
    log: Logger,
  ): ProofAggregationProverClientV2 {
    return ABProverClientRouter.create(
      proverAConfig = config.proverA,
      proverBConfig = config.proverB,
      switchBlockNumberInclusive = config.switchBlockNumberInclusive,
      switchBlockTimestamp = config.switchBlockTimestamp,
    ) { proverConfig ->
      PreRiscvProofAggregationClient(
        config = proverConfig.proofAggregation,
        invalidityProverConfig = proverConfig.invalidity,
        vertx = vertx,
        enableRequestFilesCleanup = config.enableRequestFilesCleanup,
        log = log,
      )
        .also { support.aggregationWaitingResponses.addReporter(it) }
    }
  }

  override fun preRiscvInvalidityProverClient(): InvalidityProverClientV1 {
    if (config.proverA.invalidity == null) {
      throw IllegalStateException("Invalidity prover config is not configured")
    }

    return ABProverClientRouter.create(
      proverAConfig = config.proverA,
      proverBConfig = config.proverB,
      switchBlockNumberInclusive = config.switchBlockNumberInclusive,
      switchBlockTimestamp = config.switchBlockTimestamp,
    ) { proverConfig ->
      PreRiscvInvalidityProverClient(
        config = proverConfig.invalidity!!,
        vertx = vertx,
        enableRequestFilesCleanup = config.enableRequestFilesCleanup,
      )
        .also { support.invalidityWaitingResponses.addReporter(it) }
    }
  }

  override fun executionProverClient(): L2ExecutionProverClientV1 {
    return ABProverClientRouter.create(
      proverAConfig = config.proverA.execution,
      proverBConfig = config.proverB?.execution,
      switchBlockNumberInclusive = config.switchBlockNumberInclusive,
      switchBlockTimestamp = config.switchBlockTimestamp,
    ) { proverConfig ->
      buildL2ExecutionProverClient(proverConfig)
        .also { support.executionWaitingResponses.addReporter(it) }
    }
  }

  override fun rollupProverClient(): RollupProverClientV1 {
    return ABProverClientRouter.create(
      proverAConfig = config.proverA,
      proverBConfig = config.proverB,
      switchBlockNumberInclusive = config.switchBlockNumberInclusive,
      switchBlockTimestamp = config.switchBlockTimestamp,
    ) { proverConfig ->
      val rollupConfig = requireNotNull(proverConfig.rollup) {
        "prover.rollup must be configured to use rollupProverClient"
      }
      FileBasedRollupProverClient(
        transport = fileBasedTransport(
          config = rollupConfig,
          responseDtoClass = RollupProofResponseDto::class.java,
          fileNameProvider = RollupProofFileNameProvider,
        ),
        // The file-based request mapper inlines each execution proof by reading it back through
        // the execution transport, so it needs that transport rather than the rollup one.
        l2ExecutionProofTransport = fileBasedTransport(
          config = proverConfig.execution,
          responseDtoClass = L2ExecutionProofResponseDto::class.java,
          fileNameProvider = L2ExecutionProofFileNameProvider,
        ),
        programVk = requireNotNull(rollupConfig.programVk) {
          "programVk must be configured for the RISC-V rollup prover"
        },
        chainId = chainId.toLong(),
      ).also { support.aggregationWaitingResponses.addReporter(it) }
    }
  }

  override fun rollupAggregationProverClient(): RollupAggregationProverClientV1 {
    return ABProverClientRouter.create(
      proverAConfig = config.proverA,
      proverBConfig = config.proverB,
      switchBlockNumberInclusive = config.switchBlockNumberInclusive,
      switchBlockTimestamp = config.switchBlockTimestamp,
    ) { proverConfig ->
      val aggregationConfig = proverConfig.proofAggregation
      val rollupConfig = requireNotNull(proverConfig.rollup) {
        "prover.rollup must be configured to use rollupAggregationProverClient"
      }
      FileBasedRollupAggregationProverClient(
        transport = fileBasedTransport(
          config = aggregationConfig,
          responseDtoClass = RollupAggregationProofResponseDto::class.java,
          fileNameProvider = RollupAggregationProofFileNameProvider,
        ),
        // Aggregation inlines the rollup proofs it aggregates, read back through their transport.
        rollupProofTransport = fileBasedTransport(
          config = rollupConfig,
          responseDtoClass = RollupProofResponseDto::class.java,
          fileNameProvider = RollupProofFileNameProvider,
        ),
        programVk = requireNotNull(aggregationConfig.programVk) {
          "programVk must be configured for the RISC-V rollup-aggregation prover"
        },
      ).also { support.aggregationWaitingResponses.addReporter(it) }
    }
  }

  private fun buildL2ExecutionProverClient(proverConfig: FileBasedProverConfig): L2ExecutionProverClient {
    require(!l2MessageServiceAddress.isNullOrEmpty()) {
      "l2MessageServiceAddress must be configured for the RISC-V execution prover"
    }

    return L2ExecutionProverClient(
      transport = fileBasedTransport(
        config = proverConfig,
        responseDtoClass = L2ExecutionProofResponseDto::class.java,
        fileNameProvider = L2ExecutionProofFileNameProvider,
      ),
      programVk = requireNotNull(proverConfig.programVk) {
        "programVk must be configured for the RISC-V execution prover"
      },
      l2MessageServiceAddress = l2MessageServiceAddress,
      forkName = requireNotNull(proverConfig.forkName) {
        "forkName must be configured for the RISC-V execution prover"
      },
    )
  }

  /** A [FileBasedProverProofTransport] over [config], reading/writing [ResponseDto] JSON files. */
  private fun <RequestDto : Any, ResponseDto> fileBasedTransport(
    config: FileBasedProverConfig,
    responseDtoClass: Class<ResponseDto>,
    fileNameProvider: linea.clients.ProverFileNameProvider<BlockIntervalProofIndex>,
  ): FileBasedProverProofTransport<RequestDto, ResponseDto, BlockIntervalProofIndex> =
    FileBasedProverProofTransport(
      config = config,
      vertx = vertx,
      fileWriter = FileWriter(vertx, JsonSerialization.proofResponseMapperV1),
      fileReader = FileReader(vertx, JsonSerialization.proofResponseMapperV1, responseDtoClass),
      requestFileNameProvider = fileNameProvider,
      responseFileNameProvider = fileNameProvider,
      enableRequestFilesCleanup = this.config.enableRequestFilesCleanup,
    )
}
