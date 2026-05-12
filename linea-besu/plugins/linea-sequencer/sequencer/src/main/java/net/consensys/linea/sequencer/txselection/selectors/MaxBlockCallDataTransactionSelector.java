/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.sequencer.txselection.selectors;

import static net.consensys.linea.sequencer.txselection.LineaTransactionSelectionResult.BLOCK_CALLDATA_OVERFLOW;
import static org.hyperledger.besu.plugin.data.TransactionSelectionResult.SELECTED;

import lombok.extern.slf4j.Slf4j;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.plugin.data.TransactionProcessingResult;
import org.hyperledger.besu.plugin.data.TransactionSelectionResult;
import org.hyperledger.besu.plugin.services.txselection.AbstractStatefulPluginTransactionSelector;
import org.hyperledger.besu.plugin.services.txselection.SelectorsStateManager;
import org.hyperledger.besu.plugin.services.txselection.TransactionEvaluationContext;

/**
 * This class implements TransactionSelector and provides a specific implementation for evaluating
 * transactions based on the size of the call data. It checks if adding a transaction to the block
 * pushes the call data size of the block over the limit.
 */
@Slf4j
public class MaxBlockCallDataTransactionSelector
    extends AbstractStatefulPluginTransactionSelector<Long> {

  private final int maxBlockCallDataSize;

  public MaxBlockCallDataTransactionSelector(
      final SelectorsStateManager stateManager, final int maxBlockCallDataSize) {
    super(stateManager, 0L, SelectorsStateManager.StateDuplicator::duplicateLong);
    this.maxBlockCallDataSize = maxBlockCallDataSize;
  }

  /**
   * Evaluates a transaction before processing. Checks if adding the transaction to the block pushes
   * the call data size of the block over the limit.
   *
   * @param evaluationContext The current selection context.
   * @return BLOCK_CALLDATA_FULL if the call data size of a transactions pushes the size for the
   *     block over the limit, otherwise SELECTED.
   */
  @Override
  public TransactionSelectionResult evaluateTransactionPreProcessing(
      final TransactionEvaluationContext evaluationContext) {

    final Transaction transaction = evaluationContext.getPendingTransaction().getTransaction();
    final int transactionCallDataSize = transaction.getPayload().size();

    final var stateCumulativeBlockCallDataSize = getWorkingState();

    final long newCumulativeBlockCallDataSize =
        Math.addExact(stateCumulativeBlockCallDataSize, transactionCallDataSize);

    if (newCumulativeBlockCallDataSize > maxBlockCallDataSize) {
      log.atTrace()
          .setMessage(
              "Cumulative block calldata size including tx {} is {} greater than the max allowed {}, skipping tx")
          .addArgument(transaction::getHash)
          .addArgument(newCumulativeBlockCallDataSize)
          .addArgument(maxBlockCallDataSize)
          .log();
      return BLOCK_CALLDATA_OVERFLOW;
    }

    log.atTrace()
        .setMessage("Cumulative block calldata size including tx {} is {}")
        .addArgument(transaction::getHash)
        .addArgument(newCumulativeBlockCallDataSize)
        .log();

    return SELECTED;
  }

  /**
   * No evaluation is performed post-processing.
   *
   * @param evaluationContext The current selection context.
   * @param processingResult The result of the transaction processing.
   * @return Always returns SELECTED.
   */
  @Override
  public TransactionSelectionResult evaluateTransactionPostProcessing(
      final TransactionEvaluationContext evaluationContext,
      final TransactionProcessingResult processingResult) {
    final long newCumulativeBlockCallDataSize =
        Math.addExact(
            getWorkingState(),
            evaluationContext.getPendingTransaction().getTransaction().getPayload().size());

    setWorkingState(newCumulativeBlockCallDataSize);

    return SELECTED;
  }
}
