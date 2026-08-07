package linea.coordinator.config.v2.toml

import linea.coordinator.config.v2.CoordinatorConfig
import lineth.config.docs.ConfigDoc
import lineth.config.docs.ConfigSection
import lineth.traces.TracesCountersV4
import lineth.traces.TracesCountersV5
import lineth.traces.TracingModuleV4
import lineth.traces.TracingModuleV5
import lineth.web3j.SmartContractErrors
import net.consensys.linea.ethereum.gaspricing.dynamiccap.TimeOfDayMultipliers

data class CoordinatorConfigFileToml(
  @param:ConfigSection("Shared defaults (L1/L2 endpoints and retry policies) reused by coordinator services.")
  val defaults: DefaultsToml = DefaultsToml(),
  @param:ConfigSection("Lineth protocol contract addresses and genesis settings.")
  val protocol: ProtocolToml,
  @param:ConfigSection("Block conflation, blob compression, and proof aggregation settings.")
  val conflation: ConflationToml = ConflationToml(),
  @param:ConfigSection("File-based prover request/response directories and switch-over settings.")
  val prover: ProverToml,
  @param:ConfigSection("Trace generation (traces API / conflation counters) client settings.")
  val traces: TracesToml,
  @param:ConfigSection("Shomei state manager client settings.")
  val stateManager: StateManagerToml,
  @param:ConfigSection("Type-2 state proof provider (shnarf/state proof) settings.")
  val type2StateProofProvider: Type2StateProofManagerToml,
  @param:ConfigSection("L1 finalization monitor polling settings.")
  val l1FinalizationMonitor: L1FinalizationMonitorConfigToml,
  @param:ConfigSection("L1 blob/aggregation submission (data availability and finalization) settings.")
  val l1Submission: L1SubmissionConfigToml,
  @param:ConfigSection("Forced transactions handling settings; omit the section to disable the feature.")
  val forcedTransactions: ForcedTransactionsConfigToml? = null,
  @param:ConfigSection("L1 to L2 message anchoring settings.")
  val messageAnchoring: MessageAnchoringConfigToml,
  @param:ConfigSection("L2 network gas pricing (dynamic gas price publishing) settings.")
  val l2NetworkGasPricing: L2NetworkGasPricingConfigToml,
  @param:ConfigSection("Coordinator PostgreSQL persistence settings.")
  val database: DatabaseToml,
  @param:ConfigSection("Coordinator JSON-RPC and observability API settings.")
  val api: ApiConfigToml = ApiConfigToml(),
)

data class TracesLimitsConfigFileV4Toml(
  @param:ConfigDoc(
    description = "Per-module trace counter limits (v4 tracing modules). Each entry maps a " +
      "tracing module name to its maximum trace count.",
  )
  val tracesLimits: Map<TracingModuleV4, UInt>,
)

data class TracesLimitsConfigFileV5Toml(
  @param:ConfigDoc(
    description = "Per-module trace counter limits (v5 tracing modules). Each entry maps a " +
      "tracing module name to its maximum trace count.",
  )
  val tracesLimits: Map<TracingModuleV5, UInt>,
)

data class GasPriceCapTimeOfDayMultipliersConfigFileToml(
  @param:ConfigDoc(
    description = "L1 dynamic gas price cap multipliers keyed by time-of-day/day-of-week slot; " +
      "each entry scales the base gas price cap for that slot.",
  )
  val gasPriceCapTimeOfDayMultipliers: TimeOfDayMultipliers,
)

data class SmartContractErrorCodesConfigFileToml(
  @param:ConfigDoc(
    description = "Mapping of Lineth smart-contract revert error codes to human-readable messages, " +
      "used to decode on-chain rejection reasons.",
  )
  val smartContractErrors: SmartContractErrors,
)

data class CoordinatorConfigToml(
  val configs: CoordinatorConfigFileToml,
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
      this.configs.forcedTransactions?.reified(defaults = this.configs.defaults),
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
