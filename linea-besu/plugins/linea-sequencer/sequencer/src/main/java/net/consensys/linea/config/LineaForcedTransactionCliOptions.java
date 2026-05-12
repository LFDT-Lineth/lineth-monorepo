/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.config;

import com.google.common.base.MoreObjects;
import jakarta.validation.constraints.Positive;
import net.consensys.linea.plugins.LineaCliOptions;
import net.consensys.linea.sequencer.forced.LineaForcedTransactionPool;
import picocli.CommandLine;

/** CLI options for the forced transaction pool configuration. */
public class LineaForcedTransactionCliOptions implements LineaCliOptions {
  public static final String CONFIG_KEY = "forced-transaction-config";

  public static final String FORCED_TX_STATUS_CACHE_SIZE =
      "--plugin-linea-forced-tx-status-cache-size";

  @Positive
  @CommandLine.Option(
      names = {FORCED_TX_STATUS_CACHE_SIZE},
      hidden = true,
      paramLabel = "<INTEGER>",
      description =
          "Maximum number of forced transaction statuses to keep in cache (default: ${DEFAULT-VALUE})")
  private int statusCacheSize = LineaForcedTransactionPool.DEFAULT_STATUS_CACHE_SIZE;

  private LineaForcedTransactionCliOptions() {}

  /**
   * Create Linea Forced Transaction CLI options.
   *
   * @return the Linea Forced Transaction CLI options
   */
  public static LineaForcedTransactionCliOptions create() {
    return new LineaForcedTransactionCliOptions();
  }

  /**
   * Create Linea Forced Transaction CLI options from config.
   *
   * @param config the config
   * @return the Linea Forced Transaction CLI options
   */
  public static LineaForcedTransactionCliOptions fromConfig(
      final LineaForcedTransactionConfiguration config) {
    final LineaForcedTransactionCliOptions options = create();
    options.statusCacheSize = config.statusCacheSize();
    return options;
  }

  /**
   * Convert to domain object.
   *
   * @return the Linea Forced Transaction configuration
   */
  @Override
  public LineaForcedTransactionConfiguration toDomainObject() {
    return LineaForcedTransactionConfiguration.builder().statusCacheSize(statusCacheSize).build();
  }

  @Override
  public String toString() {
    return MoreObjects.toStringHelper(this)
        .add(FORCED_TX_STATUS_CACHE_SIZE, statusCacheSize)
        .toString();
  }
}
