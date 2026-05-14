/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.config

import com.sksamuel.hoplite.ConfigLoaderBuilder
import com.sksamuel.hoplite.ExperimentalHoplite
import com.sksamuel.hoplite.json.JsonPropertySource
import com.sksamuel.hoplite.toml.TomlPropertySource
import maru.config.MaruConfigLoader.appTomlConfigLoaderBuilder
import maru.config.consensus.ForkConfigDecoder
import maru.config.consensus.JsonFriendlyForksSchedule
import maru.config.decoders.TomlByteArrayHexDecoder
import java.io.File
import kotlin.io.path.Path

object MaruConfigLoader {
  @OptIn(ExperimentalHoplite::class)
  fun appTomlConfigLoaderBuilder(strict: Boolean = false) =
    ConfigLoaderBuilder
      .empty()
      .addDefaults()
      .withExplicitSealedTypes()
      .addDecoder(TomlByteArrayHexDecoder())
      .apply { if (strict) this.strict() }

  @OptIn(ExperimentalHoplite::class)
  fun genesisConfigLoaderBuilder(strict: Boolean = false) =
    ConfigLoaderBuilder
      .empty()
      .addDefaults()
      .withExplicitSealedTypes()
      .addDecoder(ForkConfigDecoder)
      .apply { if (strict) this.strict() }

  @OptIn(ExperimentalHoplite::class)
  inline fun <reified T : Any> parseConfig(
    toml: String,
    strict: Boolean = false,
  ): T =
    appTomlConfigLoaderBuilder(strict)
      .addSource(TomlPropertySource(toml))
      .build()
      .loadConfigOrThrow<T>()

  fun parseBeaconChainConfig(
    json: String,
    strict: Boolean = false,
  ): JsonFriendlyForksSchedule =
    genesisConfigLoaderBuilder(strict)
      .addSource(JsonPropertySource(json))
      .build()
      .loadConfigOrThrow<JsonFriendlyForksSchedule>()

  fun loadAppConfigs(configFiles: List<File>): MaruConfigDtoToml =
    loadConfigs<MaruConfigDtoToml>(
      configFiles.map { it.toPath() },
      confLoaderBuilderFn = { strict -> appTomlConfigLoaderBuilder(strict).build() },
    )

  fun loadGenesisConfig(genesisFilePath: String): JsonFriendlyForksSchedule =
    loadConfigs<JsonFriendlyForksSchedule>(
      listOf(Path(genesisFilePath)),
      confLoaderBuilderFn = { strict -> genesisConfigLoaderBuilder(false).build() },
    )
}
