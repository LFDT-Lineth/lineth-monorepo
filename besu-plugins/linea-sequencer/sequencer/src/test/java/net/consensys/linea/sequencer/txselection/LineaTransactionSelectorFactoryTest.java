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
package net.consensys.linea.sequencer.txselection;

import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.RETURNS_DEEP_STUBS;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.spy;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.verifyNoInteractions;
import static org.mockito.Mockito.when;

import java.io.IOException;
import java.math.BigInteger;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.stream.Stream;

import net.consensys.linea.bundles.BundlePoolService;
import net.consensys.linea.bundles.LineaLimitedBundlePool;
import net.consensys.linea.bundles.TransactionBundle;
import net.consensys.linea.config.LineaProfitabilityConfiguration;
import net.consensys.linea.config.LineaTracerConfiguration;
import net.consensys.linea.config.LineaTransactionSelectorConfiguration;
import net.consensys.linea.plugins.config.LineaL1L2BridgeSharedConfiguration;
import net.consensys.linea.sequencer.modulelimit.ModuleLineCountValidator;
import net.consensys.linea.sequencer.txselection.selectors.TraceLineLimitTransactionSelectorTest;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.hyperledger.besu.plugin.data.ProcessableBlockHeader;
import org.hyperledger.besu.plugin.data.TransactionSelectionResult;
import org.hyperledger.besu.plugin.services.BesuEvents;
import org.hyperledger.besu.plugin.services.BlockchainService;
import org.hyperledger.besu.plugin.services.txselection.BlockTransactionSelectionService;
import org.hyperledger.besu.plugin.services.txselection.SelectorsStateManager;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.ArgumentsProvider;
import org.junit.jupiter.params.provider.ArgumentsSource;

class LineaTransactionSelectorFactoryTest {
  private static final String MODULE_LINE_LIMITS_RESOURCE_NAME = "/sequencer/line-limits.toml";

  private static final Address BRIDGE_CONTRACT =
      Address.fromHexString("0x508Ca82Df566dCD1B0DE8296e70a96332cD644ec");
  private static final Bytes BRIDGE_LOG_TOPIC =
      Bytes.fromHexString("e856c2b8bd4eb0027ce32eeaf595c21b0b6b4644b326e5b7bd80a1cf8db72e6c");

  private BlockchainService mockBlockchainService;
  private LineaTransactionSelectorConfiguration mockTxSelectorConfiguration;
  private LineaL1L2BridgeSharedConfiguration l1L2BridgeConfiguration;
  private LineaProfitabilityConfiguration mockProfitabilityConfiguration;
  private Map<String, Integer> lineCountLimits;
  private BesuEvents mockEvents;
  private LineaLimitedBundlePool bundlePool;
  private BundlePoolService mockBundlePool;
  private LineaTracerConfiguration lineaTracerConfiguration;
  private LineaTransactionSelectorFactory factory;

  @TempDir static Path tempDir;
  @TempDir Path dataDir;
  static Path lineLimitsConfPath;

  @BeforeAll
  public static void beforeAll() throws IOException {
    lineLimitsConfPath = tempDir.resolve("line-limits.toml");
    Files.copy(
        TraceLineLimitTransactionSelectorTest.class.getResourceAsStream(
            MODULE_LINE_LIMITS_RESOURCE_NAME),
        lineLimitsConfPath);
  }

  @BeforeEach
  void setUp() {
    lineaTracerConfiguration =
        LineaTracerConfiguration.builder()
            .moduleLimitsFilePath(lineLimitsConfPath.toString())
            .build();
    lineCountLimits =
        new HashMap<>(ModuleLineCountValidator.createLimitModules(lineaTracerConfiguration));

    mockBlockchainService = mock(BlockchainService.class);
    when(mockBlockchainService.getChainId()).thenReturn(Optional.of(BigInteger.ONE));
    when(mockBlockchainService.getNextBlockBaseFee()).thenReturn(Optional.of(Wei.of(7)));
    mockTxSelectorConfiguration = mock(LineaTransactionSelectorConfiguration.class);
    l1L2BridgeConfiguration =
        new LineaL1L2BridgeSharedConfiguration(BRIDGE_CONTRACT, BRIDGE_LOG_TOPIC);
    mockProfitabilityConfiguration = mock(LineaProfitabilityConfiguration.class);
    mockEvents = mock(BesuEvents.class);
    bundlePool = spy(new LineaLimitedBundlePool(dataDir, 4096, mockEvents, mockBlockchainService));

    factory =
        new LineaTransactionSelectorFactory(
            mockBlockchainService,
            mockTxSelectorConfiguration,
            l1L2BridgeConfiguration,
            mockProfitabilityConfiguration,
            lineaTracerConfiguration,
            lineCountLimits,
            Optional.empty(),
            Optional.empty(),
            bundlePool);
    factory.create(new SelectorsStateManager());
  }

