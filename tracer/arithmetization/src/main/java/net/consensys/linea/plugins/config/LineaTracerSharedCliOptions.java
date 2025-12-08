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

package net.consensys.linea.plugins.config;

import com.google.common.base.MoreObjects;
import net.consensys.linea.plugins.LineaCliOptions;
import picocli.CommandLine;

public class LineaTracerSharedCliOptions implements LineaCliOptions {
  public static final String CONFIG_KEY = "tracer-shared-config";

  public static final String LIMITLESS_ENABLED = "--plugin-linea-limitless-enabled";
  public static final String COUNT_HISTORICAL_BLOCKHASHES_ENABLED =
      "--plugin-linea-count-historical-blockhash-enabled";
  public static final boolean DEFAULT_LIMITLESS_ENABLED = false;
  public static final boolean DEFAULT_COUNT_HISTORICAL_BLOCKHASHES_ENABLED = false;

  @CommandLine.Option(
      names = {LIMITLESS_ENABLED},
      hidden = true,
      paramLabel = "<BOOLEAN>",
      description =
          "If the sequencer needs to use or not the limitless prover (default: ${DEFAULT-VALUE})")
  private boolean limitlessEnabled = DEFAULT_LIMITLESS_ENABLED;

  @CommandLine.Option(
      names = {COUNT_HISTORICAL_BLOCKHASHES_ENABLED},
      hidden = true,
      paramLabel = "<BOOLEAN>",
      description =
          "If the sequencer/coordinator needs to count the lines from historical blockhashes (default: ${DEFAULT-VALUE})")
  private boolean countHistoricalBlockHashes = DEFAULT_COUNT_HISTORICAL_BLOCKHASHES_ENABLED;

  private LineaTracerSharedCliOptions() {}

  /**
   * Create Linea cli options.
   *
   * @return the Linea cli options
   */
  public static LineaTracerSharedCliOptions create() {
    return new LineaTracerSharedCliOptions();
  }

  /**
   * Linea cli options from config.
   *
   * @param config the config
   * @return the Linea cli options
   */
  public static LineaTracerSharedCliOptions fromConfig(
      final LineaTracerSharedConfiguration config) {
    final LineaTracerSharedCliOptions options = create();
    options.limitlessEnabled = config.isLimitless();
    options.countHistoricalBlockHashes = config.countHistoricalBlockHashes();
    return options;
  }

  /**
   * To domain object Linea factory configuration.
   *
   * @return the Linea factory configuration
   */
  @Override
  public LineaTracerSharedConfiguration toDomainObject() {
    return LineaTracerSharedConfiguration.builder()
        .isLimitless(limitlessEnabled)
        .countHistoricalBlockHashes(countHistoricalBlockHashes)
        .build();
  }

  @Override
  public String toString() {
    return MoreObjects.toStringHelper(this)
        .add(LIMITLESS_ENABLED, limitlessEnabled)
        .add(COUNT_HISTORICAL_BLOCKHASHES_ENABLED, countHistoricalBlockHashes)
        .toString();
  }
}
