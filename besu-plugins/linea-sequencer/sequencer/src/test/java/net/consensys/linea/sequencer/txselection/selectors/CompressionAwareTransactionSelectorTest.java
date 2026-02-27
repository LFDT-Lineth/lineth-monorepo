/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.sequencer.txselection.selectors;

import static net.consensys.linea.sequencer.txselection.LineaTransactionSelectionResult.BLOCK_COMPRESSED_SIZE_OVERFLOW;
import static org.assertj.core.api.Assertions.assertThat;
import static org.hyperledger.besu.plugin.data.TransactionSelectionResult.SELECTED;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.times;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import linea.blob.BlobCompressor;
import linea.blob.BlobCompressorVersion;
import linea.blob.GoBackedBlobCompressor;
import net.consensys.linea.utils.CachingTransactionCompressor;
import net.consensys.linea.utils.TestTransactionFactory;
import net.consensys.linea.utils.TransactionCompressor;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.datatypes.PendingTransaction;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.core.Difficulty;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.hyperledger.besu.plugin.data.ProcessableBlockHeader;
import org.hyperledger.besu.plugin.data.TransactionProcessingResult;
import org.hyperledger.besu.plugin.data.TransactionSelectionResult;
import org.hyperledger.besu.plugin.services.txselection.SelectorsStateManager;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

class CompressionAwareTransactionSelectorTest {

  /** A small data limit sufficient for test transactions. */
  private static final int TEST_DATA_LIMIT = 4 * 1024;

  /** Block header overhead used in tests. */
  private static final int TEST_HEADER_OVERHEAD = 1024;

  private static final TransactionCompressor TX_COMPRESSOR =
      new CachingTransactionCompressor(
          GoBackedBlobCompressor.getInstance(BlobCompressorVersion.V2, 128 * 1024));

  private SelectorsStateManager selectorsStateManager;
  private TestTransactionFactory txFactory;

  @BeforeEach
  void setUp() {
    selectorsStateManager = new SelectorsStateManager();
    txFactory = new TestTransactionFactory();
  }

  /**
   * Re-initializes the shared compressor with a new data limit. Calling {@code getInstance} with a
   * new limit re-runs {@code Init} on the underlying native singleton and returns a wrapper that
   * uses it.
   */
  private static GoBackedBlobCompressor compressorWithLimit(final int dataLimit) {
    return GoBackedBlobCompressor.getInstance(BlobCompressorVersion.V2, dataLimit);
  }

  @Test
  void fastExecutionPathSelectsTransactionsWhenCumulativeCompressedSizeBelowLimit() {
    final int blobSizeLimit = TEST_DATA_LIMIT;
    final var selector = createSelector(blobSizeLimit);

    final var tx1 = txFactory.createTransaction();
    final var tx2 = txFactory.createTransaction();
    final var tx3 = txFactory.createTransaction();

    assertThat(evaluateTx(selector, wrapTx(tx1))).isEqualTo(SELECTED);
    assertThat(evaluateTx(selector, wrapTx(tx2))).isEqualTo(SELECTED);
    assertThat(evaluateTx(selector, wrapTx(tx3))).isEqualTo(SELECTED);
  }

  @Test
  void rejectsWhenBlockCannotFitMoreTransactions() {
    final var probeTx = txFactory.createTransaction();
    final int perTxCompressed = TX_COMPRESSOR.getCompressedSize(probeTx);

    // Use a small header overhead so we can test with a tight blob size limit.
    // The blob size limit must be > header overhead, but we want it smaller than one tx.
    final int smallHeaderOverhead = 10;
    final int blobSizeLimit = smallHeaderOverhead + perTxCompressed - 1;

    txFactory = new TestTransactionFactory();
    final var selector = createSelector(blobSizeLimit, smallHeaderOverhead);

    final var tx = txFactory.createTransaction();
    // Overflow may come from post-processing (async slow execution path) rather than
    // pre-processing.
    assertThat(evaluateTx(selector, wrapTx(tx))).isEqualTo(BLOCK_COMPRESSED_SIZE_OVERFLOW);
  }

