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

import static net.consensys.linea.sequencer.txselection.LineaTransactionSelectionResult.TX_GAS_EXCEEDS_USER_MAX_BLOCK_GAS;
import static net.consensys.linea.sequencer.txselection.LineaTransactionSelectionResult.TX_TOO_LARGE_FOR_REMAINING_USER_GAS;
import static org.assertj.core.api.Assertions.assertThat;
import static org.hyperledger.besu.plugin.data.TransactionSelectionResult.SELECTED;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.datatypes.PendingTransaction;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.plugin.data.ProcessableBlockHeader;
import org.hyperledger.besu.plugin.data.TransactionProcessingResult;
import org.hyperledger.besu.plugin.data.TransactionSelectionResult;
import org.hyperledger.besu.plugin.services.txselection.PluginTransactionSelector;
import org.hyperledger.besu.plugin.services.txselection.SelectorsStateManager;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

public class MaxBlockGasTransactionSelectorTest {
  private static final int MAX_GAS_PER_BLOCK = 1000;
  private static final int MAX_GAS_PER_BLOCK_20_PERCENTAGE = 200;
  private static final int MAX_GAS_PER_BLOCK_80_PERCENTAGE = 800;
  private static int seq = 1;
  private PluginTransactionSelector transactionSelector;
  private SelectorsStateManager selectorsStateManager;

  @BeforeEach
  public void initialize() {
    selectorsStateManager = new SelectorsStateManager();
    transactionSelector =
        new MaxBlockGasTransactionSelector(selectorsStateManager, MAX_GAS_PER_BLOCK);
    selectorsStateManager.blockSelectionStarted();
  }

  @Test
  public void shouldSelectWhen_GasUsedByTransaction_IsLessThan_MaxGasPerBlock() {
    final var mockTransactionProcessingResult =
        mockTransactionProcessingResult(MAX_GAS_PER_BLOCK - 1);
    final var evaluationContext = mockEvaluationContext();
    verifyTransactionSelection(
        transactionSelector, evaluationContext, mockTransactionProcessingResult, SELECTED);
  }

  @Test
  public void shouldSelectWhen_GasUsedByTransaction_IsEqual_MaxGasPerBlock() {
    final var mockTransactionProcessingResult = mockTransactionProcessingResult(MAX_GAS_PER_BLOCK);
    final var evaluationContext = mockEvaluationContext();
    verifyTransactionSelection(
        transactionSelector, evaluationContext, mockTransactionProcessingResult, SELECTED);
  }

  @Test
  public void shouldNotSelectWhen_GasUsedByTransaction_IsGreaterThan_MaxGasPerBlock() {
    final var mockTransactionProcessingResult =
        mockTransactionProcessingResult(MAX_GAS_PER_BLOCK + 1);
    final var evaluationContext = mockEvaluationContext();
    verifyTransactionSelection(
        transactionSelector,
        evaluationContext,
        mockTransactionProcessingResult,
        TX_GAS_EXCEEDS_USER_MAX_BLOCK_GAS);
  }

  @Test
  public void shouldNotSelectWhen_CumulativeGasUsed_IsGreaterThan_MaxGasPerBlock() {
    final var evaluationContext1 = mockEvaluationContext();

    // block empty, transaction 80% max gas, should select
    verifyTransactionSelection(
        transactionSelector,
        evaluationContext1,
        mockTransactionProcessingResult(MAX_GAS_PER_BLOCK_80_PERCENTAGE),
        SELECTED);

    final var evaluationContext2 = mockEvaluationContext();

    // block 80% full, transaction 80% max gas, should not select
    verifyTransactionSelection(
        transactionSelector,
        evaluationContext2,
        mockTransactionProcessingResult(MAX_GAS_PER_BLOCK_80_PERCENTAGE),
        TX_TOO_LARGE_FOR_REMAINING_USER_GAS);

    final var evaluationContext3 = mockEvaluationContext();

    // block 80% full, transaction 20% max gas, should select
    verifyTransactionSelection(
        transactionSelector,
        evaluationContext3,
        mockTransactionProcessingResult(MAX_GAS_PER_BLOCK_20_PERCENTAGE),
        SELECTED);
  }

  private void verifyTransactionSelection(
      final PluginTransactionSelector selector,
      final TestTransactionEvaluationContext evaluationContext,
      final TransactionProcessingResult processingResult,
      final TransactionSelectionResult expectedSelectionResult) {
    var selectionResult =
        selector.evaluateTransactionPostProcessing(evaluationContext, processingResult);
    assertThat(selectionResult).isEqualTo(expectedSelectionResult);
    notifySelector(selector, evaluationContext, processingResult, selectionResult);
  }

  private TestTransactionEvaluationContext mockEvaluationContext() {
    PendingTransaction pendingTransaction = mock(PendingTransaction.class);
    Transaction transaction = mock(Transaction.class);
    when(pendingTransaction.getTransaction()).thenReturn(transaction);
    when(transaction.getHash()).thenReturn(Hash.wrap(Bytes32.repeat((byte) seq++)));
    return new TestTransactionEvaluationContext(
        mock(ProcessableBlockHeader.class), pendingTransaction);
  }

  private TransactionProcessingResult mockTransactionProcessingResult(long gasUsedByTransaction) {
    TransactionProcessingResult mockTransactionProcessingResult =
        mock(TransactionProcessingResult.class);
    when(mockTransactionProcessingResult.getEstimateGasUsedByTransaction())
        .thenReturn(gasUsedByTransaction);
    return mockTransactionProcessingResult;
  }

  private void notifySelector(
      final PluginTransactionSelector selector,
      final TestTransactionEvaluationContext evaluationContext,
      final TransactionProcessingResult processingResult,
      final TransactionSelectionResult selectionResult) {
    if (selectionResult.equals(SELECTED)) {
      selector.onTransactionSelected(evaluationContext, processingResult);
    } else {
      selector.onTransactionNotSelected(evaluationContext, selectionResult);
    }
  }
}
