package lineth.coordinator.config.v2

import lineth.coordinator.config.v2.toml.loadConfigs
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import java.nio.file.Path
import kotlin.time.Instant

class LocalStackConfigsParsingTest {
  @Test
  fun `should keep local stack testing configs updated with the code`() {
    // Just assert that Files have been loaded and parsed correctly
    // This is to prevent Code changes in coordinator and forgetting to update config files used in the local stack
    loadConfigs(
      coordinatorConfigFiles =
      listOf(
        Path.of("../../docker/config/coordinator/coordinator-config-v2.toml"),
        Path.of("../../docker/config/coordinator/coordinator-config-v2-override-local-dev.toml"),
      ),
      tracesLimitsFileV4 = Path.of("../../docker/config/common/traces-limits-v4.4.toml"),
      tracesLimitsFileV5 = Path.of("../../docker/config/common/traces-limits-v5.toml"),
      gasPriceCapTimeOfDayMultipliersFile = Path.of(
        "../../docker/config/common/gas-price-cap-time-of-day-multipliers.toml",
      ),
      smartContractErrorsFile = Path.of("../../docker/config/common/smart-contract-errors.toml"),
      enforceStrict = true,
    ).also { configs ->
      // just small assertion to ensure that the configs are loaded and overridden correctly
      assertThat(configs.database.host).isEqualTo("127.0.0.1")
    }
  }

  @Test
  fun `should load RISC-V local stack override`() {
    loadConfigs(
      coordinatorConfigFiles =
      listOf(
        Path.of("../../docker/config/coordinator/coordinator-config-v2.toml"),
        Path.of("../../docker/config/coordinator/coordinator-config-riscv.toml"),
      ),
      tracesLimitsFileV4 = Path.of("../../docker/config/common/traces-limits-v4.4.toml"),
      tracesLimitsFileV5 = Path.of("../../docker/config/common/traces-limits-v5.toml"),
      gasPriceCapTimeOfDayMultipliersFile = Path.of(
        "../../docker/config/common/gas-price-cap-time-of-day-multipliers.toml",
      ),
      smartContractErrorsFile = Path.of("../../docker/config/common/smart-contract-errors.toml"),
      enforceStrict = true,
    ).also { configs ->
      assertThat(configs.protocol.l1.contractAddress).isEqualTo("0xCf7Ed3AccA5a467e9e704C703E8D87F634fB0Fc9")
      assertThat(configs.conflation.blocksLimit).isEqualTo(1u)
      assertThat(configs.conflation.riscvStartingBlockTimestampInclusive).isEqualTo(Instant.fromEpochSeconds(0))
      assertThat(configs.conflation.proofAggregation.timestampBasedHardForks)
        .containsExactly(Instant.fromEpochSeconds(0))
      assertThat(configs.riscvProversConfig?.proverA?.execution?.forkName).isEqualTo("Osaka")
      assertThat(configs.riscvProversConfig?.proverA?.execution?.requestsDirectory)
        .isEqualTo(Path.of("/data/prover/riscv/execution/requests"))
      assertThat(requireNotNull(configs.traces.counters).endpoints.map { it.toExternalForm() })
        .containsExactly("http://sequencer:8545/")
      assertThat(requireNotNull(configs.traces.conflation).endpoints.map { it.toExternalForm() })
        .containsExactly("http://sequencer:8545/")
      assertThat(configs.type2StateProofProvider.disabled).isTrue()
      assertThat(configs.l1Submission.disabled).isTrue()
      assertThat(configs.messageAnchoring?.disabled).isTrue()
      assertThat(configs.forcedTransactions?.disabled).isTrue()
      assertThat(configs.l2NetworkGasPricing?.disabled).isTrue()
    }
  }
}
