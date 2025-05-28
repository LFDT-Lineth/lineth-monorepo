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

import static net.consensys.linea.sequencer.txselection.LineaTransactionSelectionResult.TX_GAS_EXCEEDS_USER_MAX_BLOCK_GAS;
import static net.consensys.linea.sequencer.txselection.LineaTransactionSelectionResult.TX_TOO_LARGE_FOR_REMAINING_USER_GAS;
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
 * if the cumulative gas used by the block, including the current pending transaction, is below the
 * user configured max amount, if not the transaction is not selected. This means that the user can
 * configure a max gas per block that is below the limit defined by the protocol.
 */
@Slf4j
public class MaxBlockGasTransactionSelector
    extends AbstractStatefulPluginTransactionSelector<Long> {

  private final long maxGasPerBlock;

  public MaxBlockGasTransactionSelector(
      final SelectorsStateManager selectorsStateManager, final long maxGasPerBlock) {
    super(selectorsStateManager, 0L, SelectorsStateManager.StateDuplicator::duplicateLong);
    this.maxGasPerBlock = maxGasPerBlock;
  }

  @Override
  public TransactionSelectionResult evaluateTransactionPreProcessing(
      final TransactionEvaluationContext evaluationContext) {
    // Evaluation done in post-processing, no action needed here.
    return SELECTED;
  }

  /**
   * Evaluates a transaction post-processing. Checks if adding the gas used of the transaction, to
   * the cumulative gas used of the block till now, is below the configured max gas used per block
   * specified by the operator of the node.
   *
   * @param evaluationContext The current selection context.
   * @return TX_TOO_LARGE_FOR_REMAINING_USER_GAS if adding this transaction pushes the gas used by
   *     the block over the limit, TX_GAS_EXCEEDS_USER_MAX_BLOCK_GAS if the gas used by this
   *     transaction alone is greater than the max gas used per block limit, otherwise SELECTED.
   */
  @Override
  public TransactionSelectionResult evaluateTransactionPostProcessing(
      final TransactionEvaluationContext evaluationContext,
      final TransactionProcessingResult processingResult) {

    final Transaction transaction = evaluationContext.getPendingTransaction().getTransaction();
    final long gasUsedByTransaction = processingResult.getEstimateGasUsedByTransaction();

    if (gasUsedByTransaction > maxGasPerBlock) {
      log.atTrace()
          .setMessage(
              "Not selecting transaction {}, its gas used {} is greater than max user gas per block {},"
                  + " removing it from the txpool")
          .addArgument(transaction::getHash)
          .addArgument(gasUsedByTransaction)
          .addArgument(maxGasPerBlock)
          .log();
      return TX_GAS_EXCEEDS_USER_MAX_BLOCK_GAS;
    }

    final var stateCumulativeBlockGasUsed = getWorkingState();

    final long newCumulativeBlockGasUsed =
        Math.addExact(stateCumulativeBlockGasUsed, gasUsedByTransaction);

    if (newCumulativeBlockGasUsed > maxGasPerBlock) {
      log.atTrace()
          .setMessage(
              "Not selecting transaction {}, its cumulative block gas used {} exceeds max user gas per block {},"
                  + " skipping it")
          .addArgument(transaction::getHash)
          .addArgument(newCumulativeBlockGasUsed)
          .addArgument(maxGasPerBlock)
          .log();
      return TX_TOO_LARGE_FOR_REMAINING_USER_GAS;
    }

    setWorkingState(newCumulativeBlockGasUsed);

    return SELECTED;
  }
}
