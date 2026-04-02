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

package net.consensys.linea.testing;

import com.google.common.cache.Cache;
import com.google.common.cache.CacheBuilder;
import java.util.Map;
import java.util.Optional;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.ethereum.core.InMemoryKeyValueStorageProvider;
import org.hyperledger.besu.ethereum.referencetests.BonsaiReferenceTestWorldState;
import org.hyperledger.besu.ethereum.referencetests.BonsaiReferenceTestWorldStateStorage;
import org.hyperledger.besu.ethereum.referencetests.ReferenceTestWorldState;
import org.hyperledger.besu.ethereum.trie.pathbased.bonsai.cache.BonsaiCachedMerkleTrieLoader;
import org.hyperledger.besu.ethereum.trie.pathbased.bonsai.cache.CodeCache;
import org.hyperledger.besu.ethereum.trie.pathbased.bonsai.cache.NoOpBonsaiCachedWorldStorageManager;
import org.hyperledger.besu.ethereum.trie.pathbased.bonsai.storage.BonsaiPreImageProxy;
import org.hyperledger.besu.ethereum.trie.pathbased.bonsai.storage.BonsaiWorldStateKeyValueStorage;
import org.hyperledger.besu.ethereum.trie.pathbased.common.trielog.TrieLogAddedEvent;
import org.hyperledger.besu.ethereum.trie.pathbased.common.trielog.TrieLogManager;
import org.hyperledger.besu.ethereum.trie.pathbased.common.worldview.PathBasedWorldState;
import org.hyperledger.besu.ethereum.trie.pathbased.common.worldview.accumulator.PathBasedWorldStateUpdateAccumulator;
import org.hyperledger.besu.ethereum.worldstate.DataStorageConfiguration;
import org.hyperledger.besu.evm.internal.EvmConfiguration;
import org.hyperledger.besu.evm.worldstate.WorldUpdater;
import org.hyperledger.besu.metrics.noop.NoOpMetricsSystem;
import org.hyperledger.besu.plugin.data.BlockHeader;
import org.hyperledger.besu.plugin.services.trielogs.TrieLog;

/**
 * Extension of {@link BonsaiReferenceTestWorldState} that allows configuring {@link
 * DataStorageConfiguration} and disabling parallel state root computation.
 *
 * <p>This class exists because Besu's {@link BonsaiReferenceTestWorldState#create} method hardcodes
 * {@link DataStorageConfiguration#DEFAULT_BONSAI_CONFIG} and uses {@code
 * createStatefulConfigWithTrie()} which enables parallel state root computation. For deterministic
 * testing, we need to disable parallel computation.
 */
public class LineaBonsaiReferenceTestWorldState extends BonsaiReferenceTestWorldState {

  protected LineaBonsaiReferenceTestWorldState(
      final BonsaiReferenceTestWorldStateStorage worldStateKeyValueStorage,
      final BonsaiCachedMerkleTrieLoader bonsaiCachedMerkleTrieLoader,
      final NoOpBonsaiCachedWorldStorageManager cachedWorldStorageManager,
      final TrieLogManager trieLogManager,
      final BonsaiPreImageProxy preImageProxy,
      final EvmConfiguration evmConfiguration,
      final boolean parallelStateRootComputationEnabled) {
    super(
        worldStateKeyValueStorage,
        bonsaiCachedMerkleTrieLoader,
        cachedWorldStorageManager,
        trieLogManager,
        preImageProxy,
        evmConfiguration);
    // Override the WorldStateConfig that was set by BonsaiReferenceTestWorldState's constructor
    // (via createStatefulConfigWithTrie() which hardcodes parallelStateRootComputationEnabled=true)
    this.worldStateConfig.setParallelStateRootComputationEnabled(
        parallelStateRootComputationEnabled);
  }

