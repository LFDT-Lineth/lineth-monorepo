package lineth.coordinator.clients.prover.riscv

import io.vertx.core.Vertx
import linea.clients.BlobCompressionProverClientV2
import linea.clients.ExecutionProverClientV2
import linea.clients.InvalidityProverClientV1
import linea.clients.L2ExecutionProverClientV1
import linea.clients.ProofAggregationProverClientV2
import linea.domain.BlockIntervalProofIndex
import lineth.coordinator.clients.prover.serialization.JsonSerialization
import lineth.fileio.FileReader
import lineth.fileio.FileWriter
import lineth.metrics.LineaMetricsCategory
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.metrics.micrometer.GaugeAggregator
import org.apache.logging.log4j.Logger

class ProverClientFactory(
  private val vertx: Vertx,
  private val config: ProversConfig,
  private val l2MessageServiceAddress: String? = null,
  metricsFacade: MetricsFacade,
) {
  private val executionWaitingResponsesMetric = GaugeAggregator()
  private val blobWaitingResponsesMetric = GaugeAggregator()
  private val aggregationWaitingResponsesMetric = GaugeAggregator()
  private val invalidityWaitingResponsesMetric = GaugeAggregator()

  init {
    metricsFacade.createGauge(
      category = LineaMetricsCategory.BATCH,
      name = "prover.waiting",
      description = "Number of RISC-V execution proof waiting responses",
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

  fun executionProverClient(): L2ExecutionProverClientV1 {
    return ABProverClientRouter.create(
      proverAConfig = config.proverA.execution,
      proverBConfig = config.proverB?.execution,
      switchBlockNumberInclusive = config.switchBlockNumberInclusive,
      switchBlockTimestamp = config.switchBlockTimestamp,
    ) { proverConfig ->
      buildL2ExecutionProverClient(proverConfig)
        .also { executionWaitingResponsesMetric.addReporter(it) }
    }
  }

  fun preRiscvExecutionProverClient(): ExecutionProverClientV2 {
    return ABProverClientRouter.create(
      proverAConfig = config.proverA.execution,
      proverBConfig = config.proverB?.execution,
      switchBlockNumberInclusive = config.switchBlockNumberInclusive,
      switchBlockTimestamp = config.switchBlockTimestamp,
    ) { proverConfig ->
      PreRiscvExecutionProverClient(
        config = proverConfig,
        vertx = vertx,
      ).also { executionWaitingResponsesMetric.addReporter(it) }
    }
  }

  fun preRiscvBlobCompressionProverClient(
    log: Logger = PreRiscvBlobCompressionProverClient.LOG,
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
        log = log,
      )
        .also { blobWaitingResponsesMetric.addReporter(it) }
    }
  }

  fun preRiscvProofAggregationProverClient(
    log: Logger = PreRiscvProofAggregationClient.LOG,
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
        log = log,
      )
        .also { aggregationWaitingResponsesMetric.addReporter(it) }
    }
  }

  fun preRiscvInvalidityProverClient(): InvalidityProverClientV1 {
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
      )
        .also { invalidityWaitingResponsesMetric.addReporter(it) }
    }
  }

  private fun buildL2ExecutionProverClient(proverConfig: FileBasedProverConfig): L2ExecutionProverClient {
    require(!l2MessageServiceAddress.isNullOrEmpty()) {
      "l2MessageServiceAddress must be configured for the RISC-V execution prover"
    }

    val transport = FileBasedProverProofTransport<
      L2ExecutionProofRequestDto,
      L2ExecutionProofResponseDto,
      BlockIntervalProofIndex,
      >(
      config = proverConfig,
      vertx = vertx,
      fileWriter = FileWriter(vertx, JsonSerialization.proofResponseMapperV1),
      fileReader = FileReader(
        vertx,
        JsonSerialization.proofResponseMapperV1,
        L2ExecutionProofResponseDto::class.java,
      ),
      requestFileNameProvider = L2ExecutionProofFileNameProvider,
      responseFileNameProvider = L2ExecutionProofFileNameProvider,
    )
    return L2ExecutionProverClient(
      transport = transport,
      programVk = requireNotNull(proverConfig.programVk) {
        "programVk must be configured for the RISC-V execution prover"
      },
      l2MessageServiceAddress = l2MessageServiceAddress,
      forkName = requireNotNull(proverConfig.forkName) {
        "forkName must be configured for the RISC-V execution prover"
      },
    )
  }
}
