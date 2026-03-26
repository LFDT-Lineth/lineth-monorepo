/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.rpc.methods;

import static java.util.Optional.empty;
import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.Mockito.RETURNS_DEEP_STUBS;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.spy;
import static org.mockito.Mockito.when;

import java.nio.file.Path;
import java.util.List;
import java.util.Optional;
import java.util.UUID;
import net.consensys.linea.bundles.BundleParameter;
import net.consensys.linea.bundles.LineaLimitedBundlePool;
import org.hyperledger.besu.crypto.SignatureAlgorithmType;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.datatypes.PendingTransaction;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.hyperledger.besu.ethereum.core.TransactionTestFixture;
import org.hyperledger.besu.plugin.services.BesuEvents;
import org.hyperledger.besu.plugin.services.BlockchainService;
import org.hyperledger.besu.plugin.services.rpc.PluginRpcRequest;
import org.hyperledger.besu.plugin.services.txvalidator.PluginTransactionPoolValidator;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

class LineaSendBundleTest {
  private static final long CHAIN_HEAD_BLOCK_NUMBER = 100L;
  private static final long MAX_GAS_LIMIT_PER_TX = 1_000_000L;
  @TempDir Path dataDir;
  private LineaSendBundle lineaSendBundle;
  private BesuEvents mockEvents;
  private LineaLimitedBundlePool bundlePool;
  private BlockchainService blockchainService;

  private Transaction mockTX1 =
      new TransactionTestFixture()
          .nonce(1)
          .gasLimit(21000)
          .createTransaction(
              SignatureAlgorithmType.DEFAULT_SIGNATURE_ALGORITHM_TYPE.get().generateKeyPair());

  private Transaction mockTX2 =
      new TransactionTestFixture()
          .nonce(1)
          .gasLimit(21000)
          .createTransaction(
              SignatureAlgorithmType.DEFAULT_SIGNATURE_ALGORITHM_TYPE.get().generateKeyPair());

  private Transaction mockTX3 =
      new TransactionTestFixture()
          .nonce(1)
          .gasLimit(MAX_GAS_LIMIT_PER_TX + 1)
          .createTransaction(
              SignatureAlgorithmType.DEFAULT_SIGNATURE_ALGORITHM_TYPE.get().generateKeyPair());

  @BeforeEach
  void setup() {
    mockEvents = mock(BesuEvents.class);
    blockchainService = mock(BlockchainService.class, RETURNS_DEEP_STUBS);
    when(blockchainService.getChainHeadHeader().getNumber()).thenReturn(CHAIN_HEAD_BLOCK_NUMBER);
    bundlePool = spy(new LineaLimitedBundlePool(dataDir, 4096L, mockEvents, blockchainService));
    lineaSendBundle =
        new LineaSendBundle(blockchainService).init(bundlePool, createTransactionValidator());
  }

  private PluginTransactionPoolValidator createTransactionValidator() {
    return (tx, isLocal, hasPriority) ->
        tx.getGasLimit() > MAX_GAS_LIMIT_PER_TX
            ? Optional.of("Gas limit exceeded")
            : Optional.empty();
  }

  @Test
  void testExecute_ValidBundle() {
    List<String> transactions = List.of(mockTX1.encoded().toHexString());
    var expectedTxBundleHash = Hash.hash(mockTX1.encoded());

    Optional<Long> minTimestamp = Optional.of(1000L);
    Optional<Long> maxTimestamp = Optional.of(System.currentTimeMillis() + 5000L);

    BundleParameter bundleParams =
        new BundleParameter(
            transactions,
            CHAIN_HEAD_BLOCK_NUMBER + 1,
            minTimestamp,
            maxTimestamp,
            empty(),
            empty(),
            empty());

    PluginRpcRequest request = mock(PluginRpcRequest.class);
    when(request.getParams()).thenReturn(new Object[] {bundleParams});

    // Execute
    LineaSendBundle.BundleResponse response = lineaSendBundle.execute(request);

    // Validate response
    assertNotNull(response);
    assertEquals(expectedTxBundleHash.getBytes().toHexString(), response.bundleHash());
  }

