/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
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
