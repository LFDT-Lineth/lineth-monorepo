package lineth.coordinator.config.v2.docs

import linea.config.docs.ConfigDocsSpec
import linea.config.docs.ConfigFileRoot
import linea.config.docs.sectionByPackagePrefix
import lineth.coordinator.config.v2.toml.CoordinatorConfigFilesToml
import lineth.coordinator.config.v2.toml.GasPriceCapTimeOfDayMultipliersConfigFileToml
import lineth.coordinator.config.v2.toml.SmartContractErrorCodesConfigFileToml
import lineth.coordinator.config.v2.toml.TracesLimitsConfigFileV4Toml
import lineth.coordinator.config.v2.toml.TracesLimitsConfigFileV5Toml

/**
 * Coordinator-specific configuration for the generic `config-docs` tooling. Lives in the
 * `configDocs` source set (compiled against the config classes but kept out of the production
 * jar) and is named from `coordinator/config/build.gradle` via `configDocs { spec = "…" }`.
 */
object CoordinatorConfigDocsSpec : ConfigDocsSpec {
  /** Config data classes live in this package; used to distinguish nested sections from leaves. */
  override val sectionDetector = sectionByPackagePrefix("lineth.coordinator.config.v2.toml")

  /** The Coordinator config files documented by this tooling, keyed by a stable label. */
  override val files: List<ConfigFileRoot> = listOf(
    ConfigFileRoot(
      label = "coordinator",
      description = "Main Coordinator configuration.",
      rootClass = CoordinatorConfigFilesToml::class,
    ),
    ConfigFileRoot(
      label = "traces-limits-v4",
      description = "Per-module trace counter limits for v4 tracing modules.",
      rootClass = TracesLimitsConfigFileV4Toml::class,
    ),
    ConfigFileRoot(
      label = "traces-limits-v5",
      description = "Per-module trace counter limits for v5 tracing modules.",
      rootClass = TracesLimitsConfigFileV5Toml::class,
    ),
    ConfigFileRoot(
      label = "gas-price-cap-time-of-day-multipliers",
      description = "L1 dynamic gas price cap time-of-day multipliers.",
      rootClass = GasPriceCapTimeOfDayMultipliersConfigFileToml::class,
    ),
    ConfigFileRoot(
      label = "smart-contract-errors",
      description = "Optional override of the smart-contract revert error codes bundled in the coordinator jar.",
      rootClass = SmartContractErrorCodesConfigFileToml::class,
    ),
  )

  override val jsonSchemaPath = "docs/tech/components/coordinator-config-schema.json"
  override val markdownPath = "docs/tech/components/coordinator-config-reference.md"
  override val markdownTitle = "Coordinator Configuration Reference"
  override val regenerateCommand = "./gradlew :coordinator:config:generateConfigDocs"

  /**
   * Ephemeral MDX partial output path, relative to the repository root. Lives under the
   * `coordinator/config/build/` directory (gitignored) so it is never committed; the
   * `coordinator-config-docs` workflow uploads it as an immutable artifact and publishes only
   * `docs/reference/component-configuration/_generated/coordinator/` to Consensys/doc.linea.
   */
  override val mdxPartialPath =
    "coordinator/config/build/config-docs-mdx/_generated/coordinator/reference.mdx"
}
