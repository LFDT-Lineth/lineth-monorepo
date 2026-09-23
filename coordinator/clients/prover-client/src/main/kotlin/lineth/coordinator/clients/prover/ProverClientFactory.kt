package lineth.coordinator.clients.prover

import io.vertx.core.Vertx
import io.vertx.core.http.HttpVersion
import io.vertx.core.http.PoolOptions
import io.vertx.ext.web.client.WebClientOptions
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
import net.consensys.linea.httprest.client.VertxHttpRestClient
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.metrics.micrometer.GaugeAggregator
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import java.net.URL

class ProverClientFactory(
  private val vertx: Vertx,
  private val preRiscvConfig: ProversConfig<PreRiscvProverConfig>? = null,
  private val config: ProversConfig<ProverConfig>? = null,
  private val chainId: Long = 0L,
  private val l2MessageServiceAddress: String? = null,
  metricsFacade: MetricsFacade,
) {
  private val l2ExecutionWaitingResponsesMetric = GaugeAggregator()
  private val rollupWaitingResponsesMetric = GaugeAggregator()
  private val rollupAggregationWaitingResponsesMetric = GaugeAggregator()
  private val executionWaitingResponsesMetric = GaugeAggregator()
  private val blobWaitingResponsesMetric = GaugeAggregator()
  private val aggregationWaitingResponsesMetric = GaugeAggregator()
  private val invalidityWaitingResponsesMetric = GaugeAggregator()

  init {
    metricsFacade.createGauge(
      category = LineaMetricsCategory.RISCV_L2_EXECUTION,
      name = "prover.waiting",
      description = "Number of RISC-V l2-execution proof waiting responses",
      measurementSupplier = l2ExecutionWaitingResponsesMetric,
    )
    metricsFacade.createGauge(
      category = LineaMetricsCategory.RISCV_ROLLUP,
      name = "prover.waiting",
      description = "Number of RISC-V rollup proof waiting responses",
      measurementSupplier = rollupWaitingResponsesMetric,
    )
    metricsFacade.createGauge(
      category = LineaMetricsCategory.RISCV_ROLLUP_AGGREGATION,
      name = "prover.waiting",
      description = "Number of RISC-V rollup-aggregation proof waiting responses",
      measurementSupplier = rollupAggregationWaitingResponsesMetric,
    )
    metricsFacade.createGauge(
      category = LineaMetricsCategory.BATCH,
      name = "prover.waiting",
      description = "Number of execution proof waiting responses",
      measurementSupplier = executionWaitingResponsesMetric,
    )
    metricsFacade.createGauge(
      category = LineaMetricsCategory.BLOB,
      name = "prover.waiting",
      description = "Number of blob compression proof waiting responses",
      measurementSupplier = blobWaitingResponsesMetric,
    )
    metricsFacade.createGauge(
      category = LineaMetricsCategory.AGGREGATION,
      name = "prover.waiting",
      description = "Number of aggregation proof waiting responses",
      measurementSupplier = aggregationWaitingResponsesMetric,
    )
    metricsFacade.createGauge(
      category = LineaMetricsCategory.FORCED_TRANSACTION,
      name = "prover.waiting",
      description = "Number of invalidity proof waiting responses",
      measurementSupplier = invalidityWaitingResponsesMetric,
    )
  }

  private fun requireRiscvConfig(): ProversConfig<ProverConfig> =
    requireNotNull(config) { "RISC-V prover config must be configured" }

  private fun requirePreRiscvConfig(): ProversConfig<PreRiscvProverConfig> =
    requireNotNull(preRiscvConfig) { "Pre RISC-V prover config must be configured" }

  fun l2ExecutionProverClient(): L2ExecutionProverClientV1 {
    val config = requireRiscvConfig()
    return ABProverClientRouter.create(
      proverAConfig = config.proverSwitch.current.l2Execution,
      proverBConfig = config.proverSwitch.next?.l2Execution,
      switchBlockNumberInclusive = config.switchBlockNumberInclusive,
      switchBlockTimestamp = config.switchBlockTimestamp,
    ) { proverConfig ->
      buildL2ExecutionProverClient(proverConfig)
        .also { l2ExecutionWaitingResponsesMetric.addReporter(it) }
    }
  }

  fun rollupProverClient(): RollupProverClientV1 {
    val config = requireRiscvConfig()
    return ABProverClientRouter.create(
      proverAConfig = config.proverSwitch.current,
      proverBConfig = config.proverSwitch.next,
      switchBlockNumberInclusive = config.switchBlockNumberInclusive,
      switchBlockTimestamp = config.switchBlockTimestamp,
    ) { proverConfig ->
      if (proverConfig.rollup.fileBased != null) {
        buildFileBasedRollupProverClient(
          proverConfig = proverConfig.rollup,
          l2ExecutionProverConfig = proverConfig.l2Execution,
        )
      } else if (proverConfig.rollup.restfulBased != null) {
        buildRestfulBasedRollupProverClient(
          proverConfig = proverConfig.rollup,
        )
      } else {
        throw IllegalStateException("fileBased and restfulBased in rollup prover config cannot be both null")
      }
        .also { rollupWaitingResponsesMetric.addReporter(it) }
    }
  }

  fun rollupAggregationProverClient(): RollupAggregationProverClientV1 {
    val config = requireRiscvConfig()
    return ABProverClientRouter.create(
      proverAConfig = config.proverSwitch.current,
      proverBConfig = config.proverSwitch.next,
      switchBlockNumberInclusive = config.switchBlockNumberInclusive,
      switchBlockTimestamp = config.switchBlockTimestamp,
    ) { proverConfig ->
      if (proverConfig.rollupAggregation.fileBased != null) {
        buildFileBasedRollupAggregationProverClient(
          proverConfig = proverConfig.rollupAggregation,
          rollupProverConfig = proverConfig.rollup,
        )
      } else if (proverConfig.rollupAggregation.restfulBased != null) {
        buildRestfulBasedRollupAggregationProverClient(
          proverConfig = proverConfig.rollupAggregation,
        )
      } else {
        throw IllegalStateException(
          "fileBased and restfulBased in rollup aggregation prover config cannot be both null",
        )
      }
        .also { rollupAggregationWaitingResponsesMetric.addReporter(it) }
    }
  }

  fun preRiscvExecutionProverClient(): ExecutionProverClientV2 {
    val preRiscvConfig = requirePreRiscvConfig()
    return ABProverClientRouter.create(
      proverAConfig = preRiscvConfig.proverSwitch.current.execution,
      proverBConfig = preRiscvConfig.proverSwitch.next?.execution,
      switchBlockNumberInclusive = preRiscvConfig.switchBlockNumberInclusive,
      switchBlockTimestamp = preRiscvConfig.switchBlockTimestamp,
    ) { proverConfig ->
      PreRiscvExecutionProverClient(
        config = proverConfig,
        vertx = vertx,
        enableRequestFilesCleanup = preRiscvConfig.enableRequestFilesCleanup,
      ).also { executionWaitingResponsesMetric.addReporter(it) }
    }
  }

  fun preRiscvBlobCompressionProverClient(
    log: Logger = PreRiscvBlobCompressionProverClient.LOG,
  ): BlobCompressionProverClientV2 {
    val preRiscvConfig = requirePreRiscvConfig()
    return ABProverClientRouter.create(
      proverAConfig = preRiscvConfig.proverSwitch.current.blobCompression,
      proverBConfig = preRiscvConfig.proverSwitch.next?.blobCompression,
      switchBlockNumberInclusive = preRiscvConfig.switchBlockNumberInclusive,
      switchBlockTimestamp = preRiscvConfig.switchBlockTimestamp,
    ) { proverConfig ->
      PreRiscvBlobCompressionProverClient(
        config = proverConfig,
        vertx = vertx,
        enableRequestFilesCleanup = preRiscvConfig.enableRequestFilesCleanup,
        log = log,
      )
        .also { blobWaitingResponsesMetric.addReporter(it) }
    }
  }

  fun preRiscvProofAggregationProverClient(
    log: Logger = PreRiscvProofAggregationClient.LOG,
  ): ProofAggregationProverClientV2 {
    val preRiscvConfig = requirePreRiscvConfig()
    return ABProverClientRouter.create(
      proverAConfig = preRiscvConfig.proverSwitch.current,
      proverBConfig = preRiscvConfig.proverSwitch.next,
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
        .also { aggregationWaitingResponsesMetric.addReporter(it) }
    }
  }

  fun preRiscvInvalidityProverClient(): InvalidityProverClientV1 {
    val preRiscvConfig = requirePreRiscvConfig()
    if (preRiscvConfig.proverSwitch.current.invalidity == null) {
      throw IllegalStateException("Invalidity prover config is not configured")
    }

    return ABProverClientRouter.create(
      proverAConfig = preRiscvConfig.proverSwitch.current,
      proverBConfig = preRiscvConfig.proverSwitch.next,
      switchBlockNumberInclusive = preRiscvConfig.switchBlockNumberInclusive,
      switchBlockTimestamp = preRiscvConfig.switchBlockTimestamp,
    ) { proverConfig ->
      PreRiscvInvalidityProverClient(
        config = proverConfig.invalidity!!,
        vertx = vertx,
        enableRequestFilesCleanup = preRiscvConfig.enableRequestFilesCleanup,
      )
        .also { invalidityWaitingResponsesMetric.addReporter(it) }
    }
  }

  private fun restClient(vertx: Vertx, endpoint: URL): VertxHttpRestClient {
    val webClientOptions = WebClientOptions()
      .setProtocolVersion(HttpVersion.HTTP_1_1)
      .setDefaultHost(endpoint.host)
      .setDefaultPort(if (endpoint.port != -1) endpoint.port else endpoint.defaultPort)
      .setSsl(endpoint.protocol == "https")
    return VertxHttpRestClient(
      webClientOptions,
      PoolOptions(),
      vertx,
      LogManager.getLogger("RestfulProverClient.restClient"),
    )
  }

  private fun <RequestDto : Any, ResponseDto> buildProofTransport(
    proverConfig: ProverClientConfig,
    requestFileNameProvider: ProverFileNameProvider<BlockIntervalProofIndex>,
    responseFileNameProvider: ProverFileNameProvider<BlockIntervalProofIndex>,
    proofType: String,
    responseDtoClass: Class<ResponseDto>,
  ): ProverProofTransport<RequestDto, ResponseDto, BlockIntervalProofIndex> {
    val transport = if (proverConfig.fileBased != null) {
      FileBasedProverProofTransport<
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
    } else if (proverConfig.restfulBased != null) {
      RestfulProverProofTransport<
        RequestDto,
        ResponseDto,
        BlockIntervalProofIndex,
        >(
        restClient = restClient(vertx, proverConfig.restfulBased.endpoint),
        vertx = vertx,
        chainId = chainId,
        proofType = proofType,
        proofStartBlockProvider = { it.startBlockNumber },
        proofEndBlockProvider = { it.endBlockNumber },
        proofHashProvider = { it.hash },
        restfulApiBasePath = proverConfig.restfulBased.restfulApiBasePath,
        restfulApiVersion = proverConfig.restfulBased.restfulApiVersion,
        responseDtoClass = responseDtoClass,
        pollingInterval = proverConfig.restfulBased.pollingInterval,
        pollingTimeout = proverConfig.restfulBased.pollingTimeout,
      )
    } else {
      throw IllegalStateException("RISC-V prover transport configuration is not configured")
    }
    return transport
  }

  private fun buildL2ExecutionProofTransport(proverConfig: ProverClientConfig) =
    buildProofTransport<L2ExecutionProofRequestDto, L2ExecutionProofResponseDto>(
      proverConfig = proverConfig,
      requestFileNameProvider = L2ExecutionProofFileNameProvider,
      responseFileNameProvider = L2ExecutionProofFileNameProvider,
      proofType = "execution",
      responseDtoClass = L2ExecutionProofResponseDto::class.java,
    )

  private fun <RequestDto : Any> buildRollupProofTransport(proverConfig: ProverClientConfig) =
    buildProofTransport<RequestDto, RollupProofResponseDto>(
      proverConfig = proverConfig,
      requestFileNameProvider = RollupProofFileNameProvider,
      responseFileNameProvider = RollupProofFileNameProvider,
      proofType = "rollup",
      responseDtoClass = RollupProofResponseDto::class.java,
    )

  private fun <RequestDto : Any> buildRollupAggregationProofTransport(proverConfig: ProverClientConfig) =
    buildProofTransport<RequestDto, RollupAggregationProofResponseDto>(
      proverConfig = proverConfig,
      requestFileNameProvider = RollupAggregationProofFileNameProvider,
      responseFileNameProvider = RollupAggregationProofFileNameProvider,
      proofType = "aggregation",
      responseDtoClass = RollupAggregationProofResponseDto::class.java,
    )

  private fun buildL2ExecutionProverClient(proverConfig: ProverClientConfig): L2ExecutionProverClient {
    require(!l2MessageServiceAddress.isNullOrEmpty()) {
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
    proverConfig: ProverClientConfig,
    l2ExecutionProverConfig: ProverClientConfig,
  ): FileBasedRollupProverClient {
    return FileBasedRollupProverClient(
      transport = buildRollupProofTransport(proverConfig),
      l2ExecutionProofTransport = buildL2ExecutionProofTransport(l2ExecutionProverConfig),
      programId = proverConfig.programId,
      provingSystemVersion = proverConfig.provingSystemVersion,
      chainId = chainId,
    )
  }

  private fun buildRestfulBasedRollupProverClient(
    proverConfig: ProverClientConfig,
  ): RestfulRollupProverClient {
    return RestfulRollupProverClient(
      transport = buildRollupProofTransport(proverConfig),
      programId = proverConfig.programId,
      provingSystemVersion = proverConfig.provingSystemVersion,
      chainId = chainId,
    )
  }

  private fun buildFileBasedRollupAggregationProverClient(
    proverConfig: ProverClientConfig,
    rollupProverConfig: ProverClientConfig,
  ): FileBasedRollupAggregationProverClient {
    return FileBasedRollupAggregationProverClient(
      transport = buildRollupAggregationProofTransport(proverConfig),
      rollupProofTransport = buildRollupProofTransport(rollupProverConfig),
      programId = proverConfig.programId,
      provingSystemVersion = proverConfig.provingSystemVersion,
    )
  }

  private fun buildRestfulBasedRollupAggregationProverClient(
    proverConfig: ProverClientConfig,
  ): RestfulRollupAggregationProverClient {
    return RestfulRollupAggregationProverClient(
      transport = buildRollupAggregationProofTransport(proverConfig),
      programId = proverConfig.programId,
      provingSystemVersion = proverConfig.provingSystemVersion,
    )
  }
}
