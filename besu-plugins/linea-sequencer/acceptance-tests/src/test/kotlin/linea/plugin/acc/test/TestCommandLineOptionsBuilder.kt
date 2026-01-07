/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package linea.plugin.acc.test

import linea.plugin.acc.test.LineaPluginTestBase.Companion.getResourcePath
import org.web3j.tx.gas.DefaultGasProvider
import java.util.Properties

/** This class is used to build a list of command line options for testing. */
class TestCommandLineOptionsBuilder {
  private val cliOptions = Properties()

  init {
    cliOptions.setProperty("--plugin-linea-max-tx-calldata-size=", MAX_VALUE)
    cliOptions.setProperty("--plugin-linea-max-block-calldata-size=", MAX_VALUE)
    cliOptions.setProperty(
      "--plugin-linea-max-tx-gas-limit=",
      DefaultGasProvider.GAS_LIMIT.toString(),
    )
    cliOptions.setProperty("--plugin-linea-deny-list-path=", getResourcePath("/emptyDenyList.txt"))
    cliOptions.setProperty(
      "--plugin-linea-module-limit-file-path=",
      getResourcePath("/noModuleLimits.toml"),
    )
    cliOptions.setProperty("--plugin-linea-max-block-gas=", MAX_VALUE)
    cliOptions.setProperty(
      "--plugin-linea-l1l2-bridge-contract=",
      "0x00000000000000000000000000000000DEADBEEF",
    )
    cliOptions.setProperty(
      "--plugin-linea-l1l2-bridge-topic=",
      "0x1234567812345678123456781234567812345678123456781234567812345678",
    )
  }

  fun set(option: String, value: String): TestCommandLineOptionsBuilder {
    cliOptions.setProperty(option, value)
    return this
  }

  fun build(): List<String> {
    val optionsList = ArrayList<String>(cliOptions.size)
    for (key in cliOptions.stringPropertyNames()) {
      optionsList.add(key + cliOptions.getProperty(key))
    }
    return optionsList
  }

  companion object {
    private val MAX_VALUE = Integer.MAX_VALUE.toString()
  }
}
