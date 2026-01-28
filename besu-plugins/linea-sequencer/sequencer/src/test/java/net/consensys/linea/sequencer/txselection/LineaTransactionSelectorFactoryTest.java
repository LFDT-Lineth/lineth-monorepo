/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.sequencer.txselection;

import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.Mockito.RETURNS_DEEP_STUBS;
import static org.mockito.Mockito.atLeastOnce;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.spy;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.verifyNoInteractions;
import static org.mockito.Mockito.when;

import java.io.IOException;
import java.math.BigInteger;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Collections;
import java.util.HashMap;
import java.util.List;
import java.util.Objects;
import java.util.Optional;
import java.util.concurrent.atomic.AtomicReference;
import java.util.stream.Stream;
import net.consensys.linea.bl.TransactionProfitabilityCalculator;
import net.consensys.linea.bundles.LineaLimitedBundlePool;
import net.consensys.linea.bundles.TransactionBundle;
import net.consensys.linea.config.LineaProfitabilityConfiguration;
import net.consensys.linea.config.LineaTracerConfiguration;
import net.consensys.linea.config.LineaTransactionSelectorConfiguration;
import net.consensys.linea.plugins.config.LineaL1L2BridgeSharedConfiguration;
import net.consensys.linea.sequencer.liveness.LivenessService;
import net.consensys.linea.sequencer.modulelimit.ModuleLineCountValidator;
import net.consensys.linea.sequencer.txselection.selectors.TraceLineLimitTransactionSelectorTest;
import net.consensys.linea.utils.CachingTransactionCompressor;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.HardforkId;
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
  private static final Bytes32 BRIDGE_LOG_TOPIC =
      Bytes32.fromHexString("e856c2b8bd4eb0027ce32eeaf595c21b0b6b4644b326e5b7bd80a1cf8db72e6c");
  private static final long BLOCK_TIMESTAMP = 1753867173L;

  private LineaLimitedBundlePool bundlePool;
  private LineaTransactionSelectorFactory factory;
  private BlockchainService mockBlockchainService;

  @TempDir static Path tempDir;
  @TempDir Path dataDir;
  static Path lineLimitsConfPath;

  @BeforeAll
  public static void beforeAll() throws IOException {
    lineLimitsConfPath = tempDir.resolve("line-limits.toml");
    Files.copy(
        Objects.requireNonNull(
            TraceLineLimitTransactionSelectorTest.class.getResourceAsStream(
                MODULE_LINE_LIMITS_RESOURCE_NAME)),
        lineLimitsConfPath);
  }

  @BeforeEach
  void setUp() {
    setUpWithLivenessService(Optional.empty());
  }

  private void setUpWithLivenessService(Optional<LivenessService> livenessService) {
    LineaTracerConfiguration lineaTracerConfiguration =
        LineaTracerConfiguration.builder()
            .moduleLimitsFilePath(lineLimitsConfPath.toString())
            .moduleLimitsMap(
                new HashMap<>(
                    ModuleLineCountValidator.createLimitModules(lineLimitsConfPath.toString())))
            .isLimitless(false)
            .build();

    mockBlockchainService = mock(BlockchainService.class, RETURNS_DEEP_STUBS);
    when(mockBlockchainService.getChainId()).thenReturn(Optional.of(BigInteger.ONE));
    when(mockBlockchainService.getNextBlockBaseFee()).thenReturn(Optional.of(Wei.of(7)));
    when(mockBlockchainService.getChainHeadHeader().getTimestamp()).thenReturn(BLOCK_TIMESTAMP);
    when(mockBlockchainService.getNextBlockHardforkId(any(), anyLong()))
        .thenReturn(HardforkId.MainnetHardforkId.OSAKA);

    LineaTransactionSelectorConfiguration mockTxSelectorConfiguration =
        mock(LineaTransactionSelectorConfiguration.class);
    LineaL1L2BridgeSharedConfiguration l1L2BridgeConfiguration =
        new LineaL1L2BridgeSharedConfiguration(BRIDGE_CONTRACT, BRIDGE_LOG_TOPIC);
    LineaProfitabilityConfiguration mockProfitabilityConfiguration =
        mock(LineaProfitabilityConfiguration.class);
    BesuEvents mockEvents = mock(BesuEvents.class);
    bundlePool = spy(new LineaLimitedBundlePool(dataDir, 4096, mockEvents, mockBlockchainService));
    InvalidTransactionByLineCountCache invalidTransactionByLineCountCache =
        new InvalidTransactionByLineCountCache(10);
    final var transactionCompressor = new CachingTransactionCompressor();
    TransactionProfitabilityCalculator transactionProfitabilityCalculator =
        new TransactionProfitabilityCalculator(
            mockProfitabilityConfiguration, transactionCompressor);

    factory =
        new LineaTransactionSelectorFactory(
            mockBlockchainService,
            mockTxSelectorConfiguration,
            l1L2BridgeConfiguration,
            mockProfitabilityConfiguration,
            lineaTracerConfiguration,
            livenessService,
            Optional.empty(),
            Optional.empty(),
            bundlePool,
            invalidTransactionByLineCountCache,
            new AtomicReference<>(Collections.emptyMap()),
            new AtomicReference<>(Collections.emptyMap()),
            new AtomicReference<>(Collections.emptySet()),
            transactionProfitabilityCalculator);
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

    factory.selectPendingTransactions(mockBts, mockPendingBlockHeader, Collections.emptyList());

    verify(mockBts, atLeastOnce()).commit();
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

    factory.selectPendingTransactions(mockBts, mockPendingBlockHeader, Collections.emptyList());

    verify(mockBts).rollback();
  }

  @Test
  void testSelectPendingTransactions_WithoutBundles() {
    var mockBts = mock(BlockTransactionSelectionService.class);
    var mockPendingBlockHeader = mock(ProcessableBlockHeader.class);
    when(mockPendingBlockHeader.getNumber()).thenReturn(1L);

    factory.selectPendingTransactions(mockBts, mockPendingBlockHeader, Collections.emptyList());

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
        Optional.empty(),
        false);
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
