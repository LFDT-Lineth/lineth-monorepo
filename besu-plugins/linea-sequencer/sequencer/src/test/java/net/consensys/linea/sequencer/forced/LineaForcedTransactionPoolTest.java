/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.sequencer.forced;

import static net.consensys.linea.sequencer.txselection.LineaTransactionSelectionResult.TX_FILTERED_ADDRESS_FROM;
import static net.consensys.linea.sequencer.txselection.LineaTransactionSelectionResult.TX_FILTERED_ADDRESS_TO;
import static net.consensys.linea.sequencer.txselection.LineaTransactionSelectionResult.txModuleLineCountOverflow;
import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.RETURNS_DEEP_STUBS;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.atomic.AtomicLong;
import java.util.stream.Collectors;
import net.consensys.linea.utils.TestTransactionFactory;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.hyperledger.besu.plugin.data.AddedBlockContext;
import org.hyperledger.besu.plugin.data.TransactionSelectionResult;
import org.hyperledger.besu.plugin.services.txselection.BlockTransactionSelectionService;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

class LineaForcedTransactionPoolTest {
  private static final long TEST_TIMESTAMP = 1_700_000_000L;

  private LineaForcedTransactionPool pool;
  private TestTransactionFactory txFactory;
  private AtomicLong forcedTxNumberGenerator;

  @BeforeEach
  void setUp() {
    pool = new LineaForcedTransactionPool(100, null);
    txFactory = new TestTransactionFactory();
    forcedTxNumberGenerator = new AtomicLong(1);
  }

  @Test
  void addForcedTransactions_addsToQueue() {
    final List<ForcedTransaction> ftxs = createForcedTransactions(3);

    pool.addForcedTransactions(ftxs);

    assertThat(pool.pendingCount()).isEqualTo(3);
  }

  @Test
  void processForBlock_tentativelySelectsTransaction() {
    final ForcedTransaction ftx = createForcedTransaction();
    pool.addForcedTransactions(List.of(ftx));

    pool.processForBlock(100L, alwaysSelect());

    // Transaction is tentatively selected but still in queue until block is added
    assertThat(pool.pendingCount()).isEqualTo(1);
    assertThat(pool.getInclusionStatus(ftx.forcedTransactionNumber())).isEmpty();

    // Simulate block added
    pool.onBlockAdded(createBlockContext(100L, TEST_TIMESTAMP, List.of(ftx)));

    assertThat(pool.pendingCount()).isZero();
    assertInclusionStatus(
        ftx.forcedTransactionNumber(), 100L, ForcedTransactionInclusionResult.Included);
  }

  @Test
  void processForBlock_includesMultipleTransactionsInOrder() {
    final List<ForcedTransaction> ftxs = createForcedTransactions(3);
    pool.addForcedTransactions(ftxs);

    pool.processForBlock(50L, alwaysSelect());

    // All tentatively selected but still in queue
    assertThat(pool.pendingCount()).isEqualTo(3);

    // Simulate block added with all transactions
    pool.onBlockAdded(createBlockContext(50L, TEST_TIMESTAMP, ftxs));

    assertThat(pool.pendingCount()).isZero();
    for (final ForcedTransaction ftx : ftxs) {
      assertInclusionStatus(
          ftx.forcedTransactionNumber(), 50L, ForcedTransactionInclusionResult.Included);
    }
  }

  @Test
  void processForBlock_rejectsTransactionAtIndex0WithFinalStatus() {
    final ForcedTransaction ftx = createForcedTransaction();
    pool.addForcedTransactions(List.of(ftx));

    pool.processForBlock(100L, alwaysRejectInvalidTransient("NONCE_TOO_LOW"));

    // Transaction is tentatively rejected but still in queue until block is added
    assertThat(pool.pendingCount()).isEqualTo(1);
    assertThat(pool.getInclusionStatus(ftx.forcedTransactionNumber())).isEmpty();

    // Simulate block added (rejection is confirmed)
    pool.onBlockAdded(createBlockContext(100L, TEST_TIMESTAMP, List.of()));

    assertThat(pool.pendingCount()).isZero();
    assertInclusionStatus(
        ftx.forcedTransactionNumber(), 100L, ForcedTransactionInclusionResult.BadNonce);
  }

