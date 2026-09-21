package lineth.coordinator.clients.prover

import io.vertx.core.Vertx
import linea.clients.BlobCompressionProverClientV2
import linea.clients.ExecutionProverClientV2
import linea.clients.InvalidityProverClientV1
import linea.clients.ProofAggregationProverClientV2
import lineth.metrics.LineaMetricsCategory
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.metrics.micrometer.GaugeAggregator
import org.apache.logging.log4j.Logger

class PreRiscvProverClientFactory(
  private val vertx: Vertx,
  private val config: ProversConfig<PreRiscvProverConfig>,
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

  fun preRiscvExecutionProverClient(): ExecutionProverClientV2 {
    return ABProverClientRouter.create(
      proverAConfig = config.proverSwitch.current.execution,
      proverBConfig = config.proverSwitch.next?.execution,
      switchBlockNumberInclusive = config.switchBlockNumberInclusive,
      switchBlockTimestamp = config.switchBlockTimestamp,
    ) { proverConfig ->
      PreRiscvExecutionProverClient(
        config = proverConfig,
        vertx = vertx,
        enableRequestFilesCleanup = config.enableRequestFilesCleanup,
      ).also { executionWaitingResponsesMetric.addReporter(it) }
    }
  }

  fun preRiscvBlobCompressionProverClient(
    log: Logger = PreRiscvBlobCompressionProverClient.LOG,
  ): BlobCompressionProverClientV2 {
    return ABProverClientRouter.create(
      proverAConfig = config.proverSwitch.current.blobCompression,
      proverBConfig = config.proverSwitch.next?.blobCompression,
      switchBlockNumberInclusive = config.switchBlockNumberInclusive,
      switchBlockTimestamp = config.switchBlockTimestamp,
    ) { proverConfig ->
      PreRiscvBlobCompressionProverClient(
        config = proverConfig,
        vertx = vertx,
        enableRequestFilesCleanup = config.enableRequestFilesCleanup,
        log = log,
      )
        .also { blobWaitingResponsesMetric.addReporter(it) }
    }
  }

  fun preRiscvProofAggregationProverClient(
    log: Logger = PreRiscvProofAggregationClient.LOG,
  ): ProofAggregationProverClientV2 {
    return ABProverClientRouter.create(
      proverAConfig = config.proverSwitch.current,
      proverBConfig = config.proverSwitch.next,
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
        .also { aggregationWaitingResponsesMetric.addReporter(it) }
    }
  }

  fun preRiscvInvalidityProverClient(): InvalidityProverClientV1 {
    if (config.proverSwitch.current.invalidity == null) {
      throw IllegalStateException("Invalidity prover config is not configured")
    }

    return ABProverClientRouter.create(
      proverAConfig = config.proverSwitch.current,
      proverBConfig = config.proverSwitch.next,
      switchBlockNumberInclusive = config.switchBlockNumberInclusive,
      switchBlockTimestamp = config.switchBlockTimestamp,
    ) { proverConfig ->
      PreRiscvInvalidityProverClient(
        config = proverConfig.invalidity!!,
        vertx = vertx,
        enableRequestFilesCleanup = config.enableRequestFilesCleanup,
      )
        .also { invalidityWaitingResponsesMetric.addReporter(it) }
    }
  }
}