  /**
   * Creates a {@link ReferenceTestWorldState} with a custom {@link DataStorageConfiguration} and
   * parallel state root computation disabled.
   *
   * @param accounts the initial accounts to populate the world state with
   * @param evmConfiguration the EVM configuration
   * @param dataStorageConfiguration the data storage configuration
   * @return a new ReferenceTestWorldState instance with parallel computation disabled
   */
  public static ReferenceTestWorldState create(
      final Map<String, ReferenceTestWorldState.AccountMock> accounts,
      final EvmConfiguration evmConfiguration,
      final DataStorageConfiguration dataStorageConfiguration) {
    return create(accounts, evmConfiguration, dataStorageConfiguration, false);
  }

  /**
   * Creates a {@link ReferenceTestWorldState} with a custom {@link DataStorageConfiguration} and
   * configurable parallel state root computation.
   *
   * @param accounts the initial accounts to populate the world state with
   * @param evmConfiguration the EVM configuration
   * @param dataStorageConfiguration the data storage configuration
   * @param parallelStateRootComputationEnabled whether to enable parallel state root computation
   * @return a new ReferenceTestWorldState instance
   */
  public static ReferenceTestWorldState create(
      final Map<String, ReferenceTestWorldState.AccountMock> accounts,
      final EvmConfiguration evmConfiguration,
      final DataStorageConfiguration dataStorageConfiguration,
      final boolean parallelStateRootComputationEnabled) {

    final var metricsSystem = new NoOpMetricsSystem();

    final var bonsaiCachedMerkleTrieLoader = new BonsaiCachedMerkleTrieLoader(metricsSystem);

    final var trieLogManager = new InMemoryTrieLogManager();

    final var preImageProxy = new BonsaiPreImageProxy.BonsaiReferenceTestPreImageProxy();

    final var bonsaiWorldStateKeyValueStorage =
        new BonsaiWorldStateKeyValueStorage(
            new InMemoryKeyValueStorageProvider(), metricsSystem, dataStorageConfiguration);

    final var worldStateKeyValueStorage =
        new BonsaiReferenceTestWorldStateStorage(bonsaiWorldStateKeyValueStorage, preImageProxy);

    final var noOpCachedWorldStorageManager =
        new NoOpBonsaiCachedWorldStorageManager(
            bonsaiWorldStateKeyValueStorage, evmConfiguration, new CodeCache());

    final var worldState =
        new LineaBonsaiReferenceTestWorldState(
            worldStateKeyValueStorage,
            bonsaiCachedMerkleTrieLoader,
            noOpCachedWorldStorageManager,
            trieLogManager,
            preImageProxy,
            evmConfiguration,
            parallelStateRootComputationEnabled);

    final WorldUpdater updater = worldState.updater();
    for (final Map.Entry<String, ReferenceTestWorldState.AccountMock> entry : accounts.entrySet()) {
      ReferenceTestWorldState.insertAccount(
          updater, Address.fromHexString(entry.getKey()), entry.getValue());
    }
    updater.commit();
    return worldState;
  }

  /**
   * In-memory TrieLogManager for reference tests. This is equivalent to Besu's
   * ReferenceTestsInMemoryTrieLogManager which is package-private.
   */
  private static class InMemoryTrieLogManager extends TrieLogManager {

    private final Cache<Hash, byte[]> trieLogCache =
        CacheBuilder.newBuilder().maximumSize(5).build();

    InMemoryTrieLogManager() {
      super(null, null, 0, null);
    }

    @Override
    public synchronized void saveTrieLog(
        final PathBasedWorldStateUpdateAccumulator<?> localUpdater,
        final Hash forWorldStateRootHash,
        final BlockHeader forBlockHeader,
        final PathBasedWorldState forWorldState) {
      TrieLog trieLog = trieLogFactory.create(localUpdater, forBlockHeader);
      trieLogCache.put(forBlockHeader.getBlockHash(), trieLogFactory.serialize(trieLog));
      trieLogObservers.forEach(o -> o.onTrieLogAdded(new TrieLogAddedEvent(trieLog)));
    }

    @Override
    public long getMaxLayersToLoad() {
      return 0;
    }

    @Override
    public Optional<TrieLog> getTrieLogLayer(final Hash blockHash) {
      final byte[] trielog = trieLogCache.getIfPresent(blockHash);
      trieLogCache.invalidate(blockHash);
      return Optional.ofNullable(trieLogFactory.deserialize(trielog));
    }
  }
}
