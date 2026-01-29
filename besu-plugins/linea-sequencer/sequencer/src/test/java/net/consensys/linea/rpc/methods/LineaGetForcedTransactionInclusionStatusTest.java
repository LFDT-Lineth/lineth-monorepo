/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.rpc.methods;

import static net.consensys.linea.sequencer.txselection.LineaTransactionSelectionResult.TX_FILTERED_ADDRESS_TO;
import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.RETURNS_DEEP_STUBS;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

import java.util.List;
import java.util.concurrent.atomic.AtomicLong;
import java.util.stream.Collectors;
import net.consensys.linea.sequencer.forced.ForcedTransaction;
import net.consensys.linea.sequencer.forced.LineaForcedTransactionPool;
import net.consensys.linea.utils.TestTransactionFactory;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.hyperledger.besu.plugin.data.AddedBlockContext;
import org.hyperledger.besu.plugin.data.TransactionSelectionResult;
import org.hyperledger.besu.plugin.services.exception.PluginRpcEndpointException;
import org.hyperledger.besu.plugin.services.rpc.PluginRpcRequest;
import org.hyperledger.besu.plugin.services.txselection.BlockTransactionSelectionService;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

class LineaGetForcedTransactionInclusionStatusTest {

  private static final long TEST_TIMESTAMP = 1_700_000_000L;

  private LineaGetForcedTransactionInclusionStatus method;
  private LineaForcedTransactionPool pool;
  private TestTransactionFactory txFactory;
  private AtomicLong forcedTxNumberGenerator;

  @BeforeEach
  void setUp() {
    pool = new LineaForcedTransactionPool(100, null);
    method = new LineaGetForcedTransactionInclusionStatus().init(pool);
    txFactory = new TestTransactionFactory();
    forcedTxNumberGenerator = new AtomicLong(1);
  }

  @Test
  void getNamespace_returnsLinea() {
    assertThat(method.getNamespace()).isEqualTo("linea");
  }

  @Test
  void getName_returnsCorrectName() {
    assertThat(method.getName()).isEqualTo("getForcedTransactionInclusionStatus");
  }

  @Test
  void execute_returnsNullForUnknownTransaction() {
    final var result = method.execute(request(999L));

    assertThat(result).isNull();
  }

  @Test
  void execute_returnsIncludedStatus() {
    final ForcedTransaction ftx =
        addAndProcessTransaction(TransactionSelectionResult.SELECTED, 100L);

    final var result = method.execute(request(ftx.forcedTransactionNumber()));

    assertThat(result).isNotNull();
    assertThat(result.forcedTransactionNumber).isEqualTo(ftx.forcedTransactionNumber());
    assertThat(result.transactionHash).isEqualTo(ftx.txHash().toHexString());
    assertThat(result.inclusionResult).isEqualTo("Included");
    assertThat(result.blockNumber).isEqualTo("0x64"); // 100 in hex
    assertThat(result.blockTimestamp).isEqualTo(TEST_TIMESTAMP);
    assertThat(result.from).isEqualTo(ftx.transaction().getSender().toHexString());
  }

  @Test
  void execute_returnsBadNonceStatus() {
    final ForcedTransaction ftx =
        addAndProcessTransaction(
            TransactionSelectionResult.invalidTransient("NONCE_TOO_LOW"), 100L);

    final var result = method.execute(request(ftx.forcedTransactionNumber()));

    assertThat(result).isNotNull();
    assertThat(result.inclusionResult).isEqualTo("BadNonce");
    assertThat(result.blockNumber).isEqualTo("0x64");
  }

  @Test
  void execute_returnsBadBalanceStatus() {
    final ForcedTransaction ftx =
        addAndProcessTransaction(
            TransactionSelectionResult.invalidTransient("UPFRONT_COST_EXCEEDS_BALANCE"), 100L);

    final var result = method.execute(request(ftx.forcedTransactionNumber()));

    assertThat(result).isNotNull();
    assertThat(result.inclusionResult).isEqualTo("BadBalance");
  }

  @Test
  void execute_returnsFilteredAddressToStatus() {
    final ForcedTransaction ftx = addAndProcessTransaction(TX_FILTERED_ADDRESS_TO, 100L);

    final var result = method.execute(request(ftx.forcedTransactionNumber()));

    assertThat(result).isNotNull();
    assertThat(result.inclusionResult).isEqualTo("FilteredAddressTo");
  }

  @Test
  void execute_rejectsNullForcedTransactionNumber() {
    assertThatThrownBy(() -> method.execute(request(null)))
        .isInstanceOf(PluginRpcEndpointException.class);
  }

  private ForcedTransaction addAndProcessTransaction(
      final TransactionSelectionResult result, final long blockNumber) {
    final Transaction tx = txFactory.createTransaction();
    final long forcedTxNumber = forcedTxNumberGenerator.getAndIncrement();
    final ForcedTransaction ftx =
        new ForcedTransaction(forcedTxNumber, tx.getHash(), tx, Long.MAX_VALUE);
    pool.addForcedTransactions(List.of(ftx));

    final BlockTransactionSelectionService bts = mock(BlockTransactionSelectionService.class);
    when(bts.evaluatePendingTransaction(any())).thenReturn(result);
    pool.processForBlock(blockNumber, bts);

    // Simulate block added to finalize the status
    final List<ForcedTransaction> includedTxs = result.selected() ? List.of(ftx) : List.of();
    pool.onBlockAdded(createBlockContext(blockNumber, TEST_TIMESTAMP, includedTxs));

    return ftx;
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

  private PluginRpcRequest request(final Long forcedTransactionNumber) {
    final PluginRpcRequest req = mock(PluginRpcRequest.class);
    when(req.getParams()).thenReturn(new Object[] {forcedTransactionNumber});
    return req;
  }
}