  @Test
  void processForBlock_retriesTransactionFailedAtIndexGreaterThan0() {
    final List<ForcedTransaction> ftxs = createForcedTransactions(2);
    pool.addForcedTransactions(ftxs);

    // Block 1: first tx succeeds, second tx fails at index 1
    pool.processForBlock(100L, selectThenReject("NONCE_TOO_LOW"));

    // First tx tentatively selected, second tx still pending (will retry)
    assertThat(pool.pendingCount()).isEqualTo(2);
    assertThat(pool.getInclusionStatus(ftxs.get(0).forcedTransactionNumber())).isEmpty();
    assertThat(pool.getInclusionStatus(ftxs.get(1).forcedTransactionNumber())).isEmpty();

    // Simulate block added with first tx
    pool.onBlockAdded(createBlockContext(100L, TEST_TIMESTAMP, List.of(ftxs.get(0))));

    assertThat(pool.pendingCount()).isEqualTo(1);
    assertInclusionStatus(
        ftxs.get(0).forcedTransactionNumber(), 100L, ForcedTransactionInclusionResult.Included);
    assertThat(pool.getInclusionStatus(ftxs.get(1).forcedTransactionNumber())).isEmpty();

    // Block 2: second tx fails at index 0 - tentative rejection
    pool.processForBlock(101L, alwaysRejectInvalidTransient("NONCE_TOO_LOW"));

    // Still in queue until block added
    assertThat(pool.pendingCount()).isEqualTo(1);
    assertThat(pool.getInclusionStatus(ftxs.get(1).forcedTransactionNumber())).isEmpty();

    // Simulate block added (rejection confirmed)
    pool.onBlockAdded(createBlockContext(101L, TEST_TIMESTAMP, List.of()));

    assertThat(pool.pendingCount()).isZero();
    assertInclusionStatus(
        ftxs.get(1).forcedTransactionNumber(), 101L, ForcedTransactionInclusionResult.BadNonce);
  }

  @Test
  void processForBlock_stopsOnFirstFailureToPreventMultipleInvalidityProofs() {
    final List<ForcedTransaction> ftxs = createForcedTransactions(3);
    pool.addForcedTransactions(ftxs);

    // Block 1: first tx succeeds, second tx fails - third tx should NOT be evaluated
    pool.processForBlock(100L, selectThenReject("UPFRONT_COST_EXCEEDS_BALANCE"));

    assertThat(pool.pendingCount()).isEqualTo(3); // All still in queue until block added
    assertThat(pool.getInclusionStatus(ftxs.get(0).forcedTransactionNumber())).isEmpty();
    assertThat(pool.getInclusionStatus(ftxs.get(1).forcedTransactionNumber())).isEmpty();
    assertThat(pool.getInclusionStatus(ftxs.get(2).forcedTransactionNumber())).isEmpty();

    // Simulate block added with first tx
    pool.onBlockAdded(createBlockContext(100L, TEST_TIMESTAMP, List.of(ftxs.get(0))));

    assertThat(pool.pendingCount()).isEqualTo(2); // tx2 and tx3 still pending
    assertInclusionStatus(
        ftxs.get(0).forcedTransactionNumber(), 100L, ForcedTransactionInclusionResult.Included);
    assertThat(pool.getInclusionStatus(ftxs.get(1).forcedTransactionNumber()))
        .isEmpty(); // Will retry
    assertThat(pool.getInclusionStatus(ftxs.get(2).forcedTransactionNumber()))
        .isEmpty(); // Not tried yet

    pool.processForBlock(101L, alwaysRejectInvalidTransient("NONCE_TOO_LOW"));
    pool.onBlockAdded(createBlockContext(101L, TEST_TIMESTAMP, List.of()));

    assertThat(pool.pendingCount()).isEqualTo(1); // tx3 still pending
    assertInclusionStatus(
        ftxs.get(1).forcedTransactionNumber(), 101L, ForcedTransactionInclusionResult.BadNonce);
    assertThat(pool.getInclusionStatus(ftxs.get(2).forcedTransactionNumber()))
        .isEmpty(); // Not tried yet

    pool.processForBlock(
        102L, alwaysRejectWith(txModuleLineCountOverflow("PRECOMPILE_RIPEMD_BLOCKS")));
    pool.onBlockAdded(createBlockContext(102L, TEST_TIMESTAMP, List.of()));

    assertThat(pool.pendingCount()).isEqualTo(0);
    assertInclusionStatus(
        ftxs.get(2).forcedTransactionNumber(),
        102L,
        ForcedTransactionInclusionResult.BadPrecompile);
  }

