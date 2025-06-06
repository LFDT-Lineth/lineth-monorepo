package linea.coordinator.config.v2

import linea.coordinator.config.v2.toml.loadConfigs
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import java.nio.file.Path

class LocalStackConfigsParsingTest {
  @Test
  fun `should keep local stack testing configs updated with the code`() {
    // Just assert that Files have been loaded and parsed correctly
    // This is to prevent Code changes in coordinator and forgetting to update config files used in the local stack
    loadConfigs(
      coordinatorConfigFiles = listOf(
        Path.of("../../config/coordinator/coordinator-config-v2.toml"),
        Path.of("../../config/coordinator/coordinator-config-v2-override-local-dev.toml"),
      ),
      tracesLimitsFileV2 = Path.of("../../config/common/traces-limits-v2.toml"),
      gasPriceCapTimeOfDayMultipliersFile = Path.of("../../config/common/gas-price-cap-time-of-day-multipliers.toml"),
      smartContractErrorsFile = Path.of("../../config/common/smart-contract-errors.toml"),
      enforceStrict = true,
    ).also { configs ->
      // just small assertion to ensure that the configs are loaded and overridden correctly
      assertThat(configs.database.host).isEqualTo("127.0.0.1")
    }
  }
}