  @Test
  void eventuallyFillsBlockAndRejects() {
    final var probeTx = txFactory.createTransaction();
    final int perTxCompressed = TX_COMPRESSOR.getCompressedSize(probeTx);

    // Use a small header overhead so we can test with a tight blob size limit.
    final int smallHeaderOverhead = 10;
    final int blobSizeLimit = smallHeaderOverhead + perTxCompressed * 5;

    txFactory = new TestTransactionFactory();
    final var selector = createSelector(blobSizeLimit, smallHeaderOverhead);

    int selectedCount = 0;
    boolean rejected = false;
    for (int i = 0; i < 100 && !rejected; i++) {
      final var result = evaluateTx(selector, wrapTx(txFactory.createTransaction()));
      if (result.equals(SELECTED)) {
        selectedCount++;
      } else {
        assertThat(result).isEqualTo(BLOCK_COMPRESSED_SIZE_OVERFLOW);
        rejected = true;
      }
    }
    assertThat(selectedCount).isGreaterThanOrEqualTo(1);
    assertThat(rejected).isTrue();
  }

  @Test
  void slowExecutionPathMaximisesTransactionsAboveFastPathLimit() {
    // Measure per-tx compressed size and compute how many txs the fast execution path alone would
    // allow.
    // The fast execution path budget is: blobSizeLimit - headerOverhead.
    // With the two-phase selector, the slow execution path should allow strictly more txs.
    final var probeTx = txFactory.createTransaction();
    final int perTxCompressed = TX_COMPRESSOR.getCompressedSize(probeTx);

    // Use a limit where the fast execution path alone allows exactly N txs:
    //   fastExecutionPathLimit = blobSizeLimit - headerOverhead
    //   N = floor(fastExecutionPathLimit / perTxCompressed)
    // We pick blobSizeLimit so N is a small known number.
    final int fastExecutionPathTxCount = 3;
    final int blobSizeLimit = perTxCompressed * fastExecutionPathTxCount + TEST_HEADER_OVERHEAD + 1;

    txFactory = new TestTransactionFactory();
    final var selector = createSelector(blobSizeLimit);

    int selectedCount = 0;
    for (int i = 0; i < 100; i++) {
      final var result = evaluateTx(selector, wrapTx(txFactory.createTransaction()));
      if (result.equals(SELECTED)) {
        selectedCount++;
      } else {
        break;
      }
    }

    // The slow execution path (canAppendBlock) should allow strictly more than the fast execution
    // path alone.
    // Fast execution path alone would stop after fastExecutionPathTxCount txs; slow execution path
    // should squeeze in more.
    assertThat(selectedCount)
        .as(
            "Slow execution path should allow more than %d txs that the fast execution path alone would permit",
            fastExecutionPathTxCount)
        .isGreaterThan(fastExecutionPathTxCount);
  }

