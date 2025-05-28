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
import static net.consensys.linea.sequencer.txselection.LineaTransactionSelectionResult.BUNDLE_TOO_LARGE_FOR_REMAINING_BUNDLE_BLOCK_GAS;
import static org.hyperledger.besu.plugin.data.TransactionSelectionResult.SELECTED;

import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.bundles.TransactionBundle;
import org.hyperledger.besu.plugin.data.TransactionProcessingResult;
import org.hyperledger.besu.plugin.data.TransactionSelectionResult;
import org.hyperledger.besu.plugin.services.txselection.AbstractStatefulPluginTransactionSelector;
import org.hyperledger.besu.plugin.services.txselection.SelectorsStateManager;
import org.hyperledger.besu.plugin.services.txselection.TransactionEvaluationContext;

@Slf4j
public class MaxBundleGasPerBlockTransactionSelector
    extends AbstractStatefulPluginTransactionSelector<
        MaxBundleGasPerBlockTransactionSelector.BundleGasTracker> {
  private final long maxBundleGasPerBlock;

  public MaxBundleGasPerBlockTransactionSelector(
      final SelectorsStateManager selectorsStateManager, final long maxBundleGasPerBlock) {
    super(selectorsStateManager, new BundleGasTracker(0L, 0L), BundleGasTracker::duplicate);
    this.maxBundleGasPerBlock = maxBundleGasPerBlock;
  }

  @Override
  public TransactionSelectionResult evaluateTransactionPreProcessing(
      final TransactionEvaluationContext txContext) {
    return SELECTED;
  }

  @Override
  public TransactionSelectionResult evaluateTransactionPostProcessing(
      final TransactionEvaluationContext txContext,
      final TransactionProcessingResult transactionProcessingResult) {

    // short circuit if we are not a PendingBundleTx
    if (!(txContext.getPendingTransaction()
        instanceof TransactionBundle.PendingBundleTx pendingBundleTx)) {
      return SELECTED;
    }

    final long gasUsedByTransaction = transactionProcessingResult.getEstimateGasUsedByTransaction();

    final long currentBundleGasUsed =
        pendingBundleTx.isBundleStart() ? 0L : getWorkingState().currentBundleGasUsed();
    final long newCurrentBundleGasUsed = currentBundleGasUsed + gasUsedByTransaction;

    final long cumulativeBlockBundleGasUsed = getWorkingState().cumulativeBlockBundleGasUsed();
    final long newCumulativeBlockBundleGasUsed =
        cumulativeBlockBundleGasUsed + gasUsedByTransaction;

    setWorkingState(new BundleGasTracker(newCumulativeBlockBundleGasUsed, newCurrentBundleGasUsed));

    if (newCurrentBundleGasUsed > maxBundleGasPerBlock) {
      log.atTrace()
          .setMessage(
              "Not selecting bundle transaction {} since the current gas used by the bundle is greater than the max {};"
                  + " gas used by tx {} + gas already used by the bundle {} = {}")
          .addArgument(pendingBundleTx::toTraceLog)
          .addArgument(maxBundleGasPerBlock)
          .addArgument(gasUsedByTransaction)
          .addArgument(currentBundleGasUsed)
          .addArgument(newCurrentBundleGasUsed)
          .log();
      return BUNDLE_GAS_EXCEEDS_MAX_BUNDLE_BLOCK_GAS;
    }

    if (newCumulativeBlockBundleGasUsed > maxBundleGasPerBlock) {
      log.atTrace()
          .setMessage(
              "Not selecting bundle transaction {} since the cumulative gas used by bundles in this block is greater than the max {};"
                  + " gas used by tx {} + gas already used by the bundle {} = {}")
          .addArgument(pendingBundleTx::toTraceLog)
          .addArgument(maxBundleGasPerBlock)
          .addArgument(gasUsedByTransaction)
          .addArgument(cumulativeBlockBundleGasUsed)
          .addArgument(newCumulativeBlockBundleGasUsed)
          .log();
      return BUNDLE_TOO_LARGE_FOR_REMAINING_BUNDLE_BLOCK_GAS;
    }
    return SELECTED;
  }

  /**
   * Track the gas used by transactions in bundles
   *
   * @param cumulativeBlockBundleGasUsed the gas used by selected bundle transactions since the
   *     beginning of the block
   * @param currentBundleGasUsed the gas used only by transactions belonging to the current bundle
   */
  public record BundleGasTracker(long cumulativeBlockBundleGasUsed, long currentBundleGasUsed) {

    static BundleGasTracker duplicate(final BundleGasTracker bundleGasTracker) {
      // since the record is immutable there is no need to create another instance, and we can just
      // return the same
      return bundleGasTracker;
    }
  }
}
