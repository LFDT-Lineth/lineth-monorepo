package linea.coordinator.config.v2

import linea.web3j.SmartContractErrors
import net.consensys.zkevm.coordinator.clients.prover.ProversConfig

data class CoordinatorConfig(
  val protocol: ProtocolConfig,
  val conflation: ConflationConfig,
  val proversConfig: ProversConfig,
  val traces: TracesConfig,
  val stateManager: StateManagerConfig,
  val type2StateProofProvider: Type2StateProofManagerConfig,
  val l1FinalizationMonitor: L1FinalizationMonitorConfig,
  val l1Submission: L1SubmissionConfig? = null,
  val forcedTransactions: ForcedTransactionsConfig? = null,
  val messageAnchoring: MessageAnchoringConfig? = null,
  val l2NetworkGasPricing: L2NetworkGasPricingConfig? = null,
  val database: DatabaseConfig,
  val api: ApiConfig,
  val smartContractErrors: SmartContractErrors,
)
