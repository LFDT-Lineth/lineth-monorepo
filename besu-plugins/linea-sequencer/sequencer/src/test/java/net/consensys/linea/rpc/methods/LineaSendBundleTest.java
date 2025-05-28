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
package net.consensys.linea.rpc.methods;

import static java.util.Optional.empty;
import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.hyperledger.besu.ethereum.core.PrivateTransactionDataFixture.SIGNATURE_ALGORITHM;
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
import net.consensys.linea.bundles.TransactionBundle;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.hyperledger.besu.ethereum.core.TransactionTestFixture;
import org.hyperledger.besu.plugin.services.BesuEvents;
import org.hyperledger.besu.plugin.services.BlockchainService;
import org.hyperledger.besu.plugin.services.rpc.PluginRpcRequest;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

class LineaSendBundleTest {
  private static final long CHAIN_HEAD_BLOCK_NUMBER = 100L;
  @TempDir Path dataDir;
  private LineaSendBundle lineaSendBundle;
  private BesuEvents mockEvents;
  private LineaLimitedBundlePool bundlePool;
  private BlockchainService blockchainService;

  private Transaction mockTX1 =
      new TransactionTestFixture()
          .nonce(1)
          .gasLimit(21000)
          .createTransaction(SIGNATURE_ALGORITHM.get().generateKeyPair());

  private Transaction mockTX2 =
      new TransactionTestFixture()
          .nonce(1)
          .gasLimit(21000)
          .createTransaction(SIGNATURE_ALGORITHM.get().generateKeyPair());

  @BeforeEach
  void setup() {
    mockEvents = mock(BesuEvents.class);
    blockchainService = mock(BlockchainService.class, RETURNS_DEEP_STUBS);
    when(blockchainService.getChainHeadHeader().getNumber()).thenReturn(CHAIN_HEAD_BLOCK_NUMBER);
    bundlePool = spy(new LineaLimitedBundlePool(dataDir, 4096L, mockEvents, blockchainService));
    lineaSendBundle = new LineaSendBundle(blockchainService).init(bundlePool);
  }

  @Test
  void testExecute_ValidBundle() {
    List<String> transactions = List.of(mockTX1.encoded().toHexString());
    var expectedTxBundleHash = Hash.hash(mockTX1.encoded());

    Optional<Long> minTimestamp = Optional.of(1000L);
    Optional<Long> maxTimestamp = Optional.of(System.currentTimeMillis() + 5000L);

    BundleParameter bundleParams =
        new BundleParameter(
            transactions, 123L, minTimestamp, maxTimestamp, empty(), empty(), empty());

    PluginRpcRequest request = mock(PluginRpcRequest.class);
    when(request.getParams()).thenReturn(new Object[] {bundleParams});

    // Execute
    LineaSendBundle.BundleResponse response = lineaSendBundle.execute(request);

    // Validate response
    assertNotNull(response);
    assertEquals(expectedTxBundleHash.toHexString(), response.bundleHash());
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
            123L,
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
    assertEquals(expectedUUIDBundleHash.toHexString(), response.bundleHash());

    // Replace bundle:
    transactions = List.of(mockTX2.encoded().toHexString(), mockTX1.encoded().toHexString());
    bundleParams =
        new BundleParameter(
            transactions,
            12345L,
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
    assertEquals(expectedUUIDBundleHash.toHexString(), response.bundleHash());

    // assert the new block number:
    assertTrue(bundlePool.get(expectedUUIDBundleHash).blockNumber().equals(12345L));
    List<TransactionBundle.PendingBundleTx> pts =
        bundlePool.get(expectedUUIDBundleHash).pendingTransactions();
    // assert the new tx2 is present
    assertTrue(pts.stream().map(pt -> pt.getTransaction()).anyMatch(t -> t.equals(mockTX2)));
  }

  @Test
  void testExecute_ExpiredBundle() {
    List<String> transactions = List.of(mockTX1.encoded().toHexString());
    Optional<Long> maxTimestamp = Optional.of(5000L);
    BundleParameter bundleParams =
        new BundleParameter(transactions, 123L, empty(), maxTimestamp, empty(), empty(), empty());

    PluginRpcRequest request = mock(PluginRpcRequest.class);
    when(request.getParams()).thenReturn(new Object[] {bundleParams});

    assertThatThrownBy(() -> lineaSendBundle.execute(request))
        .isInstanceOf(RuntimeException.class)
        .hasMessageMatching(
            "bundle max timestamp [0-9]+ is in the past, current timestamp is [0-9]+");
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
            "bundle block number 100 is not greater than current chain head block number 100");
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

    assertTrue(exception.getMessage().contains("malformed linea_sendBundle json param"));
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
    assertThat(response1.bundleHash()).isEqualTo(Hash.hash(mockTX1.encoded()).toHexString());

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
