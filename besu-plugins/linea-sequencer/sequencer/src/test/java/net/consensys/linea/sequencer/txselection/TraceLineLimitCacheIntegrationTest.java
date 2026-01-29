/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.sequencer.txselection;

import static net.consensys.linea.sequencer.txselection.LineaTransactionSelectionResult.txModuleLineCountOverflowCached;
import static org.assertj.core.api.Assertions.assertThat;
import static org.hyperledger.besu.plugin.data.TransactionSelectionResult.SELECTED;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

import java.io.IOException;
import java.math.BigInteger;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.HashMap;
import java.util.Optional;
import net.consensys.linea.config.LineaTracerConfiguration;
import net.consensys.linea.plugins.config.LineaL1L2BridgeSharedConfiguration;
import net.consensys.linea.sequencer.modulelimit.ModuleLineCountValidator;
import net.consensys.linea.sequencer.txpoolvalidation.validators.TraceLineLimitValidator;
import net.consensys.linea.sequencer.txselection.selectors.TestTransactionEvaluationContext;
import net.consensys.linea.sequencer.txselection.selectors.TraceLineLimitTransactionSelector;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.HardforkId;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.datatypes.PendingTransaction;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.plugin.data.ProcessableBlockHeader;
import org.hyperledger.besu.plugin.data.TransactionProcessingResult;
import org.hyperledger.besu.plugin.services.BlockchainService;
import org.hyperledger.besu.plugin.services.txselection.SelectorsStateManager;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

/**
 * Integration test that verifies the complete flow of the trace line count cache: 1. Transaction
 * selection marks a transaction as over-the-line-count limit 2. The transaction is added to the
 * shared cache 3. Transaction pool validation rejects the same transaction based on the cache
 */
public class TraceLineLimitCacheIntegrationTest {
  private static final int CACHE_SIZE = 5;
  private static final String MODULE_LINE_LIMITS_RESOURCE_NAME = "/sequencer/line-limits.toml";

  @TempDir static Path tempDir;
  static Path lineLimitsConfPath;

  private InvalidTransactionByLineCountCache sharedCache;
  private TraceLineLimitTransactionSelector transactionSelector;
  private TraceLineLimitValidator transactionPoolValidator;
  private LineaTracerConfiguration tracerConfiguration;
  private BlockchainService blockchainService;

  @BeforeAll
  public static void beforeAll() throws IOException {
    lineLimitsConfPath = tempDir.resolve("line-limits.toml");
    Files.copy(
        TraceLineLimitCacheIntegrationTest.class.getResourceAsStream(
            MODULE_LINE_LIMITS_RESOURCE_NAME),
        lineLimitsConfPath);
  }

  @BeforeEach
  void setUp() {
    blockchainService = mock(BlockchainService.class);
    when(blockchainService.getChainId()).thenReturn(Optional.of(BigInteger.ONE));
    when(blockchainService.getNextBlockHardforkId(any(), anyLong()))
        .thenReturn(HardforkId.MainnetHardforkId.OSAKA);
    // Create shared cache that both components will use
    sharedCache = new InvalidTransactionByLineCountCache(CACHE_SIZE);

    // Set up tracer configuration with low limits to trigger overflow
    tracerConfiguration =
        LineaTracerConfiguration.builder()
            .moduleLimitsFilePath(lineLimitsConfPath.toString())
            .moduleLimitsMap(
                new HashMap<>(
                    ModuleLineCountValidator.createLimitModules(lineLimitsConfPath.toString())))
            .build();

    // Set a low enough limit to exceed line counts even with a mock tx
    tracerConfiguration.moduleLimitsMap().put("EXT", 5);

    // Create transaction selector
    SelectorsStateManager stateManager = new SelectorsStateManager();
    transactionSelector =
        new TraceLineLimitTransactionSelector(
            stateManager,
            blockchainService,
            LineaL1L2BridgeSharedConfiguration.builder()
                .contract(Address.fromHexString("0xdeadbeef"))
                .topic(Bytes32.fromHexString("0xc0ffee"))
                .build(),
            tracerConfiguration,
            sharedCache);

    stateManager.blockSelectionStarted();

    // Create transaction pool validator using the same shared cache
    transactionPoolValidator = new TraceLineLimitValidator(sharedCache);
  }