  /**
   * Verifies that after a large transaction triggers {@code BLOCK_COMPRESSED_SIZE_OVERFLOW} on the
   * slow execution path, the block-building state is rolled back and a subsequent small transaction
   * that fits in the remaining space is still selected.
   *
   * <p>This exercises the key invariant that {@code BLOCK_COMPRESSED_SIZE_OVERFLOW} has {@code
   * stop=false}: the block-building loop must keep going and should not give up on smaller
   * candidates after a big one overflows.
   */
  @Test
  void slowExecutionPathSelectsSmallTransactionAfterLargeTransactionCausesOverflow() {
    final var probeTx = txFactory.createTransaction();
    final int perTxCompressed = TX_COMPRESSOR.getCompressedSize(probeTx);

    // Fast execution path allows exactly 3 txs: fastExecutionPathLimit = blobSizeLimit -
    // TEST_HEADER_OVERHEAD
    //   = perTxCompressed * 3 + 1.
    // From slowExecutionPathMaximisesTransactionsAboveFastPathLimit we know the slow execution path
    // accepts
    // at least one more minimal tx in this configuration, so the 4th small tx is guaranteed
    // to be SELECTED after the oversized one is rejected.
    final int blobSizeLimit = perTxCompressed * 3 + TEST_HEADER_OVERHEAD + 1;

    txFactory = new TestTransactionFactory();
    final var selector = createSelector(blobSizeLimit);

    for (int i = 0; i < 3; i++) {
      assertThat(evaluateTx(selector, wrapTx(txFactory.createTransaction())))
          .as("tx %d should be selected on the fast execution path", i + 1)
          .isEqualTo(SELECTED);
    }

    // A transaction whose payload alone exceeds the entire blob budget uses the slow execution path
    // and
    // cannot be appended: appendBlock returns false → BLOCK_COMPRESSED_SIZE_OVERFLOW.
    final var largeTx = txFactory.createTransactionWithPayload(Bytes.random(blobSizeLimit));
    assertThat(evaluateTx(selector, wrapTx(largeTx)))
        .as("large transaction should overflow the compressed block")
        .isEqualTo(BLOCK_COMPRESSED_SIZE_OVERFLOW);

    // After the rejection, state is rolled back to the committed snapshot (3 txs selected).
    // A minimal transaction must still be selectable because there is remaining capacity.
    final var smallTx = txFactory.createTransaction();
    assertThat(evaluateTx(selector, wrapTx(smallTx)))
        .as("minimal transaction should be selected after the large transaction was rejected")
        .isEqualTo(SELECTED);
  }

  /**
   * Verifies that after a successful slow-execution-path check, {@code cumulativeCompressedSize} is
   * updated to the actual {@code compressedSizeAfter} returned by {@link
   * BlobCompressor#appendBlock}.
   *
   * <p>With the old sentinel logic ({@code cumulativeCompressedSize} permanently set to {@code
   * fastExecutionPathLimit} after the first slow-execution-path tx), every subsequent transaction
   * was forced through the slow execution path. With the fix, if {@code compressedSizeAfter <
   * fastExecutionPathLimit}, subsequent small transactions can re-enter the fast execution path and
   * avoid an unnecessary compression check.
   *
   * <p>This is verified by counting {@link BlobCompressor#appendBlock} invocations: with the fix
   * only the single slow-execution-path transaction triggers a compression check; the following
   * small transaction that fits within the remaining fast-execution-path budget does not.
   */
  @Test
  void afterSlowPathSuccess_cumulativeSizeUpdatedToActualCompressedSize() {
    final int headerOverhead = TEST_HEADER_OVERHEAD;
    final int blobSizeLimit = TEST_DATA_LIMIT;
    final int fastExecutionPathLimit = blobSizeLimit - headerOverhead;

    final TransactionCompressor mockTxCompressor = mock(TransactionCompressor.class);
    final BlobCompressor mockBlobCompressor = mock(BlobCompressor.class);

    // appendBlock reports that the block fits with a compressed size well below
    // fastExecutionPathLimit.
    final int compressedSizeAfter = fastExecutionPathLimit / 2;
    when(mockBlobCompressor.appendBlock(any()))
        .thenReturn(new BlobCompressor.AppendResult(true, 0, compressedSizeAfter));

    selectorsStateManager = new SelectorsStateManager();
    final var selector =
        new CompressionAwareTransactionSelector(
            selectorsStateManager,
            blobSizeLimit,
            headerOverhead,
            mockTxCompressor,
            mockBlobCompressor);
    selectorsStateManager.blockSelectionStarted();

    // tx1: just below fastExecutionPathLimit → fast execution path; cumulative =
    // fastExecutionPathLimit - 1.
    final var tx1 = txFactory.createTransaction();
    when(mockTxCompressor.getCompressedSize(tx1)).thenReturn(fastExecutionPathLimit - 1);
    assertThat(evaluateTx(selector, wrapTx(tx1))).isEqualTo(SELECTED);

    // tx2: size 2 pushes conservative cumulative above fastExecutionPathLimit → slow execution
    // path.
    // appendBlock returns compressedSizeAfter = fastExecutionPathLimit / 2.
    final var tx2 = txFactory.createTransaction();
    when(mockTxCompressor.getCompressedSize(tx2)).thenReturn(2);
    assertThat(evaluateTx(selector, wrapTx(tx2))).isEqualTo(SELECTED);

    // tx3: compressedSizeAfter + tx3Size <= fastExecutionPathLimit, so it fits on the fast
    // execution path.
    // With the fix, no slow-execution-path check is needed.
    final var tx3 = txFactory.createTransaction();
    when(mockTxCompressor.getCompressedSize(tx3)).thenReturn(fastExecutionPathLimit / 2 - 1);
    assertThat(evaluateTx(selector, wrapTx(tx3))).isEqualTo(SELECTED);

    // Only tx2 should have triggered a slow-execution-path compression check.
    verify(mockBlobCompressor, times(1)).appendBlock(any());
  }

