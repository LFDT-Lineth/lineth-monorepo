/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.sequencer.txselection.selectors;

import static net.consensys.linea.sequencer.txselection.LineaTransactionSelectionResult.TX_UNPROFITABLE;
import static net.consensys.linea.sequencer.txselection.LineaTransactionSelectionResult.TX_UNPROFITABLE_UPFRONT;
import static org.assertj.core.api.Assertions.assertThat;
import static org.hyperledger.besu.plugin.data.TransactionSelectionResult.SELECTED;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

import java.util.Optional;
import net.consensys.linea.bl.TransactionProfitabilityCalculator;
import net.consensys.linea.config.LineaProfitabilityCliOptions;
import net.consensys.linea.config.LineaProfitabilityConfiguration;
import net.consensys.linea.utils.CachingTransactionCompressor;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.bouncycastle.crypto.digests.KeccakDigest;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.datatypes.PendingTransaction;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.plugin.data.ProcessableBlockHeader;
import org.hyperledger.besu.plugin.data.TransactionProcessingResult;
import org.hyperledger.besu.plugin.data.TransactionSelectionResult;
import org.hyperledger.besu.plugin.services.BlockchainService;
import org.hyperledger.besu.plugin.services.txselection.PluginTransactionSelector;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

public class ProfitableTransactionSelectorTest {
  private static final int FIXED_GAS_COST_WEI = 600_000;
  private static final int VARIABLE_GAS_COST_WEI = 1_000_000;
  private static final double MIN_MARGIN = 1.5;
  private static final Wei BASE_FEE = Wei.of(7);
  private final LineaProfitabilityConfiguration profitabilityConf =
      LineaProfitabilityCliOptions.create().toDomainObject().toBuilder()
          .minMargin(MIN_MARGIN)
          .fixedCostWei(FIXED_GAS_COST_WEI)
          .variableCostWei(VARIABLE_GAS_COST_WEI)
          .build();
  private ProfitableTransactionSelector transactionSelector;

  @BeforeEach
  public void initialize() {
    transactionSelector = newSelectorForNewBlock();
  }

  private ProfitableTransactionSelector newSelectorForNewBlock() {
    final var blockchainService = mock(BlockchainService.class);
    when(blockchainService.getNextBlockBaseFee()).thenReturn(Optional.of(BASE_FEE));
    final var transactionCompressor = new CachingTransactionCompressor();
    final var transactionProfitabilityCalculator =
        new TransactionProfitabilityCalculator(profitabilityConf, transactionCompressor);
    return new ProfitableTransactionSelector(
        blockchainService, profitabilityConf, Optional.empty(), transactionProfitabilityCalculator);
  }

  @Test
  public void shouldSelectWhenProfitable() {
    var mockTransactionProcessingResult = mockTransactionProcessingResult(21000);
    verifyTransactionSelection(
        transactionSelector,
        mockEvaluationContext(false, 100, Wei.of(1_100_000_000), Wei.of(1_000_000_000), 21000),
        mockTransactionProcessingResult,
        SELECTED,
        SELECTED);
  }

  @Test
  public void shouldNotSelectWhenUnprofitableUpfront() {
    var mockTransactionProcessingResult = mockTransactionProcessingResult(21000);
    verifyTransactionSelection(
        transactionSelector,
        mockEvaluationContext(false, 10000, Wei.of(1_000_100), Wei.of(1_000_000), 21000),
        mockTransactionProcessingResult,
        TX_UNPROFITABLE_UPFRONT,
        null);
  }

  @Test
  public void shouldNotSelectWhenUnprofitable() {
    var mockTransactionProcessingResult = mockTransactionProcessingResult(21000);
    verifyTransactionSelection(
        transactionSelector,
        mockEvaluationContext(false, 10000, Wei.of(1_000_100), Wei.of(1_000_000), 210000),
        mockTransactionProcessingResult,
        SELECTED,
        TX_UNPROFITABLE);
  }

  @Test
  public void shouldSelectPrevUnprofitableAfterGasPriceBump() {
    var mockTransactionProcessingResult = mockTransactionProcessingResult(21000);
    verifyTransactionSelection(
        transactionSelector,
        mockEvaluationContext(
            false, 1000, Wei.of(1_100_000_000).multiply(9), Wei.of(1_000_000_000), 210000),
        mockTransactionProcessingResult,
        SELECTED,
        SELECTED);
  }

  @Test
  public void shouldSelectPriorityTxEvenWhenUnprofitable() {
    var mockTransactionProcessingResult = mockTransactionProcessingResult(21000);
    verifyTransactionSelection(
        transactionSelector,
        mockEvaluationContext(true, 1000, Wei.of(1_100_000_000), Wei.of(1_000_000_000), 21000),
        mockTransactionProcessingResult,
        SELECTED,
        SELECTED);
  }

