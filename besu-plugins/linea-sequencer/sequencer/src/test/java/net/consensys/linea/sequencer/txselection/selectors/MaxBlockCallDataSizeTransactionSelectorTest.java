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

import static net.consensys.linea.sequencer.txselection.LineaTransactionSelectionResult.BLOCK_CALLDATA_OVERFLOW;
import static org.assertj.core.api.Assertions.assertThat;
import static org.hyperledger.besu.plugin.data.TransactionSelectionResult.SELECTED;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

import org.apache.tuweni.bytes.Bytes;
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

public class MaxBlockCallDataSizeTransactionSelectorTest {
  private static final int BLOCK_CALL_DATA_MAX_SIZE = 100;
  private static final int BLOCK_CALL_DATA_HALF_SIZE = 50;
  private static final int TX_CALL_DATA_SIZE = BLOCK_CALL_DATA_HALF_SIZE + 1;
  private static int seq = 1;
  private PluginTransactionSelector transactionSelector;
  private SelectorsStateManager selectorsStateManager;

  @BeforeEach
  public void initialize() {
    selectorsStateManager = new SelectorsStateManager();
    transactionSelector =
        new MaxBlockCallDataTransactionSelector(selectorsStateManager, BLOCK_CALL_DATA_MAX_SIZE);
    selectorsStateManager.blockSelectionStarted();
  }

  @Test
  public void shouldSelectTransactionWhen_BlockCallDataSize_IsLessThan_MaxBlockCallDataSize() {
    final var evaluationContext1 = mockTransactionOfCallDataSize(BLOCK_CALL_DATA_HALF_SIZE);
    verifyTransactionSelection(transactionSelector, evaluationContext1, SELECTED);

    final var evaluationContext2 = mockTransactionOfCallDataSize(BLOCK_CALL_DATA_HALF_SIZE - 1);
    verifyTransactionSelection(transactionSelector, evaluationContext2, SELECTED);
  }

  @Test
  public void shouldSelectTransactionWhen_BlockCallDataSize_IsEqualTo_MaxBlockCallDataSize() {
    final var evaluationContext1 = mockTransactionOfCallDataSize(BLOCK_CALL_DATA_HALF_SIZE);
    verifyTransactionSelection(transactionSelector, evaluationContext1, SELECTED);

    final var evaluationContext2 = mockTransactionOfCallDataSize(BLOCK_CALL_DATA_HALF_SIZE);
    verifyTransactionSelection(transactionSelector, evaluationContext2, SELECTED);
  }

  @Test
  public void
      shouldNotSelectTransactionWhen_BlockCallDataSize_IsGreaterThan_MaxBlockCallDataSize() {
    final var evaluationContext1 = mockTransactionOfCallDataSize(BLOCK_CALL_DATA_HALF_SIZE);
    verifyTransactionSelection(transactionSelector, evaluationContext1, SELECTED);

    final var evaluationContext2 = mockTransactionOfCallDataSize(BLOCK_CALL_DATA_HALF_SIZE + 1);
    verifyTransactionSelection(transactionSelector, evaluationContext2, BLOCK_CALLDATA_OVERFLOW);
  }

  @Test
  public void shouldNotSelectAdditionalTransactionOnceBlockIsFull() {
    final var evaluationContext1 = mockTransactionOfCallDataSize(TX_CALL_DATA_SIZE);
    verifyTransactionSelection(transactionSelector, evaluationContext1, SELECTED);

    final var evaluationContext2 = mockTransactionOfCallDataSize(TX_CALL_DATA_SIZE);
    verifyTransactionSelection(transactionSelector, evaluationContext2, BLOCK_CALLDATA_OVERFLOW);

    final var evaluationContext3 = mockTransactionOfCallDataSize(TX_CALL_DATA_SIZE);
    verifyTransactionSelection(transactionSelector, evaluationContext3, BLOCK_CALLDATA_OVERFLOW);
  }

  private void verifyTransactionSelection(
      final PluginTransactionSelector selector,
      final TestTransactionEvaluationContext evaluationContext,
      final TransactionSelectionResult expectedPreProcessedResult) {
    final var preProcessedResult = selector.evaluateTransactionPreProcessing(evaluationContext);
    assertThat(preProcessedResult).isEqualTo(expectedPreProcessedResult);
    final var processingResult = mock(TransactionProcessingResult.class);
    selector.evaluateTransactionPostProcessing(evaluationContext, processingResult);
    notifySelector(selector, evaluationContext, preProcessedResult, processingResult);
  }

  private void notifySelector(
      final PluginTransactionSelector selector,
      final TestTransactionEvaluationContext evaluationContext,
      final TransactionSelectionResult selectionResult,
      final TransactionProcessingResult processingResult) {
    if (selectionResult.equals(SELECTED)) {
      selector.onTransactionSelected(evaluationContext, processingResult);
    } else {
      selector.onTransactionNotSelected(evaluationContext, selectionResult);
    }
  }

  private TestTransactionEvaluationContext mockTransactionOfCallDataSize(final int size) {
    PendingTransaction pendingTransaction = mock(PendingTransaction.class);
    Transaction transaction = mock(Transaction.class);
    when(pendingTransaction.getTransaction()).thenReturn(transaction);
    when(transaction.getPayload()).thenReturn(Bytes.repeat((byte) 1, size));
    when(transaction.getHash()).thenReturn(Hash.wrap(Bytes32.repeat((byte) seq++)));
    return new TestTransactionEvaluationContext(
        mock(ProcessableBlockHeader.class), pendingTransaction);
  }
}
