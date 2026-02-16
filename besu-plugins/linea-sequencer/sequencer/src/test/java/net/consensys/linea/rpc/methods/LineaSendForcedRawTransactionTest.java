/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.rpc.methods;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.anyList;
import static org.mockito.Mockito.doAnswer;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;
import java.util.concurrent.TimeUnit;
import net.consensys.linea.rpc.methods.LineaSendForcedRawTransaction.ForcedTransactionResponse;
import net.consensys.linea.sequencer.forced.ForcedTransaction;
import net.consensys.linea.sequencer.forced.ForcedTransactionPoolService;
import net.consensys.linea.sequencer.forced.LineaForcedTransactionPool;
import net.consensys.linea.utils.TestTransactionFactory;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.hyperledger.besu.plugin.services.exception.PluginRpcEndpointException;
import org.hyperledger.besu.plugin.services.rpc.PluginRpcRequest;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

class LineaSendForcedRawTransactionTest {

  private LineaSendForcedRawTransaction method;
  private LineaForcedTransactionPool pool;
  private TestTransactionFactory txFactory;

  @BeforeEach
  void setUp() {
    pool = new LineaForcedTransactionPool(100, null);
    method = new LineaSendForcedRawTransaction().init(pool);
    txFactory = new TestTransactionFactory();
  }

  @Test
  void getNamespace_returnsLinea() {
    assertThat(method.getNamespace()).isEqualTo("linea");
  }

  @Test
  void getName_returnsCorrectName() {
    assertThat(method.getName()).isEqualTo("sendForcedRawTransaction");
  }

  @Test
  void execute_acceptsValidTransaction() {
    final Transaction tx = txFactory.createTransaction();
    final String rawTx = TestTransactionFactory.encodeTransaction(tx);

    final List<ForcedTransactionResponse> result =
        method.execute(request(new ForcedTxParam(6L, rawTx, "0x1000")));

    assertThat(result).hasSize(1);
    assertThat(result.get(0).forcedTransactionNumber).isEqualTo(6L);
    assertThat(result.get(0).hash).isEqualTo(tx.getHash().toHexString());
    assertThat(result.get(0).error).isNull();
    assertThat(pool.pendingCount()).isEqualTo(1);
  }

  @Test
  void execute_acceptsMultipleTransactions() {
    final Transaction tx1 = txFactory.createTransaction();
    final Transaction tx2 = txFactory.createTransaction();

    final List<ForcedTransactionResponse> result =
        method.execute(
            request(
                new ForcedTxParam(6L, TestTransactionFactory.encodeTransaction(tx1), "0x1000"),
                new ForcedTxParam(7L, TestTransactionFactory.encodeTransaction(tx2), "0x2000")));

    assertThat(result).hasSize(2);
    assertThat(result.get(0).forcedTransactionNumber).isEqualTo(6L);
    assertThat(result.get(0).hash).isEqualTo(tx1.getHash().toHexString());
    assertThat(result.get(0).error).isNull();
    assertThat(result.get(1).forcedTransactionNumber).isEqualTo(7L);
    assertThat(result.get(1).hash).isEqualTo(tx2.getHash().toHexString());
    assertThat(result.get(1).error).isNull();
    assertThat(pool.pendingCount()).isEqualTo(2);
  }

  @Test
  void execute_parsesDeadlineCorrectly() {
    final Transaction tx = txFactory.createTransaction();

    method.execute(
        request(new ForcedTxParam(100L, TestTransactionFactory.encodeTransaction(tx), "0x1388")));

    // Verify transaction was added to the pool
    assertThat(pool.pendingCount()).isEqualTo(1);
  }

  @Test
  void execute_returnsErrorForNullDeadline() {
    final Transaction tx = txFactory.createTransaction();

    final List<ForcedTransactionResponse> result =
        method.execute(
            request(new ForcedTxParam(6L, TestTransactionFactory.encodeTransaction(tx), null)));

    assertThat(result).hasSize(1);
    assertThat(result.get(0).forcedTransactionNumber).isEqualTo(6L);
    assertThat(result.get(0).hash).isNull();
    assertThat(result.get(0).error).contains("Deadline block number is required");
    assertThat(pool.pendingCount()).isZero();
  }

  @Test
  void execute_returnsErrorForEmptyDeadline() {
    final Transaction tx = txFactory.createTransaction();

    final List<ForcedTransactionResponse> result =
        method.execute(
            request(new ForcedTxParam(6L, TestTransactionFactory.encodeTransaction(tx), "")));

    assertThat(result).hasSize(1);
    assertThat(result.get(0).forcedTransactionNumber).isEqualTo(6L);
    assertThat(result.get(0).hash).isNull();
    assertThat(result.get(0).error).contains("Deadline block number is required");
    assertThat(pool.pendingCount()).isZero();
  }

