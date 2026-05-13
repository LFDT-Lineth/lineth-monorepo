/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.app

import maru.config.MaruConfigLoader
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.core.LoggerContext
import org.apache.logging.log4j.core.config.Configurator
import picocli.CommandLine.ArgGroup
import picocli.CommandLine.Command
import picocli.CommandLine.ITypeConverter
import picocli.CommandLine.Option
import java.io.File
import java.util.concurrent.Callable

internal class KebabToEnumConverter<T : Enum<T>>(
  private val enumClass: Class<T>,
) : ITypeConverter<T> {
  override fun convert(value: String): T {
    // Convert kebab-case to upper snake-case
    val enumName = value.replace('-', '_').uppercase()
    return try {
      java.lang.Enum.valueOf(enumClass, enumName)
    } catch (_: IllegalArgumentException) {
      val validOptions = enumClass.enumConstants.joinToString(", ") { it.name.lowercase().replace('_', '-') }
      throw IllegalArgumentException(
        "Invalid value \"$value\". Expected one of: $validOptions",
      )
    }
  }
}

internal fun buildInGenesisFileResourcePath(networkNameInKebab: String) =
  "/beacon-genesis-files/$networkNameInKebab-genesis.json"

enum class Network(
  val networkNameInKebab: String,
) {
  LINEA_MAINNET("linea-mainnet"),
  LINEA_SEPOLIA("linea-sepolia"),
}

@Command(
  name = "maru",
  showDefaultValues = true,
  abbreviateSynopsis = true,
  description = ["Runs Maru consensus client"],
  version = ["0.0.1"],
  synopsisHeading = "%n",
  descriptionHeading = "%nDescription:%n%n",
  optionListHeading = "%nOptions:%n",
  footerHeading = "%n",
  mixinStandardHelpOptions = true,
)
class MaruAppCli(
  private val maruAppFactory: MaruAppFactoryCreator = MaruAppFactory(),
) : Callable<Int> {
  private val log = LogManager.getLogger(this.javaClass)

  @Option(
    names = ["--config"],
    paramLabel = "CONFIG.toml",
    description = [
      "Comma-separated list of configuration files or in multiple options" +
        " e.g. \"--config=CONFIG.toml --config=CONFIG.overrides.toml\"",
    ],
    arity = "1..*",
    split = ",",
    required = true,
  )
  val configFiles: List<File>? = null

  @ArgGroup(multiplicity = "0..1", exclusive = true, validate = true)
  var genesisOptions: GenesisOptions? = null

  class GenesisOptions(
    @Option(
      names = ["--maru-genesis-file", "--genesis-file"],
      paramLabel = "BEACON_GENESIS.json",
      description = [
        "Beacon chain genesis file (\"--maru-genesis-file\" will be deprecated soon " +
          "and replaced by \"--genesis-file\")",
      ],
      required = false,
    )
    var genesisFile: String? = null,
    @Option(
      names = ["--network"],
      paramLabel = "linea-mainnet|linea-sepolia (case-insensitive)",
      description = ["Connects to Linea mainnet or sepolia"],
      required = false,
    )
    val network: Network? = null,
  )

  override fun call(): Int {
    for (configFile in configFiles!!) {
      if (!validateFileCanRead(configFile)) {
        log.error("Failed to read config file: \"${configFile.path}\"")
        return 1
      }
    }

    // If "--genesis-file" and "--network" are not specified, default to set "--network=linea-mainnet"
    if (genesisOptions == null) {
      genesisOptions = GenesisOptions(network = Network.LINEA_MAINNET)
    }
    if (genesisOptions!!.genesisFile != null) {
      if (!validateFileCanRead(File(genesisOptions!!.genesisFile!!))) {
        log.error("Failed to read genesis file: \"${genesisOptions!!.genesisFile}\"")
        return 1
      }
      log.info("Using the given genesis file from \"${genesisOptions!!.genesisFile}\"")
    } else {
      genesisOptions!!.genesisFile = buildInGenesisFileResourcePath(genesisOptions!!.network!!.networkNameInKebab)
      log.info("Using the genesis file of the named network \"${genesisOptions!!.network!!.networkNameInKebab}\"")
    }

    val parsedAppConfig = MaruConfigLoader.loadAppConfigs(configFiles)
    val parsedBeaconGenesisConfig = MaruConfigLoader.loadGenesisConfig(genesisOptions!!.genesisFile!!)

    val app =
      maruAppFactory.create(
        config = parsedAppConfig.domainFriendly(),
        beaconGenesisConfig = parsedBeaconGenesisConfig.domainFriendly(),
      )

    Runtime
      .getRuntime()
      .addShutdownHook(
        Thread {
          app.stop().get()
          app.close()
          if (LogManager.getContext() is LoggerContext) {
            // Disable log4j auto shutdown hook is not used otherwise
            // Messages in App.stop won't appear in the logs
            Configurator.shutdown(LogManager.getContext() as LoggerContext)
          }
        },
      )

    app.start().get()
    return 0
  }

  private fun validateFileCanRead(file: File): Boolean = file.canRead()
}
