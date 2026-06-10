/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package linea.security;

import static java.util.Objects.requireNonNull;

import java.util.function.Function;
import org.hyperledger.besu.plugin.services.txselection.TransactionEvaluationContext;

public class LineaChainSecurityPolicy implements ChainSecurityPolicy {
  private Function<TransactionEvaluationContext, Boolean> shallForceIncludeTransactionFn;

  public LineaChainSecurityPolicy() {}

  public void init(Function<TransactionEvaluationContext, Boolean> shallForceIncludeTransactionFn) {
    requireNonNull(
        shallForceIncludeTransactionFn, "shallForceIncludeTransactionFn must not be null");
    this.shallForceIncludeTransactionFn = shallForceIncludeTransactionFn;
  }

  @Override
  public boolean shallForceIncludeTransaction(TransactionEvaluationContext txContext) {
    return shallForceIncludeTransactionFn.apply(txContext);
  }
}