  @Test
  void transactionRejectedBySelectionShouldBeRejectedByPool() {
    // Given: A transaction that we expect to be rejected by transaction selection
    Transaction mockTransaction = createMockTransaction();
    Hash transactionHash = mockTransaction.getHash();

    // Verify cache is initially empty
    assertThat(sharedCache.contains(transactionHash)).isFalse();
    assertThat(transactionPoolValidator.validateTransaction(mockTransaction, true, false))
        .isEmpty(); // Should accept initially

    // When: Transaction selection processes the transaction
    TestTransactionEvaluationContext evaluationContext = createEvaluationContext(mockTransaction);

    // Transaction should pass pre-processing (not in cache yet)
    var preProcessingResult =
        transactionSelector.evaluateTransactionPreProcessing(evaluationContext);
    assertThat(preProcessingResult).isEqualTo(SELECTED);

    // Transaction should fail post-processing due to line count limit
    var postProcessingResult =
        transactionSelector.evaluateTransactionPostProcessing(
            evaluationContext, mock(TransactionProcessingResult.class));
    assertThat(postProcessingResult.toString()).startsWith("TX_MODULE_LINE_COUNT_OVERFLOW");

    // Notify selector of the rejection (this should add to cache)
    transactionSelector.onTransactionNotSelected(evaluationContext, postProcessingResult);

    // Then: Transaction should now be in the shared cache
    assertThat(sharedCache.contains(transactionHash)).isTrue();

    // And: Transaction pool validator should now reject the same transaction
    Optional<String> poolValidationResult =
        transactionPoolValidator.validateTransaction(mockTransaction, true, false);
    assertThat(poolValidationResult).isPresent();
    assertThat(poolValidationResult.get())
        .contains("was already identified to go over line count limit");
    assertThat(poolValidationResult.get()).contains(transactionHash.toString());

    // Additional verification: Try different transaction parameters to ensure rejection is
    // consistent
    Optional<String> poolValidationResultLocal =
        transactionPoolValidator.validateTransaction(mockTransaction, true, false);
    Optional<String> poolValidationResultRemote =
        transactionPoolValidator.validateTransaction(mockTransaction, false, false);
    Optional<String> poolValidationResultPriority =
        transactionPoolValidator.validateTransaction(mockTransaction, false, true);

    assertThat(poolValidationResultLocal).isPresent();
    assertThat(poolValidationResultRemote).isPresent();
    assertThat(poolValidationResultPriority).isPresent();
  }

  @Test
  void transactionPoolStatusChangesAfterSelectionProcessing() {
    // Given: A transaction that we expect to be rejected by transaction selection
    Transaction mockTransaction = createMockTransaction();
    Hash transactionHash = mockTransaction.getHash();

    // Initially: Transaction should be accepted by pool validator (not in cache)
    assertThat(sharedCache.contains(transactionHash)).isFalse();
    Optional<String> initialPoolValidation =
        transactionPoolValidator.validateTransaction(mockTransaction, true, false);
    assertThat(initialPoolValidation)
        .as("Transaction should initially be accepted by pool before selection processing")
        .isEmpty();

    // When: Selection processing determines transaction exceeds line count
    TestTransactionEvaluationContext evaluationContext = createEvaluationContext(mockTransaction);
    var postProcessingResult =
        transactionSelector.evaluateTransactionPostProcessing(
            evaluationContext, mock(TransactionProcessingResult.class));
    assertThat(postProcessingResult.toString()).startsWith("TX_MODULE_LINE_COUNT_OVERFLOW");

    // And: Selector is notified of rejection (adds to cache)
    transactionSelector.onTransactionNotSelected(evaluationContext, postProcessingResult);

    // Then: Same transaction should now be rejected by pool validator
    Optional<String> finalPoolValidation =
        transactionPoolValidator.validateTransaction(mockTransaction, true, false);
    assertThat(finalPoolValidation)
        .as("Transaction should be rejected by pool after selection processing marks it as invalid")
        .isPresent();

    // Verify the reason for rejection
    assertThat(finalPoolValidation.get())
        .contains("was already identified to go over line count limit")
        .contains(transactionHash.toString());
  }