  @Test
  void testSelectPendingTransactions_WithBundles() {
    var mockBts = mock(BlockTransactionSelectionService.class);
    var mockPendingBlockHeader = mock(ProcessableBlockHeader.class);
    when(mockPendingBlockHeader.getNumber()).thenReturn(1L);

    var mockHash = Hash.wrap(Bytes32.random());
    var mockBundle = createBundle(mockHash, 1L, Optional.empty());
    bundlePool.putOrReplace(mockHash, mockBundle);

    when(mockBts.evaluatePendingTransaction(any())).thenReturn(TransactionSelectionResult.SELECTED);

    factory.selectPendingTransactions(mockBts, mockPendingBlockHeader);

    verify(mockBts).commit();
  }

  @ParameterizedTest()
  @ArgumentsSource(FailedTransactionSelectionResultProvider.class)
  void testSelectPendingTransactions_WithFailedBundle(TransactionSelectionResult failStatus) {
    var mockBts = mock(BlockTransactionSelectionService.class);
    var mockPendingBlockHeader = mock(ProcessableBlockHeader.class);
    when(mockPendingBlockHeader.getNumber()).thenReturn(1L);

    var mockHash = Hash.wrap(Bytes32.random());
    var mockBundle = createBundle(mockHash, 1L, Optional.empty());
    bundlePool.putOrReplace(mockHash, mockBundle);

    when(mockBts.evaluatePendingTransaction(any())).thenReturn(failStatus);

    factory.selectPendingTransactions(mockBts, mockPendingBlockHeader);

    verify(mockBts).rollback();
  }

  @Test
  void testSelectPendingTransactions_WithoutBundles() {
    var mockBts = mock(BlockTransactionSelectionService.class);
    var mockPendingBlockHeader = mock(ProcessableBlockHeader.class);
    when(mockPendingBlockHeader.getNumber()).thenReturn(1L);

    factory.selectPendingTransactions(mockBts, mockPendingBlockHeader);

    verifyNoInteractions(mockBts);
  }

  private TransactionBundle createBundle(
      Hash hash, long blockNumber, Optional<Transaction> optPendingTx) {
    return new TransactionBundle(
        hash,
        List.of(
            optPendingTx.isPresent()
                ? optPendingTx.get()
                : mock(Transaction.class, RETURNS_DEEP_STUBS)),
        blockNumber,
        Optional.empty(),
        Optional.empty(),
        Optional.empty(),
        Optional.empty());
  }

  static class FailedTransactionSelectionResultProvider implements ArgumentsProvider {
    @Override
    public Stream<? extends Arguments> provideArguments(
        org.junit.jupiter.api.extension.ExtensionContext context) {
      return Stream.of(
          Arguments.of(TransactionSelectionResult.BLOCK_FULL),
          Arguments.of(TransactionSelectionResult.BLOBS_FULL),
          Arguments.of(TransactionSelectionResult.BLOCK_SELECTION_TIMEOUT),
          Arguments.of(TransactionSelectionResult.BLOCK_SELECTION_TIMEOUT_INVALID_TX),
          Arguments.of(TransactionSelectionResult.TX_EVALUATION_TOO_LONG),
          Arguments.of(TransactionSelectionResult.INVALID_TX_EVALUATION_TOO_LONG),
          Arguments.of(TransactionSelectionResult.BLOCK_OCCUPANCY_ABOVE_THRESHOLD),
          Arguments.of(TransactionSelectionResult.TX_TOO_LARGE_FOR_REMAINING_GAS),
          Arguments.of(TransactionSelectionResult.TX_TOO_LARGE_FOR_REMAINING_BLOB_GAS));
    }
  }
}