  /**
   * Creates a selector with the compressor re-initialized to the given limit, mirroring production
   * behavior where the compressor's dataLimit matches the selector's blobSizeLimit.
   */
  private CompressionAwareTransactionSelector createSelector(final int blobSizeLimit) {
    return createSelector(blobSizeLimit, TEST_HEADER_OVERHEAD);
  }

  private CompressionAwareTransactionSelector createSelector(
      final int blobSizeLimit, final int headerOverhead) {
    final var compressor = compressorWithLimit(blobSizeLimit);
    selectorsStateManager = new SelectorsStateManager();
    final var selector =
        new CompressionAwareTransactionSelector(
            selectorsStateManager, blobSizeLimit, headerOverhead, TX_COMPRESSOR, compressor);
    selectorsStateManager.blockSelectionStarted();
    return selector;
  }

  /**
   * Runs the full pre-processing → post-processing lifecycle for a single transaction and returns
   * the effective selection result. Overflow from the async slow execution path surfaces in
   * post-processing, so callers must use this helper rather than inspecting only the pre-processing
   * result.
   */
  private TransactionSelectionResult evaluateTx(
      final CompressionAwareTransactionSelector selector,
      final TestTransactionEvaluationContext context) {
    final var preResult = selector.evaluateTransactionPreProcessing(context);
    final var processingResult = mock(TransactionProcessingResult.class);
    if (!preResult.equals(SELECTED)) {
      selector.onTransactionNotSelected(context, preResult);
      return preResult;
    }
    final var postResult = selector.evaluateTransactionPostProcessing(context, processingResult);
    if (!postResult.equals(SELECTED)) {
      selector.onTransactionNotSelected(context, postResult);
      return postResult;
    }
    selector.onTransactionSelected(context, processingResult);
    return SELECTED;
  }

  private TestTransactionEvaluationContext wrapTx(final Transaction tx) {
    final PendingTransaction pendingTx = mock(PendingTransaction.class);
    when(pendingTx.getTransaction()).thenReturn(tx);
    return new TestTransactionEvaluationContext(mockBlockHeader(), pendingTx);
  }

  private ProcessableBlockHeader mockBlockHeader() {
    final ProcessableBlockHeader header = mock(ProcessableBlockHeader.class);
    when(header.getNumber()).thenReturn(1L);
    when(header.getTimestamp()).thenReturn(1700000000L);
    when(header.getCoinbase()).thenReturn(Address.ZERO);
    when(header.getGasLimit()).thenReturn(30_000_000L);
    when(header.getParentHash()).thenReturn(Hash.wrap(Bytes32.ZERO));
    when(header.getDifficulty()).thenReturn(Difficulty.ZERO);
    when(header.getBaseFee()).thenAnswer(__ -> java.util.Optional.of(Wei.ZERO));
    when(header.getPrevRandao()).thenReturn(java.util.Optional.empty());
    when(header.getParentBeaconBlockRoot()).thenReturn(java.util.Optional.empty());
    return header;
  }
}