  @Test
  public void profitableAndUnprofitableTxsMix() {
    var minGasPriceBlock1 = Wei.of(1_000_000);
    var mockTransactionProcessingResult1 = mockTransactionProcessingResult(21000);
    var mockEvaluationContext1 =
        mockEvaluationContext(false, 10000, Wei.of(1_000_010), minGasPriceBlock1, 210000);
    // first try of first tx
    verifyTransactionSelection(
        transactionSelector,
        mockEvaluationContext1,
        mockTransactionProcessingResult1,
        SELECTED,
        TX_UNPROFITABLE);

    var mockTransactionProcessingResult2 = mockTransactionProcessingResult(21000);
    var mockEvaluationContext2 =
        mockEvaluationContext(false, 1000, Wei.of(1_000_010), minGasPriceBlock1, 210000);
    // first try of second tx
    verifyTransactionSelection(
        transactionSelector,
        mockEvaluationContext2,
        mockTransactionProcessingResult2,
        SELECTED,
        SELECTED);

    // simulate another block
    transactionSelector = newSelectorForNewBlock();
    // we keep the min gas price the same
    var minGasPriceBlock2 = minGasPriceBlock1;

    // second try of the first tx
    verifyTransactionSelection(
        transactionSelector,
        mockEvaluationContext1.setMinGasPrice(minGasPriceBlock2),
        mockTransactionProcessingResult1,
        SELECTED,
        TX_UNPROFITABLE);

    var mockTransactionProcessingResult3 = mockTransactionProcessingResult(21000);
    var mockEvaluationContext3 =
        mockEvaluationContext(false, 100, Wei.of(1_100_000_000), minGasPriceBlock1, 21000);

    // new profitable tx is selected
    verifyTransactionSelection(
        transactionSelector,
        mockEvaluationContext3,
        mockTransactionProcessingResult3,
        SELECTED,
        SELECTED);
  }

  private void verifyTransactionSelection(
      final ProfitableTransactionSelector selector,
      final TestTransactionEvaluationContext evaluationContext,
      final TransactionProcessingResult processingResult,
      final TransactionSelectionResult expectedPreProcessingResult,
      final TransactionSelectionResult expectedPostProcessingResult) {
    var preProcessingResult = selector.evaluateTransactionPreProcessing(evaluationContext);
    assertThat(preProcessingResult).isEqualTo(expectedPreProcessingResult);
    if (preProcessingResult.equals(SELECTED)) {
      var postProcessingResult =
          selector.evaluateTransactionPostProcessing(evaluationContext, processingResult);
      assertThat(postProcessingResult).isEqualTo(expectedPostProcessingResult);
      notifySelector(selector, evaluationContext, processingResult, postProcessingResult);
    } else {
      notifySelector(selector, evaluationContext, processingResult, preProcessingResult);
    }
  }

  private TestTransactionEvaluationContext mockEvaluationContext(
      final boolean hasPriority,
      final int size,
      final Wei effectiveGasPrice,
      final Wei minGasPrice,
      final long gasLimit) {
    PendingTransaction pendingTransaction = mock(PendingTransaction.class);
    Transaction transaction = mock(Transaction.class);
    when(transaction.getHash()).thenReturn(Hash.wrap(Bytes32.random()));
    when(transaction.getGasLimit()).thenReturn(gasLimit);
    when(transaction.encoded()).thenReturn(Bytes.wrap(pseudoRandomBytes(size)));
    when(pendingTransaction.getTransaction()).thenReturn(transaction);
    when(pendingTransaction.hasPriority()).thenReturn(hasPriority);
    return new TestTransactionEvaluationContext(
        mock(ProcessableBlockHeader.class), pendingTransaction, effectiveGasPrice, minGasPrice);
  }

  private byte[] pseudoRandomBytes(int size) {
    final int expectedCompressedSize =
        (size - 58) / 5; // This emulates old behaviour of compression ratio and size adjustment
    byte[] bytes = new byte[expectedCompressedSize];
    final KeccakDigest keccakDigest = new KeccakDigest(256);

    final byte[] out = new byte[32];
    int offset = 0;
    int i = 0;
    do {
      keccakDigest.update(new byte[] {(byte) i++}, 0, 1);
      keccakDigest.doFinal(out, 0);
      System.arraycopy(out, 0, bytes, offset, Math.min(expectedCompressedSize - offset, 32));
      offset += 32;
    } while (offset < expectedCompressedSize);

    return bytes;
  }

  private TransactionProcessingResult mockTransactionProcessingResult(long gasUsedByTransaction) {
    TransactionProcessingResult mockTransactionProcessingResult =
        mock(TransactionProcessingResult.class);
    when(mockTransactionProcessingResult.getEstimateGasUsedByTransaction())
        .thenReturn(gasUsedByTransaction);
    return mockTransactionProcessingResult;
  }

  private void notifySelector(
      final PluginTransactionSelector selector,
      final TestTransactionEvaluationContext evaluationContext,
      final TransactionProcessingResult processingResult,
      final TransactionSelectionResult selectionResult) {
    if (selectionResult.equals(SELECTED)) {
      selector.onTransactionSelected(evaluationContext, processingResult);
    } else {
      selector.onTransactionNotSelected(evaluationContext, selectionResult);
    }
  }
}
