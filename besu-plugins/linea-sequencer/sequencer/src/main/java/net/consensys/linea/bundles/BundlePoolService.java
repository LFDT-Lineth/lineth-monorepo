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
package net.consensys.linea.bundles;

import java.util.List;
import java.util.UUID;

import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.plugin.services.BesuService;

public interface BundlePoolService extends BesuService {
  @FunctionalInterface
  interface TransactionBundleAddedListener {
    void onTransactionBundleAdded(TransactionBundle transactionBundle);
  }

  @FunctionalInterface
  interface TransactionBundleRemovedListener {
    void onTransactionBundleRemoved(TransactionBundle transactionBundle);
  }

  /**
   * Retrieves a list of TransactionBundles associated with a block number.
   *
   * @param blockNumber The block number to look up.
   * @return A list of TransactionBundles for the given block number, or an empty list if none are
   *     found.
   */
  List<TransactionBundle> getBundlesByBlockNumber(long blockNumber);

  /**
   * Retrieves a TransactionBundle by its unique hash identifier.
   *
   * @param hash The hash identifier of the TransactionBundle.
   * @return The TransactionBundle associated with the hash, or null if not found.
   */
  TransactionBundle get(Hash hash);

  /**
   * Retrieves a TransactionBundle by its replacement UUID
   *
   * @param replacementUUID identifier of the TransactionBundle.
   * @return The TransactionBundle associated with the uuid, or null if not found.
   */
  TransactionBundle get(UUID replacementUUID);

  /**
   * Puts or replaces an existing TransactionBundle in the cache and updates the block index.
   *
   * @param hash The hash identifier of the TransactionBundle.
   * @param bundle The new TransactionBundle to replace the existing one.
   * @throws IllegalStateException if the pool is frozen
   */
  void putOrReplace(Hash hash, TransactionBundle bundle);

  /**
   * Puts or replaces an existing TransactionBundle by UUIDin the cache and updates the block index.
   *
   * @param replacementUUID identifier of the TransactionBundle.
   * @param bundle The new TransactionBundle to replace the existing one.
   * @throws IllegalStateException if the pool is frozen
   */
  void putOrReplace(UUID replacementUUID, TransactionBundle bundle);

  /**
   * Removes an existing TransactionBundle in the cache and updates the block index.
   *
   * @param replacementUUID identifier of the TransactionBundle.
   * @return boolean indicating if bundle was found and removed
   * @throws IllegalStateException if the pool is frozen
   */
  boolean remove(UUID replacementUUID);

  /**
   * Removes an existing TransactionBundle in the cache and updates the block index.
   *
   * @param hash The hash identifier of the TransactionBundle.
   * @return boolean indicating if bundle was found and removed
   * @throws IllegalStateException if the pool is frozen
   */
  boolean remove(Hash hash);

  /**
   * Get the number of bundles in the pool
   *
   * @return the number of bundles in the pool
   */
  long size();

  /**
   * Return true if the pool does not accept modifications anymore
   *
   * @return true if the pool does not accept modifications anymore
   */
  boolean isFrozen();

  /**
   * Save the content of the pool to disk. Note that once this operation starts, the pool will be
   * frozen and will not be possible to modify it anymore.
   */
  void saveToDisk();

  /**
   * Load the content of the pool from disk.
   *
   * @throws IllegalStateException if the pool is frozen
   */
  void loadFromDisk();

  long subscribeTransactionBundleAdded(TransactionBundleAddedListener listener);

  long subscribeTransactionBundleRemoved(TransactionBundleRemovedListener listener);

  void unsubscribeTransactionBundleAdded(long listenerId);

  void unsubscribeTransactionBundleRemoved(long listenerId);
}
