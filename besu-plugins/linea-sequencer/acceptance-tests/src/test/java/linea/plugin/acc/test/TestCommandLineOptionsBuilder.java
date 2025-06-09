/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package linea.plugin.acc.test;

import static linea.plugin.acc.test.LineaPluginTestBase.getResourcePath;

import java.util.ArrayList;
import java.util.List;
import java.util.Properties;
import org.web3j.tx.gas.DefaultGasProvider;

/** This class is used to build a list of command line options for testing. */
public class TestCommandLineOptionsBuilder {
  private final Properties cliOptions = new Properties();

  private static final String MAX_VALUE = String.valueOf(Integer.MAX_VALUE);

  public TestCommandLineOptionsBuilder() {
    cliOptions.setProperty("--plugin-linea-max-tx-calldata-size=", MAX_VALUE);
    cliOptions.setProperty("--plugin-linea-max-block-calldata-size=", MAX_VALUE);
    cliOptions.setProperty(
        "--plugin-linea-max-tx-gas-limit=", DefaultGasProvider.GAS_LIMIT.toString());
    cliOptions.setProperty("--plugin-linea-deny-list-path=", getResourcePath("/emptyDenyList.txt"));
    cliOptions.setProperty(
        "--plugin-linea-module-limit-file-path=", getResourcePath("/noModuleLimits.toml"));
    cliOptions.setProperty("--plugin-linea-max-block-gas=", MAX_VALUE);
    cliOptions.setProperty(
        "--plugin-linea-l1l2-bridge-contract=", "0x00000000000000000000000000000000DEADBEEF");
    cliOptions.setProperty("--plugin-linea-l1l2-bridge-topic=", "0x123456");
  }

  public TestCommandLineOptionsBuilder set(String option, String value) {
    cliOptions.setProperty(option, value);
    return this;
  }

  public List<String> build() {
    List<String> optionsList = new ArrayList<>(cliOptions.size());
    for (String key : cliOptions.stringPropertyNames()) {
      optionsList.add(key + cliOptions.getProperty(key));
    }
    return optionsList;
  }
}