  @Test
  void processForBlock_twoConsecutiveFailingTransactionsGetStatusInDifferentBlocks() {
    final List<ForcedTransaction> ftxs = createForcedTransactions(2);
    pool.addForcedTransactions(ftxs);

    // Block 1: first tx fails at index 0 - tentative rejection
    pool.processForBlock(100L, alwaysRejectInvalidTransient("NONCE_TOO_LOW"));

    assertThat(pool.pendingCount()).isEqualTo(2);
    assertThat(pool.getInclusionStatus(ftxs.get(0).forcedTransactionNumber())).isEmpty();
    assertThat(pool.getInclusionStatus(ftxs.get(1).forcedTransactionNumber())).isEmpty();

    // Simulate block added - rejection confirmed
    pool.onBlockAdded(createBlockContext(100L, TEST_TIMESTAMP, List.of()));

    assertThat(pool.pendingCount()).isEqualTo(1);
    assertInclusionStatus(
        ftxs.get(0).forcedTransactionNumber(), 100L, ForcedTransactionInclusionResult.BadNonce);
    assertThat(pool.getInclusionStatus(ftxs.get(1).forcedTransactionNumber())).isEmpty();

    // Block 2: second tx fails at index 0 - tentative rejection
    pool.processForBlock(101L, alwaysRejectInvalidTransient("UPFRONT_COST_EXCEEDS_BALANCE"));
    pool.onBlockAdded(createBlockContext(101L, TEST_TIMESTAMP, List.of()));

    assertThat(pool.pendingCount()).isZero();
    assertInclusionStatus(
        ftxs.get(1).forcedTransactionNumber(), 101L, ForcedTransactionInclusionResult.BadBalance);
  }

