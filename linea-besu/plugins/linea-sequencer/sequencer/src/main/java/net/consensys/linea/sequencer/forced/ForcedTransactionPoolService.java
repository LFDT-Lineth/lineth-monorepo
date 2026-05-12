/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.sequencer.forced;

import java.util.List;
import java.util.Optional;
import org.hyperledger.besu.plugin.services.BesuService;
import org.hyperledger.besu.plugin.services.txselection.BlockTransactionSelectionService;

/**
 * Service for managing forced transactions. Forced transactions are submitted via
 * linea_sendForcedRawTransaction and have highest priority in block building.
 */
public interface ForcedTransactionPoolService extends BesuService {

  /**
   * Add forced transactions to the pool. Transactions are processed in the order they are added.
   *
   * @param transactions The forced transactions to add
   */
  void addForcedTransactions(List<ForcedTransaction> transactions);

  /**
   * Process forced transactions for block building. This method should be called at the start of
   * transaction selection, before liveness bundles and regular bundles.
   *
   * <p>Processing stops on the first failure because only one invalidity proof can be generated per
   * block.
   *
   * @param blockNumber The pending block number
   * @param blockTransactionSelectionService The block transaction selection service
   */
  void processForBlock(
      long blockNumber, BlockTransactionSelectionService blockTransactionSelectionService);

  /**
   * Get the inclusion status of a forced transaction.
   *
   * @param forcedTransactionNumber The unique identifier of the forced transaction
   * @return The status if the transaction has reached a final outcome, empty otherwise
   */
  Optional<ForcedTransactionStatus> getInclusionStatus(long forcedTransactionNumber);

  /**
   * Get the number of pending forced transactions.
   *
   * @return The number of pending transactions
   */
  int pendingCount();
}