  @Test
  void testExecute_ValidBundle_withReplacement() {
    List<String> transactions = List.of(mockTX1.encoded().toHexString());
    UUID replId = UUID.randomUUID();
    var expectedUUIDBundleHash = LineaLimitedBundlePool.UUIDToHash(replId);

    Optional<Long> minTimestamp = Optional.of(1000L);
    Optional<Long> maxTimestamp = Optional.of(System.currentTimeMillis() + 5000L);

    BundleParameter bundleParams =
        new BundleParameter(
            transactions,
            CHAIN_HEAD_BLOCK_NUMBER + 1,
            minTimestamp,
            maxTimestamp,
            empty(),
            Optional.of(replId.toString()),
            empty());

    PluginRpcRequest request = mock(PluginRpcRequest.class);
    when(request.getParams()).thenReturn(new Object[] {bundleParams});

    // Execute
    LineaSendBundle.BundleResponse response = lineaSendBundle.execute(request);

    // Validate response
    assertNotNull(response);
    assertEquals(expectedUUIDBundleHash.getBytes().toHexString(), response.bundleHash());

    // Replace bundle:
    transactions = List.of(mockTX2.encoded().toHexString(), mockTX1.encoded().toHexString());
    bundleParams =
        new BundleParameter(
            transactions,
            CHAIN_HEAD_BLOCK_NUMBER + 2,
            minTimestamp,
            maxTimestamp,
            empty(),
            Optional.of(replId.toString()),
            empty());
    when(request.getParams()).thenReturn(new Object[] {bundleParams});

    // re-execute
    response = lineaSendBundle.execute(request);

    // Validate response
    assertNotNull(response);
    assertEquals(expectedUUIDBundleHash.getBytes().toHexString(), response.bundleHash());

    // assert the new block number:
    assertTrue(
        bundlePool.get(expectedUUIDBundleHash).blockNumber().equals(CHAIN_HEAD_BLOCK_NUMBER + 2));
    List<? extends PendingTransaction> pts =
        bundlePool.get(expectedUUIDBundleHash).pendingTransactions();
    // assert the new tx2 is present
    assertTrue(pts.stream().map(pt -> pt.getTransaction()).anyMatch(t -> t.equals(mockTX2)));
  }

  @Test
  void testExecute_ExpiredBundle() {
    List<String> transactions = List.of(mockTX1.encoded().toHexString());
    Optional<Long> maxTimestamp = Optional.of(5000L);
    BundleParameter bundleParams =
        new BundleParameter(
            transactions,
            CHAIN_HEAD_BLOCK_NUMBER + 1,
            empty(),
            maxTimestamp,
            empty(),
            empty(),
            empty());

    PluginRpcRequest request = mock(PluginRpcRequest.class);
    when(request.getParams()).thenReturn(new Object[] {bundleParams});

    assertThatThrownBy(() -> lineaSendBundle.execute(request))
        .isInstanceOf(RuntimeException.class)
        .hasMessageMatching(
            "Bundle max timestamp [0-9]+ is in the past, current timestamp is [0-9]+");
  }

  @Test
  void testExecute_BundleForBlockAlreadyInChain_ThrowsException() {
    List<String> transactions = List.of(mockTX1.encoded().toHexString());
    BundleParameter bundleParams =
        new BundleParameter(
            transactions, CHAIN_HEAD_BLOCK_NUMBER, empty(), empty(), empty(), empty(), empty());

    PluginRpcRequest request = mock(PluginRpcRequest.class);
    when(request.getParams()).thenReturn(new Object[] {bundleParams});

    assertThatThrownBy(() -> lineaSendBundle.execute(request))
        .isInstanceOf(RuntimeException.class)
        .hasMessageContaining(
            "Bundle block number 100 is not greater than current chain head block number 100");
  }

