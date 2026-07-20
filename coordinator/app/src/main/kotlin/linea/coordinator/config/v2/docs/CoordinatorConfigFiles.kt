package linea.coordinator.config.v2.docs

import linea.coordinator.config.v2.toml.CoordinatorConfigFileToml
import linea.coordinator.config.v2.toml.GasPriceCapTimeOfDayMultipliersConfigFileToml
import linea.coordinator.config.v2.toml.SmartContractErrorCodesConfigFileToml
import linea.coordinator.config.v2.toml.TracesLimitsConfigFileV4Toml
import linea.coordinator.config.v2.toml.TracesLimitsConfigFileV5Toml
import kotlin.reflect.KClass

/**
 * A top-level Coordinator config file and its root TOML schema class.
 *
 * The coordinator loads several distinct config files. Some share the same top-level key (e.g.
 * both traces-limits files expose `traces-limits`), so generated docs and schema snapshots are
 * organised per file (keyed by [label]) rather than in a single flat path namespace.
 *
 * @property label stable identifier for the file, used as the key in generated output.
 * @property description human-readable summary of what the file configures.
 * @property rootClass the Hoplite TOML data class the file deserialises into.
 */
data class CoordinatorConfigFileRoot(
  val label: String,
  val description: String,
  val rootClass: KClass<*>,
)

/**
 * The Coordinator config files documented by this tooling — the single source of truth shared by
 * the completeness check and the docs/schema generators.
 */
val COORDINATOR_CONFIG_FILES: List<CoordinatorConfigFileRoot> = listOf(
  CoordinatorConfigFileRoot(
    label = "coordinator",
    description = "Main Coordinator configuration.",
    rootClass = CoordinatorConfigFileToml::class,
  ),
  CoordinatorConfigFileRoot(
    label = "traces-limits-v4",
    description = "Per-module trace counter limits for v4 tracing modules.",
    rootClass = TracesLimitsConfigFileV4Toml::class,
  ),
  CoordinatorConfigFileRoot(
    label = "traces-limits-v5",
    description = "Per-module trace counter limits for v5 tracing modules.",
    rootClass = TracesLimitsConfigFileV5Toml::class,
  ),
  CoordinatorConfigFileRoot(
    label = "gas-price-cap-time-of-day-multipliers",
    description = "L1 dynamic gas price cap time-of-day multipliers.",
    rootClass = GasPriceCapTimeOfDayMultipliersConfigFileToml::class,
  ),
  CoordinatorConfigFileRoot(
    label = "smart-contract-errors",
    description = "Mapping of Linea smart-contract revert error codes to messages.",
    rootClass = SmartContractErrorCodesConfigFileToml::class,
  ),
)