  @Test
  void processForBlock_multipleCallsForSameBlockOnlyProcessOneFailure() {
    // This test verifies the key invariant: only one invalidity proof per block
    // When processForBlock is called multiple times for the same block (as happens during
    // iterative block building), we should only process one failure per block.
    final List<ForcedTransaction> ftxs = createForcedTransactions(3);
    pool.addForcedTransactions(ftxs);

    // First call to processForBlock for block 100: first tx fails (tentative)
    pool.processForBlock(100L, alwaysRejectInvalidTransient("NONCE_TOO_LOW"));

    // Simulate iterative block building: second call for same block 100
    // This should be skipped because we already had a failure in this block
    pool.processForBlock(100L, alwaysRejectInvalidTransient("NONCE_TOO_LOW"));

    // Third call for same block 100 - still should be skipped
    pool.processForBlock(100L, alwaysRejectInvalidTransient("NONCE_TOO_LOW"));

    // All still pending until block added, but only first tx has tentative rejection
    assertThat(pool.pendingCount()).isEqualTo(3);
    assertThat(pool.getInclusionStatus(ftxs.get(0).forcedTransactionNumber())).isEmpty();
    assertThat(pool.getInclusionStatus(ftxs.get(1).forcedTransactionNumber())).isEmpty();
    assertThat(pool.getInclusionStatus(ftxs.get(2).forcedTransactionNumber())).isEmpty();

    // Block added - first tx rejection confirmed
    pool.onBlockAdded(createBlockContext(100L, TEST_TIMESTAMP, List.of()));

    assertThat(pool.pendingCount()).isEqualTo(2);
    assertInclusionStatus(
        ftxs.get(0).forcedTransactionNumber(), 100L, ForcedTransactionInclusionResult.BadNonce);
    assertThat(pool.getInclusionStatus(ftxs.get(1).forcedTransactionNumber())).isEmpty();
    assertThat(pool.getInclusionStatus(ftxs.get(2).forcedTransactionNumber())).isEmpty();

    // Now move to block 101 - second tx should be processed
    pool.processForBlock(101L, alwaysRejectInvalidTransient("UPFRONT_COST_EXCEEDS_BALANCE"));
    pool.onBlockAdded(createBlockContext(101L, TEST_TIMESTAMP, List.of()));

    assertThat(pool.pendingCount()).isEqualTo(1);
    assertInclusionStatus(
        ftxs.get(1).forcedTransactionNumber(), 101L, ForcedTransactionInclusionResult.BadBalance);
    assertThat(pool.getInclusionStatus(ftxs.get(2).forcedTransactionNumber())).isEmpty();

    // Move to block 102 - third tx processed
    pool.processForBlock(
        102L, alwaysRejectWith(txModuleLineCountOverflow("PRECOMPILE_BLAKE_EFFECTIVE_CALLS")));
    pool.onBlockAdded(createBlockContext(102L, TEST_TIMESTAMP, List.of()));

    assertThat(pool.pendingCount()).isZero();
    assertInclusionStatus(
        ftxs.get(2).forcedTransactionNumber(),
        102L,
        ForcedTransactionInclusionResult.BadPrecompile);
  }

  @Test
  void processForBlock_mapsNonceErrorsCorrectly() {
    final ForcedTransaction ftx = createForcedTransaction();
    pool.addForcedTransactions(List.of(ftx));

    pool.processForBlock(100L, alwaysRejectInvalidTransient("NONCE_TOO_HIGH"));
    pool.onBlockAdded(createBlockContext(100L, TEST_TIMESTAMP, List.of()));

    assertInclusionStatus(
        ftx.forcedTransactionNumber(), 100L, ForcedTransactionInclusionResult.BadNonce);
  }

  @Test
  void processForBlock_mapsBalanceErrorsCorrectly() {
    final ForcedTransaction ftx = createForcedTransaction();
    pool.addForcedTransactions(List.of(ftx));

    pool.processForBlock(100L, alwaysRejectInvalidTransient("UPFRONT_COST_EXCEEDS_BALANCE"));
    pool.onBlockAdded(createBlockContext(100L, TEST_TIMESTAMP, List.of()));

    assertInclusionStatus(
        ftx.forcedTransactionNumber(), 100L, ForcedTransactionInclusionResult.BadBalance);
  }

  @Test
  void processForBlock_mapsFilteredAddressFromErrorCorrectly() {
    final ForcedTransaction ftx = createForcedTransaction();
    pool.addForcedTransactions(List.of(ftx));

    pool.processForBlock(100L, alwaysRejectWith(TX_FILTERED_ADDRESS_FROM));
    pool.onBlockAdded(createBlockContext(100L, TEST_TIMESTAMP, List.of()));

    assertInclusionStatus(
        ftx.forcedTransactionNumber(), 100L, ForcedTransactionInclusionResult.FilteredAddressFrom);
  }

  @Test
  void processForBlock_mapsFilteredAddressToErrorCorrectly() {
    final ForcedTransaction ftx = createForcedTransaction();
    pool.addForcedTransactions(List.of(ftx));

    pool.processForBlock(100L, alwaysRejectWith(TX_FILTERED_ADDRESS_TO));
    pool.onBlockAdded(createBlockContext(100L, TEST_TIMESTAMP, List.of()));

    assertInclusionStatus(
        ftx.forcedTransactionNumber(), 100L, ForcedTransactionInclusionResult.FilteredAddressTo);
  }

