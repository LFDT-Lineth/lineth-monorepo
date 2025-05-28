/*
 * Copyright Consensys Software Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with
 * the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on
 * an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package net.consensys.linea.config;

import java.math.BigDecimal;

import com.google.common.base.MoreObjects;
import net.consensys.linea.plugins.LineaCliOptions;
import picocli.CommandLine;

/** The Linea RPC CLI options. */
public class LineaRpcCliOptions implements LineaCliOptions {
  public static final String CONFIG_KEY = "rpc-config-sequencer";

  private static final String ESTIMATE_GAS_COMPATIBILITY_MODE_ENABLED =
      "--plugin-linea-estimate-gas-compatibility-mode-enabled";
  private static final boolean DEFAULT_ESTIMATE_GAS_COMPATIBILITY_MODE_ENABLED = false;
  private static final String ESTIMATE_GAS_COMPATIBILITY_MODE_MULTIPLIER =
      "--plugin-linea-estimate-gas-compatibility-mode-multiplier";
  private static final BigDecimal DEFAULT_ESTIMATE_GAS_COMPATIBILITY_MODE_MULTIPLIER =
      BigDecimal.valueOf(1.2);

  @CommandLine.Option(
      names = {ESTIMATE_GAS_COMPATIBILITY_MODE_ENABLED},
      paramLabel = "<BOOLEAN>",
      description =
          "Set to true to return the min mineable gas price * multiplier, instead of the profitable price (default: ${DEFAULT-VALUE})")
  private boolean estimateGasCompatibilityModeEnabled =
      DEFAULT_ESTIMATE_GAS_COMPATIBILITY_MODE_ENABLED;

  @CommandLine.Option(
      names = {ESTIMATE_GAS_COMPATIBILITY_MODE_MULTIPLIER},
      paramLabel = "<FLOAT>",
      description =
          "Set to multiplier to apply to the min priority fee per gas when the compatibility mode is enabled (default: ${DEFAULT-VALUE})")
  private BigDecimal estimateGasCompatibilityMultiplier =
      DEFAULT_ESTIMATE_GAS_COMPATIBILITY_MODE_MULTIPLIER;

  private LineaRpcCliOptions() {}

  /**
   * Create Linea RPC CLI options.
   *
   * @return the Linea RPC CLI options
   */
  public static LineaRpcCliOptions create() {
    return new LineaRpcCliOptions();
  }

  /**
   * Linea RPC CLI options from config.
   *
   * @param config the config
   * @return the Linea RPC CLI options
   */
  public static LineaRpcCliOptions fromConfig(final LineaRpcConfiguration config) {
    final LineaRpcCliOptions options = create();
    options.estimateGasCompatibilityModeEnabled = config.estimateGasCompatibilityModeEnabled();
    options.estimateGasCompatibilityMultiplier = config.estimateGasCompatibilityMultiplier();
    return options;
  }

  /**
   * To domain object Linea factory configuration.
   *
   * @return the Linea factory configuration
   */
  @Override
  public LineaRpcConfiguration toDomainObject() {
    return LineaRpcConfiguration.builder()
        .estimateGasCompatibilityModeEnabled(estimateGasCompatibilityModeEnabled)
        .estimateGasCompatibilityMultiplier(estimateGasCompatibilityMultiplier)
        .build();
  }

  @Override
  public String toString() {
    return MoreObjects.toStringHelper(this)
        .add(ESTIMATE_GAS_COMPATIBILITY_MODE_ENABLED, estimateGasCompatibilityModeEnabled)
        .add(ESTIMATE_GAS_COMPATIBILITY_MODE_MULTIPLIER, estimateGasCompatibilityMultiplier)
        .toString();
  }
}
