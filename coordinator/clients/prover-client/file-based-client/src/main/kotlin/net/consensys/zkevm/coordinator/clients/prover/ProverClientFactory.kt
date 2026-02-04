package net.consensys.zkevm.coordinator.clients.prover

import io.vertx.core.Vertx
import net.consensys.linea.metrics.LineaMetricsCategory
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.metrics.micrometer.GaugeAggregator
import net.consensys.zkevm.coordinator.clients.BlobCompressionProverClientV2
import net.consensys.zkevm.coordinator.clients.ExecutionProverClientV2
import net.consensys.zkevm.coordinator.clients.ProofAggregationProverClientV2
import net.consensys.zkevm.coordinator.clients.ProverClient
import net.consensys.zkevm.domain.ProofIndex
import org.apache.logging.log4j.Logger

class ProverClientFactory(
  private val vertx: Vertx,
  private val config: ProversConfig,
  metricsFacade: MetricsFacade,
) {
  private val executionWaitingResponsesMetric = GaugeAggregator()
  private val blobWaitingResponsesMetric = GaugeAggregator()
  private val aggregationWaitingResponsesMetric = GaugeAggregator()

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
  }

  fun executionProverClient(
    tracesVersion: String,
    stateManagerVersion: String,
    log: Logger = FileBasedExecutionProverClientV2.LOG,
  ): ExecutionProverClientV2 {
    return createClient(
      proverAConfig = config.proverA.execution,
      proverBConfig = config.proverB?.execution,
      switchBlockNumberInclusive = config.switchBlockNumberInclusive,
    ) { proverConfig ->
      FileBasedExecutionProverClientV2(
        config = proverConfig,
        vertx = vertx,
        tracesVersion = tracesVersion,
        stateManagerVersion = stateManagerVersion,
        log = log,
      ).also { executionWaitingResponsesMetric.addReporter(it) }
    }
  }

  fun blobCompressionProverClient(
    log: Logger = FileBasedBlobCompressionProverClientV2.LOG,
  ): BlobCompressionProverClientV2 {
    return createClient(
      proverAConfig = config.proverA.blobCompression,
      proverBConfig = config.proverB?.blobCompression,
      switchBlockNumberInclusive = config.switchBlockNumberInclusive,
    ) { proverConfig ->
      FileBasedBlobCompressionProverClientV2(
        config = proverConfig,
        vertx = vertx,
        log = log,
      )
        .also { blobWaitingResponsesMetric.addReporter(it) }
    }
  }

  fun proofAggregationProverClient(
    log: Logger = FileBasedProofAggregationClientV2.LOG,
  ): ProofAggregationProverClientV2 {
    return createClient(
      proverAConfig = config.proverA.proofAggregation,
      proverBConfig = config.proverB?.proofAggregation,
      switchBlockNumberInclusive = config.switchBlockNumberInclusive,
    ) { proverConfig ->
      FileBasedProofAggregationClientV2(
        config = proverConfig,
        vertx = vertx,
        log = log,
      )
        .also { aggregationWaitingResponsesMetric.addReporter(it) }
    }
  }

  private fun <ProofRequest, ProofResponse, TProofIndex> createClient(
    proverAConfig: FileBasedProverConfig,
    proverBConfig: FileBasedProverConfig?,
    switchBlockNumberInclusive: ULong?,
    clientBuilder: (FileBasedProverConfig) -> ProverClient<ProofRequest, ProofResponse, TProofIndex>,
  ): ProverClient<ProofRequest, ProofResponse, TProofIndex>
    where ProofRequest : Any, TProofIndex : ProofIndex {
    return if (switchBlockNumberInclusive != null) {
      val switchPredicate = StartBlockNumberBasedSwitchPredicate(switchBlockNumberInclusive)
      ABProverClientRouter(
        proverA = clientBuilder(proverAConfig),
        proverB = clientBuilder(proverBConfig!!),
        switchToProverBPredicate = switchPredicate::invoke,
      )
    } else {
      clientBuilder(proverAConfig)
    }
  }
}