  @Test
  void processForBlock_retriesOnUnknownRejectionReasonAtIndex0() {
    final ForcedTransaction ftx = createForcedTransaction();
    pool.addForcedTransactions(List.of(ftx));

    // First block: unknown rejection reason at index 0 - should retry
    pool.processForBlock(100L, alwaysRejectInvalidTransient("SOME_UNKNOWN_REASON"));

    assertThat(pool.pendingCount()).isEqualTo(1); // Still pending
    assertThat(pool.getInclusionStatus(ftx.forcedTransactionNumber()))
        .isEmpty(); // No status recorded

    // Second block: same unknown reason at index 0 - still retry
    pool.processForBlock(101L, alwaysRejectInvalidTransient("ANOTHER_UNKNOWN_REASON"));

    assertThat(pool.pendingCount()).isEqualTo(1); // Still pending
    assertThat(pool.getInclusionStatus(ftx.forcedTransactionNumber())).isEmpty(); // Still no status

    // Third block: now it succeeds
    pool.processForBlock(102L, alwaysSelect());

    // Still in queue until block added
    assertThat(pool.pendingCount()).isEqualTo(1);
    assertThat(pool.getInclusionStatus(ftx.forcedTransactionNumber())).isEmpty();

    // Simulate block added
    pool.onBlockAdded(createBlockContext(102L, TEST_TIMESTAMP, List.of(ftx)));

    assertThat(pool.pendingCount()).isZero();
    assertInclusionStatus(
        ftx.forcedTransactionNumber(), 102L, ForcedTransactionInclusionResult.Included);
  }

  @Test
  void processForBlock_finallyRejectsWhenKnownReasonAtIndex0AfterRetry() {
    final ForcedTransaction ftx = createForcedTransaction();
    pool.addForcedTransactions(List.of(ftx));

    // First block: unknown rejection reason - will retry
    pool.processForBlock(100L, alwaysRejectInvalidTransient("SOME_UNKNOWN_REASON"));

    assertThat(pool.pendingCount()).isEqualTo(1);
    assertThat(pool.getInclusionStatus(ftx.forcedTransactionNumber())).isEmpty();

    // Second block: known rejection reason (NONCE) - tentative rejection
    pool.processForBlock(101L, alwaysRejectInvalidTransient("NONCE_TOO_LOW"));

    // Still pending until block added
    assertThat(pool.pendingCount()).isEqualTo(1);
    assertThat(pool.getInclusionStatus(ftx.forcedTransactionNumber())).isEmpty();

    // Block added - rejection confirmed
    pool.onBlockAdded(createBlockContext(101L, TEST_TIMESTAMP, List.of()));

    assertThat(pool.pendingCount()).isZero();
    assertInclusionStatus(
        ftx.forcedTransactionNumber(), 101L, ForcedTransactionInclusionResult.BadNonce);
  }

  @Test
  void getInclusionStatus_returnsEmptyForUnknownTransaction() {
    assertThat(pool.getInclusionStatus(999L)).isEmpty();
  }

  @Test
  void processForBlock_doesNothingWhenQueueEmpty() {
    pool.processForBlock(100L, alwaysSelect());
    assertThat(pool.pendingCount()).isZero();
  }

  @Test
  void processForBlock_transactionsPreservedIfBlockNotSealed() {
    // This test verifies the key fix: transactions are not lost if block selection
    // is restarted before the block is sealed (onBlockAdded is not called)
    final ForcedTransaction ftx = createForcedTransaction();
    pool.addForcedTransactions(List.of(ftx));

    // First attempt: transaction is tentatively selected
    pool.processForBlock(100L, alwaysSelect());

    assertThat(pool.pendingCount()).isEqualTo(1); // Still in queue
    assertThat(pool.getInclusionStatus(ftx.forcedTransactionNumber())).isEmpty();

    // Block NOT sealed - no onBlockAdded call
    // New block selection attempt starts (e.g., due to timeout or new transactions)
    pool.processForBlock(100L, alwaysSelect());

    assertThat(pool.pendingCount()).isEqualTo(1); // Still in queue
    assertThat(pool.getInclusionStatus(ftx.forcedTransactionNumber())).isEmpty();

    // Finally, block is added
    pool.onBlockAdded(createBlockContext(100L, TEST_TIMESTAMP, List.of(ftx)));

    assertThat(pool.pendingCount()).isZero();
    assertInclusionStatus(
        ftx.forcedTransactionNumber(), 100L, ForcedTransactionInclusionResult.Included);
  }

