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
