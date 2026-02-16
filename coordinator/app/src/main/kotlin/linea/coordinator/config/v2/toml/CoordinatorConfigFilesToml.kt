package linea.coordinator.config.v2.toml

import linea.coordinator.config.v2.CoordinatorConfig
import linea.web3j.SmartContractErrors
import net.consensys.linea.ethereum.gaspricing.dynamiccap.TimeOfDayMultipliers
import net.consensys.linea.traces.TracesCountersV2
import net.consensys.linea.traces.TracesCountersV4
import net.consensys.linea.traces.TracesCountersV5
import net.consensys.linea.traces.TracingModuleV2
import net.consensys.linea.traces.TracingModuleV4
import net.consensys.linea.traces.TracingModuleV5

data class CoordinatorConfigFileToml(
  val defaults: DefaultsToml = DefaultsToml(),
  val protocol: ProtocolToml,
  val conflation: ConflationToml = ConflationToml(),
  val prover: ProverToml,
  val traces: TracesToml,
  val stateManager: StateManagerToml,
  val type2StateProofProvider: Type2StateProofManagerToml,
  val l1FinalizationMonitor: L1FinalizationMonitorConfigToml,
  val l1Submission: L1SubmissionConfigToml,
  val forcedTransactions: ForcedTransactionsConfigToml? = null,
  val messageAnchoring: MessageAnchoringConfigToml,
  val l2NetworkGasPricing: L2NetworkGasPricingConfigToml,
  val database: DatabaseToml,
  val api: ApiConfigToml = ApiConfigToml(),
)

data class TracesLimitsConfigFileV2Toml(
  val tracesLimits: Map<TracingModuleV2, UInt>,
)

data class TracesLimitsConfigFileV4Toml(
  val tracesLimits: Map<TracingModuleV4, UInt>,
)

data class TracesLimitsConfigFileV5Toml(
  val tracesLimits: Map<TracingModuleV5, UInt>,
)

data class GasPriceCapTimeOfDayMultipliersConfigFileToml(
  val gasPriceCapTimeOfDayMultipliers: TimeOfDayMultipliers,
)

data class SmartContractErrorCodesConfigFileToml(val smartContractErrors: SmartContractErrors)

data class CoordinatorConfigToml(
  val configs: CoordinatorConfigFileToml,
  val tracesLimitsV2: TracesLimitsConfigFileV2Toml?,
  val tracesLimitsV4: TracesLimitsConfigFileV4Toml?,
  val tracesLimitsV5: TracesLimitsConfigFileV5Toml?,
  val l1DynamicGasPriceCapTimeOfDayMultipliers: GasPriceCapTimeOfDayMultipliersConfigFileToml? = null,
  val smartContractErrors: SmartContractErrorCodesConfigFileToml? = null,
) {
  fun reified(): CoordinatorConfig {
    return CoordinatorConfig(
      protocol = configs.protocol.reified(),
      conflation =
      configs.conflation.reified(
        defaults = configs.defaults,
        tracesCountersLimitsV2 = tracesLimitsV2?.let { TracesCountersV2(it.tracesLimits) },
        tracesCountersLimitsV4 = tracesLimitsV4?.let { TracesCountersV4(it.tracesLimits) },
        tracesCountersLimitsV5 = tracesLimitsV5?.let { TracesCountersV5(it.tracesLimits) },
      ),
      proversConfig = this.configs.prover.reified(),
      traces = this.configs.traces.reified(),
      stateManager = this.configs.stateManager.reified(),
      type2StateProofProvider = this.configs.type2StateProofProvider.reified(),
      l1FinalizationMonitor =
      this.configs.l1FinalizationMonitor.reified(
        defaults = this.configs.defaults,
      ),
      l1Submission =
      this.configs.l1Submission.reified(
        l1DefaultEndpoint = this.configs.defaults.l1Endpoint,
        l1DefaultRequestRetries = this.configs.defaults.l1RequestRetries,
        timeOfDayMultipliers =
        l1DynamicGasPriceCapTimeOfDayMultipliers
          ?.gasPriceCapTimeOfDayMultipliers
          ?: emptyMap(),
      ),
      forcedTransactions =
      this.configs.forcedTransactions?.reified(
        l1DefaultEndpoint = this.configs.defaults.l1Endpoint,
        l1DefaultRequestRetries = this.configs.defaults.l1RequestRetries,
      ) ?: ForcedTransactionsConfigToml().reified(
        l1DefaultEndpoint = this.configs.defaults.l1Endpoint,
        l1DefaultRequestRetries = this.configs.defaults.l1RequestRetries,
      ),
      messageAnchoring =
      this.configs.messageAnchoring.reified(
        l1DefaultEndpoint = this.configs.defaults.l1Endpoint,
        l2DefaultEndpoint = this.configs.defaults.l2Endpoint,
      ),
      l2NetworkGasPricing =
      this.configs.l2NetworkGasPricing.reified(defaults = this.configs.defaults),
      database = this.configs.database.reified(),
      api = this.configs.api.reified(),
      smartContractErrors = smartContractErrors?.smartContractErrors ?: emptyMap(),
    )
  }
}
