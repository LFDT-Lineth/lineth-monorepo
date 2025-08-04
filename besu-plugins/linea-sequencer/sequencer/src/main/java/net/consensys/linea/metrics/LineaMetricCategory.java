/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.metrics;

import java.util.Locale;
import java.util.Optional;
import org.hyperledger.besu.plugin.services.metrics.MetricCategory;

public enum LineaMetricCategory implements MetricCategory {
  /** Sequencer profitability metric category */
  SEQUENCER_PROFITABILITY,
  /** Tx pool profitability metric category */
  TX_POOL_PROFITABILITY,
  /** Runtime pricing configuration */
  PRICING_CONF,
  /** Sequencer liveness monitoring */
  SEQUENCER_LIVENESS;

  private static final Optional<String> APPLICATION_PREFIX = Optional.of("linea_");

  private final String name;

  LineaMetricCategory() {
    this.name = name().toLowerCase(Locale.ROOT);
  }

  @Override
  public String getName() {
    return name;
  }

  @Override
  public Optional<String> getApplicationPrefix() {
    return APPLICATION_PREFIX;
  }
}
