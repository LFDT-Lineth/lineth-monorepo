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
package net.consensys.linea.sequencer.txselection.selectors;

import static net.consensys.linea.sequencer.txselection.LineaTransactionSelectionResult.BUNDLE_GAS_EXCEEDS_MAX_BUNDLE_BLOCK_GAS;
import static net.consensys.linea.sequencer.txselection.selectors.MaxBundleBlockGasTransactionSelectorTest.TestParams.concat;
import static net.consensys.linea.sequencer.txselection.selectors.MaxBundleBlockGasTransactionSelectorTest.TestParams.notSelected;
import static net.consensys.linea.sequencer.txselection.selectors.MaxBundleBlockGasTransactionSelectorTest.TestParams.selected;
import static org.assertj.core.api.Assertions.assertThat;
import static org.hyperledger.besu.plugin.data.TransactionSelectionResult.SELECTED;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;
import java.util.Optional;

import net.consensys.linea.bundles.TransactionBundle;
import net.consensys.linea.sequencer.txselection.selectors.MaxBundleGasPerBlockTransactionSelector.BundleGasTracker;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.hyperledger.besu.plugin.data.ProcessableBlockHeader;
import org.hyperledger.besu.plugin.data.TransactionProcessingResult;
import org.hyperledger.besu.plugin.data.TransactionSelectionResult;
import org.hyperledger.besu.plugin.services.txselection.PluginTransactionSelector;
import org.hyperledger.besu.plugin.services.txselection.SelectorsStateManager;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.FieldSource;

public class MaxBundleBlockGasTransactionSelectorTest {
  private static final int MAX_BUNDLE_GAS_PER_BLOCK = 1000;
  private static int seq = 1;
  private MaxBundleGasPerBlockTransactionSelector transactionSelector;
  private SelectorsStateManager selectorsStateManager;

  @BeforeEach
  public void initialize() {
    selectorsStateManager = new SelectorsStateManager();
    transactionSelector =
        new MaxBundleGasPerBlockTransactionSelector(
            selectorsStateManager, MAX_BUNDLE_GAS_PER_BLOCK);
    selectorsStateManager.blockSelectionStarted();
  }

  static final List<List<TestParams>> bundleGasLessThanEqual =
      List.of(
          selected(MAX_BUNDLE_GAS_PER_BLOCK - 1),
          selected(MAX_BUNDLE_GAS_PER_BLOCK),
          selected(MAX_BUNDLE_GAS_PER_BLOCK / 2, MAX_BUNDLE_GAS_PER_BLOCK / 2),
          selected(MAX_BUNDLE_GAS_PER_BLOCK - 100, 100));

  static final List<List<TestParams>> bundleGasGreaterThan =
      List.of(
          notSelected(
              MAX_BUNDLE_GAS_PER_BLOCK + 1,
              BUNDLE_GAS_EXCEEDS_MAX_BUNDLE_BLOCK_GAS,
              MAX_BUNDLE_GAS_PER_BLOCK + 1),
          concat(
              selected(MAX_BUNDLE_GAS_PER_BLOCK / 2, MAX_BUNDLE_GAS_PER_BLOCK / 2),
              notSelected(
                  MAX_BUNDLE_GAS_PER_BLOCK / 2,
                  BUNDLE_GAS_EXCEEDS_MAX_BUNDLE_BLOCK_GAS,
                  (MAX_BUNDLE_GAS_PER_BLOCK / 2) * 3)),
          concat(
              selected(MAX_BUNDLE_GAS_PER_BLOCK - 100),
              notSelected(
                  101,
                  BUNDLE_GAS_EXCEEDS_MAX_BUNDLE_BLOCK_GAS,
                  MAX_BUNDLE_GAS_PER_BLOCK - 100 + 101)));

  @ParameterizedTest
  @FieldSource("bundleGasLessThanEqual")
  @FieldSource("bundleGasGreaterThan")
  public void shouldSelectWhen_GasUsedByBundle_IsLessThanEqual_MaxBundleGasPerBlock(
      final List<TestParams> params) {

    final var mockTxs = params.stream().map(__ -> mockTransaction()).toList();
    final var bundle =
        new TransactionBundle(
            Hash.wrap(Bytes32.repeat((byte) seq++)),
            mockTxs,
            1L,
            Optional.empty(),
            Optional.empty(),
            Optional.empty(),
            Optional.empty());
    final var evaluationContexts =
        bundle.pendingTransactions().stream()
            .map(pt -> new TestTransactionEvaluationContext(mock(ProcessableBlockHeader.class), pt))
            .toList();

    for (int i = 0; i < params.size(); i++) {
      final var mockTransactionProcessingResult =
          mockTransactionProcessingResult(params.get(i).gasUsedByTransaction);
      verifyTransactionSelection(
          transactionSelector,
          evaluationContexts.get(i),
          mockTransactionProcessingResult,
          params.get(i).selectionResult);
      assertThat(
              (BundleGasTracker) selectorsStateManager.getSelectorWorkingState(transactionSelector))
          .isEqualTo(
              new BundleGasTracker(
                  params.get(i).expectedBlockBundleGasUsed, params.get(i).expectedBundleGasUsed));
    }
  }

  private void verifyTransactionSelection(
      final PluginTransactionSelector selector,
      final TestTransactionEvaluationContext evaluationContext,
      final TransactionProcessingResult processingResult,
      final TransactionSelectionResult expectedSelectionResult) {
    var selectionResult =
        selector.evaluateTransactionPostProcessing(evaluationContext, processingResult);
    assertThat(selectionResult).isEqualTo(expectedSelectionResult);
  }

  private Transaction mockTransaction() {
    Transaction transaction = mock(Transaction.class);
    when(transaction.getHash()).thenReturn(Hash.wrap(Bytes32.repeat((byte) seq++)));
    return transaction;
  }

  private TransactionProcessingResult mockTransactionProcessingResult(long gasUsedByTransaction) {
    TransactionProcessingResult mockTransactionProcessingResult =
        mock(TransactionProcessingResult.class);
    when(mockTransactionProcessingResult.getEstimateGasUsedByTransaction())
        .thenReturn(gasUsedByTransaction);
    return mockTransactionProcessingResult;
  }

  record TestParams(
      long gasUsedByTransaction,
      TransactionSelectionResult selectionResult,
      long expectedBlockBundleGasUsed,
      long expectedBundleGasUsed) {

    static List<TestParams> selected(final long... gasUsedByTransactions) {
      final List<TestParams> params = new ArrayList<>(gasUsedByTransactions.length);

      long cumulativeBundleGasUsed = 0;

      for (final long gasUsedByTransaction : gasUsedByTransactions) {
        cumulativeBundleGasUsed += gasUsedByTransaction;
        params.add(
            new TestParams(
                gasUsedByTransaction, SELECTED, cumulativeBundleGasUsed, cumulativeBundleGasUsed));
      }
      return params;
    }

    static List<TestParams> notSelected(
        final long gasUsedByTransaction,
        final TransactionSelectionResult selectionResult,
        final long expectedBundleGasUsed) {
      return List.of(
          new TestParams(
              gasUsedByTransaction, selectionResult, expectedBundleGasUsed, expectedBundleGasUsed));
    }

    @SafeVarargs
    static List<TestParams> concat(final List<TestParams>... lists) {
      return Arrays.stream(lists).flatMap(List::stream).toList();
    }
  }
}
