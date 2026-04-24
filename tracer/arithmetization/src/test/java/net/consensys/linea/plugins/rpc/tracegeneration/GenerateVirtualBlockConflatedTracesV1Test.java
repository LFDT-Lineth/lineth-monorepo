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

package net.consensys.linea.plugins.rpc.tracegeneration;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

import java.io.IOException;
import java.math.BigInteger;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Arrays;
import java.util.Objects;
import java.util.Optional;
import net.consensys.linea.plugins.config.LineaL1L2BridgeSharedConfiguration;
import net.consensys.linea.plugins.rpc.RequestLimiter;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.crypto.KeyPair;
import org.hyperledger.besu.crypto.SignatureAlgorithm;
import org.hyperledger.besu.crypto.SignatureAlgorithmFactory;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.hyperledger.besu.ethereum.rlp.BytesValueRLPOutput;
import org.hyperledger.besu.plugin.ServiceManager;
import org.hyperledger.besu.plugin.services.BlockSimulationService;
import org.hyperledger.besu.plugin.services.BlockchainService;
import org.hyperledger.besu.plugin.services.exception.PluginRpcEndpointException;
import org.hyperledger.besu.plugin.services.rpc.PluginRpcRequest;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

/**
 * Tests for GenerateVirtualBlockConflatedTracesV1. These tests verify that the RPC endpoint can
 * properly handle requests and output trace files to the filesystem.
 */
class GenerateVirtualBlockConflatedTracesV1Test {

  private static final long BLOCK_NUMBER = 100L;
  private static final long PARENT_BLOCK_NUMBER = 99L;
  private static final BigInteger CHAIN_ID = BigInteger.valueOf(59144L); // Linea mainnet

  @TempDir Path tempDir;

  private BlockchainService blockchainService;
  private PluginRpcRequest rpcRequest;

  private GenerateVirtualBlockConflatedTracesV1 rpcMethod;

  @BeforeEach
  void setUp() {
    ServiceManager serviceManager = mock(ServiceManager.class);
    BlockSimulationService blockSimulationService = mock(BlockSimulationService.class);
    blockchainService = mock(BlockchainService.class);
    when(serviceManager.getService(BlockSimulationService.class))
        .thenReturn(Optional.of(blockSimulationService));
    when(serviceManager.getService(BlockchainService.class))
        .thenReturn(Optional.of(blockchainService));
    rpcRequest = mock(PluginRpcRequest.class);

    TracesEndpointConfiguration endpointConfiguration =
        TracesEndpointConfiguration.builder()
            .tracesOutputPath(tempDir.toString())
            .caching(true)
            .traceCompression(false)
            .traceFileVersion(1)
            .build();

    LineaL1L2BridgeSharedConfiguration bridgeConfiguration =
        LineaL1L2BridgeSharedConfiguration.builder().build();

    RequestLimiter requestLimiter = RequestLimiter.builder().concurrentRequestsCount(1).build();

    rpcMethod =
        new GenerateVirtualBlockConflatedTracesV1(
            requestLimiter, endpointConfiguration, bridgeConfiguration, serviceManager);
  }

  @Test
  void namespaceAndMethodNameAreCorrect() {
    assertThat(rpcMethod.getNamespace()).isEqualTo("linea");
    assertThat(rpcMethod.getName()).isEqualTo("generateVirtualBlockConflatedTracesToFileV1");
  }

  @Test
  void throwsExceptionWhenParentBlockNotFound() {
    String signedTxRlp = createSignedTransactionRlp();

    VirtualBlockTraceRequestParams params =
        new VirtualBlockTraceRequestParams(BLOCK_NUMBER, new String[] {signedTxRlp});
    when(rpcRequest.getParams()).thenReturn(new Object[] {params});

    when(blockchainService.getBlockByNumber(PARENT_BLOCK_NUMBER)).thenReturn(Optional.empty());

    assertThatThrownBy(() -> rpcMethod.execute(rpcRequest))
        .isInstanceOf(PluginRpcEndpointException.class)
        .hasMessageContaining("Block not found");
  }