  @Test
  void execute_rejectsEmptyParams() {
    assertThatThrownBy(() -> method.execute(request()))
        .isInstanceOf(PluginRpcEndpointException.class);
  }

  @Test
  void execute_returnsErrorForEmptyTransactionData() {
    final List<ForcedTransactionResponse> result =
        method.execute(request(new ForcedTxParam(6L, "", "0x100")));

    assertThat(result).hasSize(1);
    assertThat(result.get(0).forcedTransactionNumber).isEqualTo(6L);
    assertThat(result.get(0).hash).isNull();
    assertThat(result.get(0).error).contains("Empty transaction data");
    assertThat(pool.pendingCount()).isZero();
  }

  @Test
  void execute_returnsErrorForInvalidDeadlineFormat() {
    final String rawTx = TestTransactionFactory.encodeTransaction(txFactory.createTransaction());

    final List<ForcedTransactionResponse> result =
        method.execute(request(new ForcedTxParam(6L, rawTx, "not-a-number")));

    assertThat(result).hasSize(1);
    assertThat(result.get(0).forcedTransactionNumber).isEqualTo(6L);
    assertThat(result.get(0).hash).isNull();
    assertThat(result.get(0).error).contains("Invalid deadlineBlockNumber format");
    assertThat(pool.pendingCount()).isZero();
  }

  @Test
  void execute_returnsErrorForInvalidTransactionData() {
    final List<ForcedTransactionResponse> result =
        method.execute(request(new ForcedTxParam(6L, "0xdeadbeef", "0x1000")));

    assertThat(result).hasSize(1);
    assertThat(result.get(0).forcedTransactionNumber).isEqualTo(6L);
    assertThat(result.get(0).hash).isNull();
    assertThat(result.get(0).error).contains("Failed to decode");
    assertThat(pool.pendingCount()).isZero();
  }

  @Test
  void execute_returnsErrorForMissingForcedTransactionNumber() {
    final String rawTx = TestTransactionFactory.encodeTransaction(txFactory.createTransaction());

    final List<ForcedTransactionResponse> result =
        method.execute(request(new ForcedTxParam(null, rawTx, "0x1000")));

    assertThat(result).hasSize(1);
    assertThat(result.get(0).forcedTransactionNumber).isEqualTo(0L);
    assertThat(result.get(0).hash).isNull();
    assertThat(result.get(0).error).contains("forcedTransactionNumber is required");
    assertThat(pool.pendingCount()).isZero();
  }

  @Test
  void execute_rejectsRequestWithDuplicateForcedTransactionNumbers() {
    final Transaction tx1 = txFactory.createTransaction();
    final Transaction tx2 = txFactory.createTransaction();

    assertThatThrownBy(
            () ->
                method.execute(
                    request(
                        new ForcedTxParam(
                            6L, TestTransactionFactory.encodeTransaction(tx1), "0x1000"),
                        new ForcedTxParam(
                            6L, TestTransactionFactory.encodeTransaction(tx2), "0x2000"))))
        .isInstanceOf(PluginRpcEndpointException.class)
        .hasMessageContaining("Duplicate forcedTransactionNumber: 6");

    assertThat(pool.pendingCount()).isZero();
  }

  @Test
  void execute_partialSuccessWhenMiddleTransactionFails() {
    // Create 4 transactions: #1, #2 valid, #3 has bad deadlineBlockNumber, #4 valid
    final Transaction tx1 = txFactory.createTransaction();
    final Transaction tx2 = txFactory.createTransaction();
    final Transaction tx3 = txFactory.createTransaction();
    final Transaction tx4 = txFactory.createTransaction();

    final List<ForcedTransactionResponse> result =
        method.execute(
            request(
                new ForcedTxParam(6L, TestTransactionFactory.encodeTransaction(tx1), "0x1000"),
                new ForcedTxParam(7L, TestTransactionFactory.encodeTransaction(tx2), "0x2000"),
                new ForcedTxParam(
                    8L, TestTransactionFactory.encodeTransaction(tx3), "bad-deadlineBlockNumber"),
                new ForcedTxParam(9L, TestTransactionFactory.encodeTransaction(tx4), "0x4000")));

    // Should return 3 elements: 2 success + 1 error (tx4 is not included)
    assertThat(result).hasSize(3);
    assertThat(result.get(0).forcedTransactionNumber).isEqualTo(6L);
    assertThat(result.get(0).hash).isEqualTo(tx1.getHash().toHexString());
    assertThat(result.get(0).error).isNull();
    assertThat(result.get(1).forcedTransactionNumber).isEqualTo(7L);
    assertThat(result.get(1).hash).isEqualTo(tx2.getHash().toHexString());
    assertThat(result.get(1).error).isNull();
    assertThat(result.get(2).forcedTransactionNumber).isEqualTo(8L);
    assertThat(result.get(2).hash).isNull();
    assertThat(result.get(2).error).contains("Invalid deadlineBlockNumber format");

    // Only first 2 transactions should be in the pool
    assertThat(pool.pendingCount()).isEqualTo(2);
  }