  @Test
  void onBlockAdded_clearsStaleSelectionsFromDifferentBlock() {
    final ForcedTransaction ftx = createForcedTransaction();
    pool.addForcedTransactions(List.of(ftx));

    // Transaction tentatively selected for block 100
    pool.processForBlock(100L, alwaysSelect());

    assertThat(pool.pendingCount()).isEqualTo(1);

    // Block 101 added instead (reorg scenario or block 100 never sealed)
    // Transaction hash NOT in block 101
    pool.onBlockAdded(createBlockContext(101L, TEST_TIMESTAMP, List.of()));

    // Transaction still in queue (not confirmed)
    assertThat(pool.pendingCount()).isEqualTo(1);
    assertThat(pool.getInclusionStatus(ftx.forcedTransactionNumber())).isEmpty();

    // Can be re-selected in next block
    pool.processForBlock(102L, alwaysSelect());
    pool.onBlockAdded(createBlockContext(102L, TEST_TIMESTAMP, List.of(ftx)));

    assertThat(pool.pendingCount()).isZero();
    assertInclusionStatus(
        ftx.forcedTransactionNumber(), 102L, ForcedTransactionInclusionResult.Included);
  }

  @Test
  void onBlockAdded_clearsStaleRejectionsFromDifferentBlock() {
    final ForcedTransaction ftx = createForcedTransaction();
    pool.addForcedTransactions(List.of(ftx));

    // Transaction tentatively rejected for block 100
    pool.processForBlock(100L, alwaysRejectInvalidTransient("NONCE_TOO_LOW"));

    assertThat(pool.pendingCount()).isEqualTo(1);
    assertThat(pool.getInclusionStatus(ftx.forcedTransactionNumber())).isEmpty();

    // Block 101 added instead (reorg scenario or block 100 never sealed)
    // Rejection for block 100 should be discarded
    pool.onBlockAdded(createBlockContext(101L, TEST_TIMESTAMP, List.of()));

    // Transaction still in queue (rejection not confirmed because block numbers don't match)
    assertThat(pool.pendingCount()).isEqualTo(1);
    assertThat(pool.getInclusionStatus(ftx.forcedTransactionNumber())).isEmpty();

    // Can be re-evaluated in next block - maybe it will succeed now
    pool.processForBlock(102L, alwaysSelect());
    pool.onBlockAdded(createBlockContext(102L, TEST_TIMESTAMP, List.of(ftx)));

    assertThat(pool.pendingCount()).isZero();
    assertInclusionStatus(
        ftx.forcedTransactionNumber(), 102L, ForcedTransactionInclusionResult.Included);
  }

  @Test
  void processForBlock_rejectionNotFinalizedIfBlockNotSealed() {
    final ForcedTransaction ftx = createForcedTransaction();
    pool.addForcedTransactions(List.of(ftx));

    // First attempt: transaction is tentatively rejected for block 100
    pool.processForBlock(100L, alwaysRejectInvalidTransient("NONCE_TOO_LOW"));

    assertThat(pool.pendingCount()).isEqualTo(1);
    assertThat(pool.getInclusionStatus(ftx.forcedTransactionNumber())).isEmpty();

    // Block 100 NOT sealed - no onBlockAdded call
    // A different block (101) is added instead (e.g., slot was missed)
    // The tentative rejection for block 100 should be discarded
    pool.onBlockAdded(createBlockContext(101L, TEST_TIMESTAMP, List.of()));

    // Transaction still in queue (rejection not confirmed - wrong block number)
    assertThat(pool.pendingCount()).isEqualTo(1);
    assertThat(pool.getInclusionStatus(ftx.forcedTransactionNumber())).isEmpty();

    // New block building for block 102 - this time the transaction succeeds
    pool.processForBlock(102L, alwaysSelect());
    pool.onBlockAdded(createBlockContext(102L, TEST_TIMESTAMP, List.of(ftx)));

    assertThat(pool.pendingCount()).isZero();
    assertInclusionStatus(
        ftx.forcedTransactionNumber(), 102L, ForcedTransactionInclusionResult.Included);
  }

