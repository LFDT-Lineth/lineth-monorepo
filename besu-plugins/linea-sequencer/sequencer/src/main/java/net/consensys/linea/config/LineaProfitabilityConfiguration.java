/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */

package net.consensys.linea.config;

import lombok.Builder;
import lombok.Getter;
import lombok.ToString;
import lombok.experimental.Accessors;
import net.consensys.linea.plugins.LineaOptionsConfiguration;

/** The Linea profitability calculator configuration. */
@Builder(toBuilder = true)
@Accessors(fluent = true)
@Getter
@ToString
public class LineaProfitabilityConfiguration implements LineaOptionsConfiguration {
  /** It is safe to keep this as long, since it will store value <= max_int * 1000 */
  private long fixedCostWei;

  /** It is safe to keep this as long, since it will store value <= max_int * 1000 */
  private long variableCostWei;

  /** It is safe to keep this as long, since it will store value <= max_int * 1000 */
  private long ethGasPriceWei;

  private double minMargin;
  private double estimateGasMinMargin;
  private double txPoolMinMargin;
  private boolean txPoolCheckApiEnabled;
  private boolean txPoolCheckP2pEnabled;
  private boolean extraDataPricingEnabled;
  private boolean extraDataSetMinGasPriceEnabled;
  private double[] profitabilityMetricsBuckets;

  /**
   * These 2 parameters must be atomically updated
   *
   * @param fixedCostWei fixed cost in Wei
   * @param variableCostWei variable cost in Wei
   * @param ethGasPriceWei gas price in Wei
   */
  public synchronized void updateFixedVariableAndGasPrice(
      final long fixedCostWei, final long variableCostWei, final long ethGasPriceWei) {
    this.fixedCostWei = fixedCostWei;
    this.variableCostWei = variableCostWei;
    this.ethGasPriceWei = ethGasPriceWei;
  }

  public synchronized long fixedCostWei() {
    return fixedCostWei;
  }

  public synchronized long variableCostWei() {
    return variableCostWei;
  }

  public synchronized long ethGasPriceWei() {
    return ethGasPriceWei;
  }
}
