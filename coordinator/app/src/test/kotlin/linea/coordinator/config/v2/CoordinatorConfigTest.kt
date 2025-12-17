package linea.coordinator.config.v2

import linea.coordinator.config.v2.toml.CoordinatorConfigFileToml
import linea.coordinator.config.v2.toml.parseConfig
import org.assertj.core.api.Assertions.assertThat
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
      ${L2NetWorkingGasPricingConfigParsingTest.toml}
      ${DataBaseConfigParsingTest.toml}
      ${ApiConfigParsingTest.toml}
      """.trimIndent()
    val config =
      CoordinatorConfigFileToml(
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
        l2NetworkGasPricing = L2NetWorkingGasPricingConfigParsingTest.config,
        database = DataBaseConfigParsingTest.config,
        api = ApiConfigParsingTest.config,
      )
    assertThat(parseConfig<CoordinatorConfigFileToml>(toml)).isEqualTo(config)
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
      CoordinatorConfigFileToml(
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
        l2NetworkGasPricing = L2NetWorkingGasPricingConfigParsingTest.configMinimal,
        database = DataBaseConfigParsingTest.configMinimal,
        api = ApiConfigParsingTest.configMinimal,
      )

    assertThat(parseConfig<CoordinatorConfigFileToml>(toml)).isEqualTo(config)
  }
}
