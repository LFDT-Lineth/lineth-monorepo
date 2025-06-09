/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.sequencer.txselection.selectors;

import com.google.common.base.Stopwatch;
import org.hyperledger.besu.datatypes.PendingTransaction;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.plugin.data.ProcessableBlockHeader;
import org.hyperledger.besu.plugin.services.txselection.TransactionEvaluationContext;

public class TestTransactionEvaluationContext implements TransactionEvaluationContext {
  private ProcessableBlockHeader processableBlockHeader;
  private PendingTransaction pendingTransaction;
  private Wei transactionGasPrice;
  private Wei minGasPrice;

  public TestTransactionEvaluationContext(
      final ProcessableBlockHeader processableBlockHeader,
      final PendingTransaction pendingTransaction,
      final Wei transactionGasPrice,
      final Wei minGasPrice) {
    this.processableBlockHeader = processableBlockHeader;
    this.pendingTransaction = pendingTransaction;
    this.transactionGasPrice = transactionGasPrice;
    this.minGasPrice = minGasPrice;
  }

  public TestTransactionEvaluationContext(
      final ProcessableBlockHeader processableBlockHeader,
      final PendingTransaction pendingTransaction) {
    this(processableBlockHeader, pendingTransaction, Wei.ONE, Wei.ONE);
  }

  @Override
  public ProcessableBlockHeader getPendingBlockHeader() {
    return processableBlockHeader;
  }

  @Override
  public PendingTransaction getPendingTransaction() {
    return pendingTransaction;
  }

  @Override
  public Stopwatch getEvaluationTimer() {
    return Stopwatch.createStarted();
  }

  @Override
  public Wei getTransactionGasPrice() {
    return transactionGasPrice;
  }

  @Override
  public Wei getMinGasPrice() {
    return minGasPrice;
  }

  public TestTransactionEvaluationContext setMinGasPrice(final Wei minGasPrice) {
    this.minGasPrice = minGasPrice;
    return this;
  }
}
