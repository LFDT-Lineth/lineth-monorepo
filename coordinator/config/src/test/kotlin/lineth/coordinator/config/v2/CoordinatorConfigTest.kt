package lineth.coordinator.config.v2

import com.sksamuel.hoplite.ConfigException
import lineth.coordinator.config.v2.toml.CoordinatorConfigFilesToml
import lineth.coordinator.config.v2.toml.parseConfig
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test

class CoordinatorConfigTest {
  @Test
  fun `should parse full configs`() {
    val toml =
      """
      ${DefaultsParsingTest.toml}
      ${ProtocolParsingTest.toml}
      ${ConflationParsingTest.toml}
      ${ProverParsingTest.toml}
      ${TracesParsingTest.toml}
      ${StateManagerParsingTest.toml}
      ${Type2StateProofProviderParsingTest.toml}
      ${L1FinalizationMonitorParsingTest.toml}
      ${L1SubmissionConfigParsingTest.toml}
      ${MessageAnchoringConfigParsingTest.toml}
      ${ForcedTransactionsConfigParsingTest.toml}
      ${L2NetWorkingGasPricingConfigParsingTest.toml}
      ${DataBaseConfigParsingTest.toml}
      ${ApiConfigParsingTest.toml}
      """.trimIndent()
    val config =
      CoordinatorConfigFilesToml(
        defaults = DefaultsParsingTest.config,
        protocol = ProtocolParsingTest.config,
        conflation = ConflationParsingTest.config,
        prover = ProverParsingTest.config,
        traces = TracesParsingTest.config,
        stateManager = StateManagerParsingTest.config,
        type2StateProofProvider = Type2StateProofProviderParsingTest.config,
        l1FinalizationMonitor = L1FinalizationMonitorParsingTest.config,
        l1Submission = L1SubmissionConfigParsingTest.config,
        messageAnchoring = MessageAnchoringConfigParsingTest.config,
        forcedTransactions = ForcedTransactionsConfigParsingTest.config,
        l2NetworkGasPricing = L2NetWorkingGasPricingConfigParsingTest.config,
        database = DataBaseConfigParsingTest.config,
        api = ApiConfigParsingTest.config,
      )
    assertThat(parseConfig<CoordinatorConfigFilesToml>(toml)).isEqualTo(config)
  }

  @Test
  fun `should parse minimal configs`() {
    val toml =
      """
      ${DefaultsParsingTest.tomlMinimal}
      ${ProtocolParsingTest.tomlMinimal}
      ${ConflationParsingTest.tomlMinimal}
      ${ProverParsingTest.tomlMinimal}
      ${TracesParsingTest.tomlMinimal}
      ${StateManagerParsingTest.tomlMinimal}
      ${Type2StateProofProviderParsingTest.tomlMinimal}
      ${L1FinalizationMonitorParsingTest.tomlMinimal}
      ${L1SubmissionConfigParsingTest.tomlMinimal}
      ${MessageAnchoringConfigParsingTest.tomlMinimal}
      ${L2NetWorkingGasPricingConfigParsingTest.tomlMinimal}
      ${DataBaseConfigParsingTest.tomlMinimal}
      ${ApiConfigParsingTest.tomlMinimal}
      """.trimIndent()
    val config =
      CoordinatorConfigFilesToml(
        defaults = DefaultsParsingTest.configMinimal,
        protocol = ProtocolParsingTest.configMinimal,
        conflation = ConflationParsingTest.configMinimal,
        prover = ProverParsingTest.configMinimal,
        traces = TracesParsingTest.configMinimal,
        stateManager = StateManagerParsingTest.configMinimal,
        type2StateProofProvider = Type2StateProofProviderParsingTest.configMinimal,
        l1FinalizationMonitor = L1FinalizationMonitorParsingTest.configMinimal,
        l1Submission = L1SubmissionConfigParsingTest.configMinimal,
        messageAnchoring = MessageAnchoringConfigParsingTest.configMinimal,
        forcedTransactions = null, // this is optional, when null will default to suitable defaults
        l2NetworkGasPricing = L2NetWorkingGasPricingConfigParsingTest.configMinimal,
        database = DataBaseConfigParsingTest.configMinimal,
        api = ApiConfigParsingTest.configMinimal,
      )

    assertThat(parseConfig<CoordinatorConfigFilesToml>(toml)).isEqualTo(config)
  }