  @Test
  void cachedTransactionShouldBeRejectedInPreProcessing() {
    // Given: A transaction that was previously marked as over-the-line-count
    Transaction mockTransaction = createMockTransaction();
    Hash transactionHash = mockTransaction.getHash();
    String overflowingModule = "EXT";

    // Add transaction to cache (simulating previous rejection)
    sharedCache.remember(transactionHash, overflowingModule);
    assertThat(sharedCache.contains(transactionHash)).isTrue();

    // When: Transaction pool validator checks the transaction
    Optional<String> poolValidationResult =
        transactionPoolValidator.validateTransaction(mockTransaction, true, false);

    // Then: Should be rejected by pool validator
    assertThat(poolValidationResult).isPresent();

    // When: Transaction selection processes the same transaction again
    TestTransactionEvaluationContext evaluationContext = createEvaluationContext(mockTransaction);
    var preProcessingResult =
        transactionSelector.evaluateTransactionPreProcessing(evaluationContext);

    // Then: Should be rejected in pre-processing (cached) with the module name
    assertThat(preProcessingResult).isEqualTo(txModuleLineCountOverflowCached(overflowingModule));
  }

  @Test
  void cacheEvictionShouldAllowPreviouslyRejectedTransactions() {
    // Given: A cache that will be filled beyond capacity
    Transaction originalTransaction = createMockTransaction();
    Hash originalHash = originalTransaction.getHash();

    // Add original transaction to cache
    sharedCache.remember(originalHash, "EXT");
    assertThat(sharedCache.contains(originalHash)).isTrue();

    // Fill cache beyond capacity to evict original transaction
    for (int i = 0; i < CACHE_SIZE; i++) {
      sharedCache.remember(Hash.wrap(Bytes32.random()), "RAM");
    }

    // Then: Original transaction should no longer be in cache (evicted)
    assertThat(sharedCache.contains(originalHash)).isFalse();

    // And: Transaction pool validator should accept the transaction
    Optional<String> poolValidationResult =
        transactionPoolValidator.validateTransaction(originalTransaction, true, false);
    assertThat(poolValidationResult).isEmpty();

    // And: Transaction selector should process normally (not cached)
    TestTransactionEvaluationContext evaluationContext =
        createEvaluationContext(originalTransaction);
    var preProcessingResult =
        transactionSelector.evaluateTransactionPreProcessing(evaluationContext);
    assertThat(preProcessingResult).isEqualTo(SELECTED);
  }

  private Transaction createMockTransaction() {
    Transaction mockTransaction = mock(Transaction.class);
    Hash transactionHash = Hash.wrap(Bytes32.random());
    when(mockTransaction.getHash()).thenReturn(transactionHash);
    when(mockTransaction.getSizeForBlockInclusion()).thenReturn(100);
    when(mockTransaction.getGasLimit()).thenReturn(21000L);
    when(mockTransaction.getPayload()).thenReturn(Bytes.repeat((byte) 1, 0));
    return mockTransaction;
  }

  private TestTransactionEvaluationContext createEvaluationContext(Transaction transaction) {
    PendingTransaction pendingTransaction = mock(PendingTransaction.class);
    when(pendingTransaction.getTransaction()).thenReturn(transaction);
    when(pendingTransaction.hasPriority()).thenReturn(false);
    return new TestTransactionEvaluationContext(
        mock(ProcessableBlockHeader.class),
        pendingTransaction,
        Wei.of(1_100_000_000),
        Wei.of(1_000_000_000));
  }
}
