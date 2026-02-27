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

import java.math.BigInteger;
import java.util.List;
import java.util.Optional;
import linea.blob.BlobCompressorVersion;
import linea.blob.GoBackedBlobCompressor;
import net.consensys.linea.bl.TransactionProfitabilityCalculator;
import net.consensys.linea.config.LineaProfitabilityCliOptions;
import net.consensys.linea.config.LineaProfitabilityConfiguration;
import net.consensys.linea.utils.CachingTransactionCompressor;
import org.apache.tuweni.bytes.Bytes;
import org.bouncycastle.asn1.sec.SECNamedCurves;
import org.bouncycastle.asn1.x9.X9ECParameters;
import org.bouncycastle.crypto.params.ECDomainParameters;
import org.hyperledger.besu.crypto.SECPSignature;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.PendingTransaction;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.hyperledger.besu.ethereum.eth.transactions.PendingTransaction.Local;
import org.hyperledger.besu.ethereum.eth.transactions.PendingTransaction.Remote;
import org.hyperledger.besu.ethereum.mainnet.ValidationResult;
import org.hyperledger.besu.ethereum.processing.TransactionProcessingResult;
import org.hyperledger.besu.plugin.data.ProcessableBlockHeader;
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

  private static final Address SENDER =
      Address.fromHexString("0x0000000000000000000000000000000000001000");
  private static final Address RECIPIENT =
      Address.fromHexString("0x0000000000000000000000000000000000001001");

  private static final SECPSignature FAKE_SIGNATURE;

  static {
    final X9ECParameters params = SECNamedCurves.getByName("secp256k1");
    final ECDomainParameters curve =
        new ECDomainParameters(params.getCurve(), params.getG(), params.getN(), params.getH());
    FAKE_SIGNATURE =
        SECPSignature.create(
            new BigInteger(
                "66397251408932042429874251838229702988618145381408295790259650671563847073199"),
            new BigInteger(
                "24729624138373455972486746091821238755870276413282629437244319694880507882088"),
            (byte) 0,
            curve.getN());
  }

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
    final var transactionCompressor =
        new CachingTransactionCompressor(
            GoBackedBlobCompressor.getInstance(BlobCompressorVersion.V2, 128 * 1024));
    final var transactionProfitabilityCalculator =
        new TransactionProfitabilityCalculator(profitabilityConf, transactionCompressor);
    return new ProfitableTransactionSelector(
        blockchainService, profitabilityConf, Optional.empty(), transactionProfitabilityCalculator);
  }

  @Test
  public void shouldSelectWhenProfitable() {
    var createProcessingResult = createProcessingResult(21000);
    verifyTransactionSelection(
        transactionSelector,
        createEvaluationContext(false, 100, Wei.of(1_100_000_000), Wei.of(1_000_000_000), 21000),
        createProcessingResult,
        SELECTED,
        SELECTED);
  }

  @Test
  public void shouldNotSelectWhenUnprofitableUpfront() {
    var createProcessingResult = createProcessingResult(21000);
    verifyTransactionSelection(
        transactionSelector,
        createEvaluationContext(false, 10000, Wei.of(1_000_100), Wei.of(1_000_000), 21000),
        createProcessingResult,
        TX_UNPROFITABLE_UPFRONT,
        null);
  }

  @Test
  public void shouldNotSelectWhenUnprofitable() {
    var createProcessingResult = createProcessingResult(21000);
    verifyTransactionSelection(
        transactionSelector,
        createEvaluationContext(false, 10000, Wei.of(1_000_100), Wei.of(1_000_000), 210000),
        createProcessingResult,
        SELECTED,
        TX_UNPROFITABLE);
  }

  @Test
  public void shouldSelectPrevUnprofitableAfterGasPriceBump() {
    var createProcessingResult = createProcessingResult(21000);
    verifyTransactionSelection(
        transactionSelector,
        createEvaluationContext(
            false, 1000, Wei.of(1_100_000_000).multiply(9), Wei.of(1_000_000_000), 210000),
        createProcessingResult,
        SELECTED,
        SELECTED);
  }

  @Test
  public void shouldSelectPriorityTxEvenWhenUnprofitable() {
    var createProcessingResult = createProcessingResult(21000);
    verifyTransactionSelection(
        transactionSelector,
        createEvaluationContext(true, 1000, Wei.of(1_100_000_000), Wei.of(1_000_000_000), 21000),
        createProcessingResult,
        SELECTED,
        SELECTED);
  }

  @Test
  public void profitableAndUnprofitableTxsMix() {
    var minGasPriceBlock1 = Wei.of(1_000_000);
    var createProcessingResult1 = createProcessingResult(21000);
    var evaluationContext1 =
        createEvaluationContext(false, 10000, Wei.of(1_000_010), minGasPriceBlock1, 210000);
    // first try of first tx
    verifyTransactionSelection(
        transactionSelector,
        evaluationContext1,
        createProcessingResult1,
        SELECTED,
        TX_UNPROFITABLE);

    var createProcessingResult2 = createProcessingResult(21000);
    var evaluationContext2 =
        createEvaluationContext(false, 1000, Wei.of(1_000_010), minGasPriceBlock1, 210000);
    // first try of second tx
    verifyTransactionSelection(
        transactionSelector, evaluationContext2, createProcessingResult2, SELECTED, SELECTED);

    // simulate another block
    transactionSelector = newSelectorForNewBlock();
    // we keep the min gas price the same
    var minGasPriceBlock2 = minGasPriceBlock1;

    // second try of the first tx
    verifyTransactionSelection(
        transactionSelector,
        evaluationContext1.setMinGasPrice(minGasPriceBlock2),
        createProcessingResult1,
        SELECTED,
        TX_UNPROFITABLE);

    var createProcessingResult3 = createProcessingResult(21000);
    var evaluationContext3 =
        createEvaluationContext(false, 100, Wei.of(1_100_000_000), minGasPriceBlock1, 21000);

    // new profitable tx is selected
    verifyTransactionSelection(
        transactionSelector, evaluationContext3, createProcessingResult3, SELECTED, SELECTED);
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

  private TestTransactionEvaluationContext createEvaluationContext(
      final boolean hasPriority,
      final int payloadSize,
      final Wei effectiveGasPrice,
      final Wei minGasPrice,
      final long gasLimit) {
    final Transaction transaction =
        Transaction.builder()
            .sender(SENDER)
            .to(RECIPIENT)
            .gasLimit(gasLimit)
            .gasPrice(effectiveGasPrice)
            .payload(Bytes.random(payloadSize))
            .value(Wei.ONE)
            .signature(FAKE_SIGNATURE)
            .build();

    final PendingTransaction pendingTransaction =
        hasPriority ? new Local.Priority(transaction) : new Remote(transaction);

    return new TestTransactionEvaluationContext(
        mock(ProcessableBlockHeader.class), pendingTransaction, effectiveGasPrice, minGasPrice);
  }

  private TransactionProcessingResult createProcessingResult(long gasUsedByTransaction) {
    return TransactionProcessingResult.successful(
        List.of(),
        gasUsedByTransaction,
        0,
        Bytes.EMPTY,
        Optional.empty(),
        ValidationResult.valid());
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
