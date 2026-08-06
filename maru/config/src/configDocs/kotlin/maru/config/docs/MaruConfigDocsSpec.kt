/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.config.docs

import linea.config.docs.ConfigDocsSpec
import linea.config.docs.ConfigFileRoot
import linea.config.docs.sectionByPackagePrefix
import maru.config.MaruConfigDtoToml

/**
 * Maru-specific configuration for the generic `config-docs` tooling. Lives in the
 * `configDocs` source set (compiled against the config classes but kept out of the production
 * jar) and is named from `maru/config/build.gradle` via `configDocs { spec = "…" }`.
 *
 * The root class is `MaruConfigDtoToml` — the Hoplite TOML schema `MaruConfigLoader.loadAppConfigs`
 * returns — not the domain `MaruConfig`, so the generated reference matches the keys operators
 * can actually set in `*.toml` files.
 */
object MaruConfigDocsSpec : ConfigDocsSpec {
  /** Config data classes live in this package; used to distinguish nested sections from leaves. */
  override val sectionDetector = sectionByPackagePrefix("maru.config")

  /** The Maru config file documented by this tooling, keyed by a stable label. */
  override val files: List<ConfigFileRoot> = listOf(
    ConfigFileRoot(
      label = "maru",
      description = "Main Maru configuration.",
      rootClass = MaruConfigDtoToml::class,
    ),
  )

  override val jsonSchemaPath = "docs/tech/components/maru-config-schema.json"
  override val markdownPath = "docs/tech/components/maru-config-reference.md"
  override val markdownTitle = "Maru Configuration Reference"
  override val regenerateCommand = "./gradlew :maru:config:generateConfigDocs"

  /**
   * Ephemeral MDX partial output path, relative to the repository root. Lives under the
   * `maru/config/build/` directory (gitignored) so it is never committed; the
   * `maru-config-docs` workflow uploads it as an immutable artifact and publishes only
   * `docs/stack/reference/_generated/maru/` to Consensys/doc.linea.
   */
  override val mdxPartialPath =
    "maru/config/build/config-docs-mdx/_generated/maru/reference.mdx"
}
