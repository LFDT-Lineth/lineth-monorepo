package lineth.coordinator.clients.prover.riscv

import io.vertx.core.Vertx
import linea.clients.L2ExecutionProverClientV1
import linea.domain.BlockIntervalProofIndex
import lineth.coordinator.clients.prover.ABProverClientRouter
import lineth.coordinator.clients.prover.FileBasedProverConfig
import lineth.coordinator.clients.prover.ProversConfig
import lineth.coordinator.clients.prover.serialization.JsonSerialization
import lineth.fileio.FileReader
import lineth.fileio.FileWriter
import lineth.metrics.LineaMetricsCategory
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.metrics.micrometer.GaugeAggregator

class RiscvProverClientFactory(
  private val vertx: Vertx,
  private val config: ProversConfig,
  private val l2MessageServiceAddress: String,
  private val coinbase: String,
  metricsFacade: MetricsFacade,
) {
  private val executionWaitingResponsesMetric = GaugeAggregator()

  init {
    metricsFacade.createGauge(
      category = LineaMetricsCategory.BATCH,
      name = "prover.waiting",
      description = "Number of RISC-V execution proof waiting responses",
      measurementSupplier = executionWaitingResponsesMetric,
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

  private fun buildL2ExecutionProverClient(proverConfig: FileBasedProverConfig): L2ExecutionProverClient {
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
      guestProgramId = requireNotNull(proverConfig.guestProgramId) {
        "guestProgramId must be configured for the RISC-V execution prover"
      },
      l2MessageServiceAddress = l2MessageServiceAddress,
      coinbase = coinbase,
    )
  }
}