  @Test
  fun `should fail to parse full configs if switching RISC-V block timestamps not aligned`() {
    val toml =
      """
      ${DefaultsParsingTest.toml}
      ${ProtocolParsingTest.toml}
      ${ConflationParsingTest.toml}
      ${ProverParsingTest.switchPreRiscvToRiscvToml}
      ${TracesParsingTest.toml}
      ${StateManagerParsingTest.toml}
      ${Type2StateProofProviderParsingTest.toml}
      ${L1FinalizationMonitorParsingTest.toml}
      ${L1SubmissionConfigParsingTest.toml}
      ${MessageAnchoringConfigParsingTest.toml}
      ${ForcedTransactionsConfigParsingTest.toml}
      ${L2NetWorkingGasPricingConfigParsingTest.toml}
      ${DataBaseConfigParsingTest.toml}
      ${ApiConfigParsingTest.toml}
      """.trimIndent()
    assertThatThrownBy { parseConfig<CoordinatorConfigFilesToml>(toml) }
      .isInstanceOf(ConfigException::class.java)
      .hasMessageContaining(
        "conflation.riscvStartingBlockTimestampInclusive must be equal to " +
          "prover.switchBlockTimestamp for switching from pre RISC-V to RISC-V provers",
      )
  }

  @Test
  fun `should parse full configs if switching RISC-V block timestamps aligned`() {
    val toml =
      """
      ${DefaultsParsingTest.toml}
      ${ProtocolParsingTest.toml}
      ${ConflationParsingTest.toml}
      ${ProverParsingTest.switchPreRiscvToRiscvToml.replace(
        "switch-block-timestamp = 1000",
        "switch-block-timestamp = 1758083131",
      )}
      ${TracesParsingTest.toml}
      ${StateManagerParsingTest.toml}
      ${Type2StateProofProviderParsingTest.toml}
      ${L1FinalizationMonitorParsingTest.toml}
      ${L1SubmissionConfigParsingTest.toml}
      ${MessageAnchoringConfigParsingTest.toml}
      ${ForcedTransactionsConfigParsingTest.toml}
      ${L2NetWorkingGasPricingConfigParsingTest.toml}
      ${DataBaseConfigParsingTest.toml}
      ${ApiConfigParsingTest.toml}
      """.trimIndent()
    val config =
      CoordinatorConfigFilesToml(
        defaults = DefaultsParsingTest.config,
        protocol = ProtocolParsingTest.config,
        conflation = ConflationParsingTest.config,
        prover = ProverParsingTest.switchPreRiscvToRiscvTomlConfig.copy(
          new = ProverParsingTest.switchPreRiscvToRiscvTomlConfig.new?.copy(
            switchBlockTimestamp = ConflationParsingTest.config.riscvStartingBlockTimestampInclusive,
          ),
        ),
        traces = TracesParsingTest.config,
        stateManager = StateManagerParsingTest.config,
        type2StateProofProvider = Type2StateProofProviderParsingTest.config,
        l1FinalizationMonitor = L1FinalizationMonitorParsingTest.config,
        l1Submission = L1SubmissionConfigParsingTest.config,
        messageAnchoring = MessageAnchoringConfigParsingTest.config,
        forcedTransactions = ForcedTransactionsConfigParsingTest.config,
        l2NetworkGasPricing = L2NetWorkingGasPricingConfigParsingTest.config,
        database = DataBaseConfigParsingTest.config,
        api = ApiConfigParsingTest.config,
      )
    assertThat(parseConfig<CoordinatorConfigFilesToml>(toml)).isEqualTo(config)
  }
}