  private void assertInclusionStatus(
      final long forcedTransactionNumber,
      final long expectedBlockNumber,
      final ForcedTransactionInclusionResult expectedResult) {
    final var status = pool.getInclusionStatus(forcedTransactionNumber);
    assertThat(status).isPresent();
    assertThat(status.get().blockNumber()).isEqualTo(expectedBlockNumber);
    assertThat(status.get().inclusionResult()).isEqualTo(expectedResult);
  }

  private ForcedTransaction createForcedTransaction() {
    final Transaction tx = txFactory.createTransaction();
    return new ForcedTransaction(
        forcedTxNumberGenerator.getAndIncrement(), tx.getHash(), tx, Long.MAX_VALUE);
  }

  private List<ForcedTransaction> createForcedTransactions(final int count) {
    final List<ForcedTransaction> ftxs = new ArrayList<>(count);
    for (int i = 0; i < count; i++) {
      ftxs.add(createForcedTransaction());
    }
    return ftxs;
  }

  private BlockTransactionSelectionService alwaysSelect() {
    final BlockTransactionSelectionService bts = mock(BlockTransactionSelectionService.class);
    when(bts.evaluatePendingTransaction(any())).thenReturn(TransactionSelectionResult.SELECTED);
    return bts;
  }

  private BlockTransactionSelectionService alwaysRejectInvalidTransient(final String reason) {
    final BlockTransactionSelectionService bts = mock(BlockTransactionSelectionService.class);
    when(bts.evaluatePendingTransaction(any()))
        .thenReturn(TransactionSelectionResult.invalidTransient(reason));
    return bts;
  }

  private BlockTransactionSelectionService alwaysRejectWith(
      final TransactionSelectionResult result) {
    final BlockTransactionSelectionService bts = mock(BlockTransactionSelectionService.class);
    when(bts.evaluatePendingTransaction(any())).thenReturn(result);
    return bts;
  }

  private BlockTransactionSelectionService selectThenReject(final String rejectReason) {
    final BlockTransactionSelectionService bts = mock(BlockTransactionSelectionService.class);
    when(bts.evaluatePendingTransaction(any()))
        .thenReturn(TransactionSelectionResult.SELECTED)
        .thenReturn(TransactionSelectionResult.invalidTransient(rejectReason));
    return bts;
  }

  @SuppressWarnings("unchecked")
  private AddedBlockContext createBlockContext(
      final long blockNumber, final long timestamp, final List<ForcedTransaction> includedTxs) {
    final AddedBlockContext context = mock(AddedBlockContext.class, RETURNS_DEEP_STUBS);
    when(context.getBlockHeader().getNumber()).thenReturn(blockNumber);
    when(context.getBlockHeader().getTimestamp()).thenReturn(timestamp);
    when(context.getEventType()).thenReturn(AddedBlockContext.EventType.HEAD_ADVANCED);

    final List<Hash> txHashes =
        includedTxs.stream().map(ForcedTransaction::txHash).collect(Collectors.toList());

    final List<Transaction> blockTxs =
        txHashes.stream()
            .map(
                hash -> {
                  final Transaction tx = mock(Transaction.class);
                  when(tx.getHash()).thenReturn(hash);
                  return tx;
                })
            .collect(Collectors.toList());

    when((List<Transaction>) context.getBlockBody().getTransactions()).thenReturn(blockTxs);
    return context;
  }
}