  @Test
  void testExecute_InvalidRequest_ThrowsException() {
    PluginRpcRequest request = mock(PluginRpcRequest.class);
    when(request.getParams()).thenReturn(new Object[] {"invalid_param"});

    Exception exception =
        assertThrows(
            RuntimeException.class,
            () -> {
              lineaSendBundle.execute(request);
            });

    assertTrue(exception.getMessage().contains("Malformed linea_sendBundle json param"));
  }

  @Test
  void testExecute_InvalidTransaction_ThrowsException() {
    List<String> transactions =
        List.of(mockTX1.encoded().toHexString(), mockTX3.encoded().toHexString());
    BundleParameter bundleParams =
        new BundleParameter(
            transactions, CHAIN_HEAD_BLOCK_NUMBER + 1, empty(), empty(), empty(), empty(), empty());

    PluginRpcRequest request = mock(PluginRpcRequest.class);
    when(request.getParams()).thenReturn(new Object[] {bundleParams});

    assertThatThrownBy(() -> lineaSendBundle.execute(request))
        .isInstanceOf(RuntimeException.class)
        .hasMessageContaining(
            "Invalid transaction in bundle: hash %s, reason: Gas limit exceeded"
                .formatted(mockTX3.getHash()));
  }

  @Test
  void testExecute_DuplicateRequest_ThrowsException() {
    List<String> transactions = List.of(mockTX1.encoded().toHexString());
    BundleParameter bundleParams1 =
        new BundleParameter(
            transactions, CHAIN_HEAD_BLOCK_NUMBER + 1, empty(), empty(), empty(), empty(), empty());

    PluginRpcRequest request1 = mock(PluginRpcRequest.class);
    when(request1.getParams()).thenReturn(new Object[] {bundleParams1});

    LineaSendBundle.BundleResponse response1 = lineaSendBundle.execute(request1);

    // first time we send the request it works
    assertThat(response1.bundleHash())
        .isEqualTo(Hash.hash(mockTX1.encoded()).getBytes().toHexString());

    BundleParameter bundleParams2 =
        new BundleParameter(
            transactions, CHAIN_HEAD_BLOCK_NUMBER + 1, empty(), empty(), empty(), empty(), empty());

    PluginRpcRequest request2 = mock(PluginRpcRequest.class);
    when(request2.getParams()).thenReturn(new Object[] {bundleParams2});

    // same request sent again return already seen
    assertThatThrownBy(() -> lineaSendBundle.execute(request2))
        .isInstanceOf(RuntimeException.class)
        .hasMessageMatching("request already seen PT[0-9]+\\.[0-9]+S ago");
  }

  @Test
  void testExecute_EmptyTransactions_ThrowsException() {
    BundleParameter bundleParams =
        new BundleParameter(List.of(), 123L, empty(), empty(), empty(), empty(), empty());

    PluginRpcRequest request = mock(PluginRpcRequest.class);
    when(request.getParams()).thenReturn(new Object[] {bundleParams});

    Exception exception =
        assertThrows(
            RuntimeException.class,
            () -> {
              lineaSendBundle.execute(request);
            });

    assertTrue(exception.getMessage().contains("Malformed bundle, no bundle transactions present"));
  }

  @Test
  void testExecute_FrozenPool_ThrowsException() {
    List<String> transactions = List.of(mockTX1.encoded().toHexString());
    BundleParameter bundleParams =
        new BundleParameter(transactions, 123L, empty(), empty(), empty(), empty(), empty());

    PluginRpcRequest request = mock(PluginRpcRequest.class);
    when(request.getParams()).thenReturn(new Object[] {bundleParams});

    // saving to disk freeze the pool
    bundlePool.saveToDisk();

    assertTrue(bundlePool.isFrozen());

    assertThatThrownBy(() -> lineaSendBundle.execute(request))
        .isInstanceOf(RuntimeException.class)
        .hasMessageMatching("Bundle pool is not accepting modifications");
  }
}
