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

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

import java.time.Instant;
import java.util.List;
import java.util.Optional;

import net.consensys.linea.bundles.TransactionBundle;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.hyperledger.besu.plugin.data.BlockHeader;
import org.hyperledger.besu.plugin.data.TransactionProcessingResult;
import org.hyperledger.besu.plugin.data.TransactionSelectionResult;
import org.hyperledger.besu.plugin.services.txselection.TransactionEvaluationContext;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

class BundleConstraintTransactionSelectorTest {

  private BundleConstraintTransactionSelector selector;

  @BeforeEach
  void setUp() {
    selector = new BundleConstraintTransactionSelector();
  }

  @Test
  void testEvaluateTransactionPreProcessing_Selected() {
    var blockHeader = mockBlockHeader(1);
    TransactionBundle bundle = createBundle(List.of(mock(Transaction.class)), 1, null, null);
    var pendingTransaction = bundle.pendingTransactions().getFirst();
    var txContext = mockTransactionEvaluationContext(blockHeader, pendingTransaction);

    var result = selector.evaluateTransactionPreProcessing(txContext);

    assertEquals(TransactionSelectionResult.SELECTED, result);
  }

  @Test
  void testEvaluateTransactionPreProcessing_FailedCriteria() {
    var blockHeader = mockBlockHeader(1);
    TransactionBundle bundle =
        createBundle(
            List.of(mock(Transaction.class)), 1, Instant.now().getEpochSecond() + 10000000, null);
    var pendingTransaction = bundle.pendingTransactions().getFirst();
    var txContext = mockTransactionEvaluationContext(blockHeader, pendingTransaction);

    var result = selector.evaluateTransactionPreProcessing(txContext);

    assertEquals(TransactionSelectionResult.invalid("Failed Bundled Transaction Criteria"), result);
  }

  @Test
  void testEvaluateTransactionPostProcessing_FailedNonRevertable() {
    var blockHeader = mockBlockHeader(1);
    TransactionBundle bundle = createBundle(List.of(mock(Transaction.class)), 1, null, null);
    var pendingTransaction = bundle.pendingTransactions().getFirst();
    var txContext = mockTransactionEvaluationContext(blockHeader, pendingTransaction);

    var transactionProcessingResult = mock(TransactionProcessingResult.class);
    when(transactionProcessingResult.isFailed()).thenReturn(true);

    var result = selector.evaluateTransactionPostProcessing(txContext, transactionProcessingResult);

    assertEquals(
        TransactionSelectionResult.invalid("Failed non revertable transaction in bundle"), result);
  }

  @Test
  void testEvaluateTransactionPostProcessing_Selected() {
    var blockHeader = mockBlockHeader(1);
    var pendingTransaction = mock(TransactionBundle.PendingBundleTx.class);
    var txContext = mockTransactionEvaluationContext(blockHeader, pendingTransaction);

    var transactionProcessingResult = mock(TransactionProcessingResult.class);
    when(transactionProcessingResult.isFailed()).thenReturn(false);

    var result = selector.evaluateTransactionPostProcessing(txContext, transactionProcessingResult);

    assertEquals(TransactionSelectionResult.SELECTED, result);
  }

  private TransactionBundle createBundle(
      List<Transaction> txs, long blockNumber, Long minTimestamp, Long maxTimestamp) {
    return new TransactionBundle(
        Hash.fromHexStringLenient("0x1234"),
        txs,
        blockNumber,
        Optional.ofNullable(minTimestamp),
        Optional.ofNullable(maxTimestamp),
        Optional.empty(),
        Optional.empty());
  }

  private TransactionEvaluationContext mockTransactionEvaluationContext(
      BlockHeader blockHeader, TransactionBundle.PendingBundleTx pendingTransaction) {
    return new TestTransactionEvaluationContext(blockHeader, pendingTransaction, Wei.ONE, Wei.ONE);
  }

  private BlockHeader mockBlockHeader(long blockNumber) {
    var blockHeader = mock(BlockHeader.class);
    when(blockHeader.getNumber()).thenReturn(blockNumber);
    return blockHeader;
  }
}