  @Test
  void returnsCachedFileWhenAvailable() throws IOException {
    String tracesEngineVersion =
        Objects.requireNonNullElse(TraceRequestParams.getTracerRuntime(), "unknown");
    String signedTxRlp = createSignedTransactionRlp();
    String txsHash = Integer.toHexString(Arrays.hashCode(new String[] {signedTxRlp}));
    String cachedFileName =
        String.format(
            "%d-%s-noncanonical.conflated.%s.lt", BLOCK_NUMBER, txsHash, tracesEngineVersion);
    Path cachedFilePath = tempDir.resolve(cachedFileName);
    Files.writeString(cachedFilePath, "cached trace content");

    VirtualBlockTraceRequestParams params =
        new VirtualBlockTraceRequestParams(BLOCK_NUMBER, new String[] {signedTxRlp});
    when(rpcRequest.getParams()).thenReturn(new Object[] {params});

    TraceFile result = rpcMethod.execute(rpcRequest);

    assertThat(result).isNotNull();
    assertThat(result.conflatedTracesFileName()).isEqualTo(cachedFilePath.toString());
  }

  @Test
  void differentTransactionsDontShareCacheEntries() throws IOException {
    String tracesEngineVersion =
        Objects.requireNonNullElse(TraceRequestParams.getTracerRuntime(), "unknown");

    String txRlp1 = createSignedTransactionRlp();
    String txRlp2 = createSignedTransactionRlpWithNonce(1);

    String txsHash1 = Integer.toHexString(Arrays.hashCode(new String[] {txRlp1}));
    String txsHash2 = Integer.toHexString(Arrays.hashCode(new String[] {txRlp2}));

    // Pre-populate the cache only for txRlp1
    String cachedFileName =
        String.format(
            "%d-%s-noncanonical.conflated.%s.lt", BLOCK_NUMBER, txsHash1, tracesEngineVersion);
    Files.writeString(tempDir.resolve(cachedFileName), "cached trace content");

    // Request with txRlp2 must NOT return the cache entry for txRlp1
    VirtualBlockTraceRequestParams params =
        new VirtualBlockTraceRequestParams(BLOCK_NUMBER, new String[] {txRlp2});
    when(rpcRequest.getParams()).thenReturn(new Object[] {params});
    when(blockchainService.getBlockByNumber(BLOCK_NUMBER - 1)).thenReturn(Optional.empty());

    assertThatThrownBy(() -> rpcMethod.execute(rpcRequest))
        .isInstanceOf(PluginRpcEndpointException.class)
        .hasMessageContaining("Block not found");

    assertThat(txsHash1).isNotEqualTo(txsHash2);
  }

  @Test
  void validatesBlockNumberMustBeAtLeastOne() {
    VirtualBlockTraceRequestParams params =
        new VirtualBlockTraceRequestParams(0L, new String[] {"0x1234"});
    when(rpcRequest.getParams()).thenReturn(new Object[] {params});

    assertThatThrownBy(() -> rpcMethod.execute(rpcRequest))
        .hasMessageContaining("INVALID_BLOCK_NUMBER");
  }

  @Test
  void validatesTransactionsMustNotBeEmpty() {
    VirtualBlockTraceRequestParams params =
        new VirtualBlockTraceRequestParams(BLOCK_NUMBER, new String[] {});
    when(rpcRequest.getParams()).thenReturn(new Object[] {params});

    assertThatThrownBy(() -> rpcMethod.execute(rpcRequest))
        .hasMessageContaining("INVALID_TRANSACTIONS");
  }

  /**
   * Creates a valid signed transaction RLP for testing. This creates a minimal signed transaction
   * that can be decoded by the RPC endpoint.
   */
  private String createSignedTransactionRlp() {
    return createSignedTransactionRlpWithNonce(0);
  }

  private String createSignedTransactionRlpWithNonce(final long nonce) {
    SignatureAlgorithm signatureAlgorithm = SignatureAlgorithmFactory.getInstance();
    BigInteger privateKeyValue =
        new BigInteger("8f2a55949038a9610f50fb23b5883af3b4ecb3c3bb792cbcefbd1542c692be63", 16);
    KeyPair keyPair =
        signatureAlgorithm.createKeyPair(signatureAlgorithm.createPrivateKey(privateKeyValue));

    Transaction tx =
        Transaction.builder()
            .type(org.hyperledger.besu.datatypes.TransactionType.FRONTIER)
            .nonce(nonce)
            .gasPrice(Wei.of(1000000000L))
            .gasLimit(21000)
            .to(Address.fromHexString("0x0000000000000000000000000000000000000001"))
            .value(Wei.of(1))
            .payload(Bytes.EMPTY)
            .chainId(CHAIN_ID)
            .signAndBuild(keyPair);

    BytesValueRLPOutput rlpOutput = new BytesValueRLPOutput();
    tx.writeTo(rlpOutput);
    return rlpOutput.encoded().toHexString();
  }
}
