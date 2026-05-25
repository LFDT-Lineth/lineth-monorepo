package linea.coordinator.app

import linea.coordinator.config.v2.CoordinatorConfig
import linea.coordinator.config.v2.toml.loadConfigs
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeAll
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.TestInstance
import java.nio.file.Path

/**
 * Guards the one failure mode of [CoordinatorConfig.toPrettyLog] that is silent: a regression
 * in the reflection walker's handling of [com.sksamuel.hoplite.Masked] or
 * [linea.coordinator.config.v2.SignerConfig.Web3jConfig] would land secrets in `kubectl logs`
 * without crashing or otherwise surfacing in normal operation.
 */
@TestInstance(TestInstance.Lifecycle.PER_CLASS)
class ConfigLoggingTest {
  private lateinit var configs: CoordinatorConfig

  @BeforeAll
  fun loadFixture() {
    configs = loadConfigs(
      coordinatorConfigFiles = listOf(
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
    )
  }

  @Test
  fun `secrets do not leak in pretty-printed config`() {
    val outputs = listOf(configs.toPrettyLog(), configs.toPrettyLog(summarizeNoisyFields = false))

    // database.password = "postgres" in the fixture; must render as ***, never plaintext.
    // Anchored on `password:` so it does not false-positive on host/username.
    outputs.forEach { yaml ->
      assertThat(yaml).doesNotContain("password: postgres")
      assertThat(yaml).contains("password: ***")
    }

    // web3j.privateKey hexes from coordinator-config-v2.toml (lines 174, 207, 241). Keep in sync if
    // those fixture keys are ever rotated — otherwise this regression check silently tests nothing.
    val privateKeyHexes = listOf(
      "5de4111afa1a4b94908f83103eb1f1706367c2e68ca870fc3fb9a804cdab365a",
      "59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d",
      "4d01ae6487860981699236a58b68f807ee5f17b12df5740b85cf4c4653be0f55",
    )
    outputs.forEach { yaml ->
      privateKeyHexes.forEach { hex ->
        assertThat(yaml).doesNotContain(hex)
        assertThat(yaml).doesNotContain(hex.uppercase())
      }
      assertThat(yaml).contains("***32 bytes***")
    }
  }
}
