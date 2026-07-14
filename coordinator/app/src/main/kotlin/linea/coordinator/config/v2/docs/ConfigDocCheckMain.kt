package linea.coordinator.config.v2.docs

import linea.coordinator.config.v2.toml.CoordinatorConfigFileToml
import linea.coordinator.config.v2.toml.GasPriceCapTimeOfDayMultipliersConfigFileToml
import linea.coordinator.config.v2.toml.SmartContractErrorCodesConfigFileToml
import linea.coordinator.config.v2.toml.TracesLimitsConfigFileV4Toml
import linea.coordinator.config.v2.toml.TracesLimitsConfigFileV5Toml
import kotlin.reflect.KClass
import kotlin.system.exitProcess

/**
 * Root Coordinator TOML config classes documented by this tooling. Each corresponds to one of
 * the config files the coordinator loads.
 */
val COORDINATOR_CONFIG_ROOTS: List<KClass<*>> = listOf(
  CoordinatorConfigFileToml::class,
  TracesLimitsConfigFileV4Toml::class,
  TracesLimitsConfigFileV5Toml::class,
  GasPriceCapTimeOfDayMultipliersConfigFileToml::class,
  SmartContractErrorCodesConfigFileToml::class,
)

/**
 * Entry point for the `checkCoordinatorConfigDocs` Gradle task. Walks the root config classes,
 * validates that every key is documented, prints any violations, and exits non-zero on failure
 * so the Gradle task (and CI) fails.
 */
object ConfigDocCheckMain {
  @JvmStatic
  fun main(args: Array<String>) {
    val keys = ConfigSchemaWalker.walkAll(COORDINATOR_CONFIG_ROOTS)
    val violations = ConfigDocValidator.validate(keys)
    if (violations.isEmpty()) {
      println("Coordinator config docs OK: ${keys.size} keys, all documented.")
      return
    }
    System.err.println(ConfigDocValidator.formatViolations(violations))
    exitProcess(1)
  }
}