  @Test
  void execute_sortsTransactionsByForcedTransactionNumberBeforeAddingToPool() {
    final List<ForcedTransaction> capturedTransactions = new ArrayList<>();
    final ForcedTransactionPoolService mockPool = mock(ForcedTransactionPoolService.class);
    doAnswer(
            invocation -> {
              capturedTransactions.addAll(invocation.getArgument(0));
              return null;
            })
        .when(mockPool)
        .addForcedTransactions(anyList());

    final LineaSendForcedRawTransaction methodWithMock =
        new LineaSendForcedRawTransaction().init(mockPool);

    final Transaction tx1 = txFactory.createTransaction();
    final Transaction tx2 = txFactory.createTransaction();
    final Transaction tx3 = txFactory.createTransaction();

    // Submit transactions out of order: 9, 5, 7
    methodWithMock.execute(
        request(
            new ForcedTxParam(9L, TestTransactionFactory.encodeTransaction(tx1), "0x1000"),
            new ForcedTxParam(5L, TestTransactionFactory.encodeTransaction(tx2), "0x2000"),
            new ForcedTxParam(7L, TestTransactionFactory.encodeTransaction(tx3), "0x3000")));

    verify(mockPool).addForcedTransactions(anyList());
    assertThat(capturedTransactions).hasSize(3);
    // Verify they are sorted by forcedTransactionNumber: 5, 7, 9
    assertThat(capturedTransactions.get(0).forcedTransactionNumber()).isEqualTo(5L);
    assertThat(capturedTransactions.get(1).forcedTransactionNumber()).isEqualTo(7L);
    assertThat(capturedTransactions.get(2).forcedTransactionNumber()).isEqualTo(9L);
  }

  @Test
  void execute_rejectsConcurrentRequests() throws Exception {
    // Use a mock pool that blocks to simulate slow processing
    final CountDownLatch firstRequestStarted = new CountDownLatch(1);
    final CountDownLatch allowFirstRequestToComplete = new CountDownLatch(1);

    final ForcedTransactionPoolService mockPool = mock(ForcedTransactionPoolService.class);
    doAnswer(
            invocation -> {
              firstRequestStarted.countDown();
              allowFirstRequestToComplete.await(5, TimeUnit.SECONDS);
              return null;
            })
        .when(mockPool)
        .addForcedTransactions(anyList());

    final LineaSendForcedRawTransaction methodWithMock =
        new LineaSendForcedRawTransaction().init(mockPool);

    final Transaction tx1 = txFactory.createTransaction();
    final Transaction tx2 = txFactory.createTransaction();

    final ExecutorService executor = Executors.newFixedThreadPool(2);
    try {
      // Start first request
      final Future<List<ForcedTransactionResponse>> future1 =
          executor.submit(
              () ->
                  methodWithMock.execute(
                      request(
                          new ForcedTxParam(
                              1L, TestTransactionFactory.encodeTransaction(tx1), "0x1000"))));

      // Wait for first request to start processing
      assertThat(firstRequestStarted.await(5, TimeUnit.SECONDS)).isTrue();

      // Second request while first is blocked should fail immediately
      assertThatThrownBy(
              () ->
                  methodWithMock.execute(
                      request(
                          new ForcedTxParam(
                              2L, TestTransactionFactory.encodeTransaction(tx2), "0x2000"))))
          .isInstanceOf(PluginRpcEndpointException.class)
          .hasMessageContaining("Another request is already being processed");

      // Allow first request to complete
      allowFirstRequestToComplete.countDown();

      // First request should complete successfully
      final List<ForcedTransactionResponse> result1 = future1.get(5, TimeUnit.SECONDS);
      assertThat(result1).hasSize(1);
      assertThat(result1.get(0).error).isNull();
    } finally {
      executor.shutdownNow();
    }
  }

  private PluginRpcRequest request(final ForcedTxParam... params) {
    final PluginRpcRequest req = mock(PluginRpcRequest.class);
    final LineaSendForcedRawTransaction.ForcedTransactionParam[] converted =
        new LineaSendForcedRawTransaction.ForcedTransactionParam[params.length];
    for (int i = 0; i < params.length; i++) {
      converted[i] =
          new LineaSendForcedRawTransaction.ForcedTransactionParam(
              params[i].forcedTransactionNumber,
              params[i].transaction,
              params[i].deadlineBlockNumber);
    }
    when(req.getParams()).thenReturn(new Object[] {converted});
    return req;
  }

  private record ForcedTxParam(
      Long forcedTransactionNumber, String transaction, String deadlineBlockNumber) {}
}
