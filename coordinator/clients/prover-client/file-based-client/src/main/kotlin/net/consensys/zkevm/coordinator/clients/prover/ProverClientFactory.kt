package net.consensys.zkevm.coordinator.clients.prover

import io.vertx.core.Vertx
import net.consensys.linea.metrics.LineaMetricsCategory
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.metrics.micrometer.GaugeAggregator
import net.consensys.zkevm.coordinator.clients.BlobCompressionProverClientV2
import net.consensys.zkevm.coordinator.clients.ExecutionProverClientV2
import net.consensys.zkevm.coordinator.clients.InvalidityProverClientV1
import net.consensys.zkevm.coordinator.clients.ProofAggregationProverClientV2
import net.consensys.zkevm.coordinator.clients.ProverClient
import net.consensys.zkevm.domain.ProofIndex
import org.apache.logging.log4j.Logger
import kotlin.time.Instant

class ProverClientFactory(
  private val vertx: Vertx,
  private val config: ProversConfig,
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

  fun executionProverClient(log: Logger = FileBasedExecutionProverClientV2.LOG): ExecutionProverClientV2 {
    return createClient(
      proverAConfig = config.proverA.execution,
      proverBConfig = config.proverB?.execution,
      switchBlockNumberInclusive = config.switchBlockNumberInclusive,
      switchBlockTimestamp = config.switchBlockTimestamp,
    ) { proverConfig ->
      FileBasedExecutionProverClientV2(
        config = proverConfig,
        vertx = vertx,
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
      switchBlockTimestamp = config.switchBlockTimestamp,
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
      proverAConfig = config.proverA,
      proverBConfig = config.proverB,
      switchBlockNumberInclusive = config.switchBlockNumberInclusive,
      switchBlockTimestamp = config.switchBlockTimestamp,
    ) { proverConfig ->
      FileBasedProofAggregationClientV2(
        config = proverConfig.proofAggregation,
        invalidityProverConfig = proverConfig.invalidity,
        vertx = vertx,
        log = log,
      )
        .also { aggregationWaitingResponsesMetric.addReporter(it) }
    }
  }

  fun createInvalidityProofClient(): InvalidityProverClientV1 {
    if (config.proverA.invalidity == null) {
      throw IllegalStateException("Invalidity prover config is not configured")
    }

    return createClient(
      proverAConfig = config.proverA,
      proverBConfig = config.proverB,
      switchBlockNumberInclusive = config.switchBlockNumberInclusive,
      switchBlockTimestamp = config.switchBlockTimestamp,
    ) { proverConfig ->
      FileBasedInvalidityProverClient(
        config = proverConfig.invalidity!!,
        vertx = vertx,
      )
        .also { invalidityWaitingResponsesMetric.addReporter(it) }
    }
  }

  private fun <TProverConfig, ProofRequest, ProofResponse, TProofIndex> createClient(
    proverAConfig: TProverConfig,
    proverBConfig: TProverConfig?,
    switchBlockNumberInclusive: ULong?,
    switchBlockTimestamp: Instant?,
    clientBuilder: (TProverConfig) -> ProverClient<ProofRequest, ProofResponse, TProofIndex>,
  ): ProverClient<ProofRequest, ProofResponse, TProofIndex>
    where ProofRequest : Any, TProofIndex : ProofIndex {
    return when {
      switchBlockNumberInclusive != null -> {
        val switchPredicate = StartBlockNumberBasedSwitchPredicate(switchBlockNumberInclusive)
        ABProverClientRouter(
          proverA = clientBuilder(proverAConfig),
          proverB = clientBuilder(proverBConfig!!),
          switchToProverBPredicate = switchPredicate::invoke,
        )
      }
      switchBlockTimestamp != null -> {
        val switchPredicate = StartBlockTimestampBasedSwitchPredicate(switchBlockTimestamp)
        ABProverClientRouter(
          proverA = clientBuilder(proverAConfig),
          proverB = clientBuilder(proverBConfig!!),
          switchToProverBPredicate = switchPredicate::invoke,
        )
      }
      else -> clientBuilder(proverAConfig)
    }
  }
}
