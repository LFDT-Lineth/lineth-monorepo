/*
 * Copyright ConsenSys Inc.
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

package net.consensys.linea.zktracer.statemanager;

import static com.google.common.primitives.UnsignedInts.toLong;

import java.util.*;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.fragment.TraceFragment;
import net.consensys.linea.zktracer.module.hub.fragment.account.AccountFragment;
import net.consensys.linea.zktracer.module.hub.fragment.storage.StorageFragment;
import net.consensys.linea.zktracer.module.hub.section.TraceSection;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.datatypes.Address;

public class StateManagerUtils {

  public static int getTxCount(Hub hub) {
    return hub.state().getUserTransactionNumber();
  }

  public static int getRelBlockNoFromBlock(Hub hub, int blockNb) {
    return hub.blockdata().getInstructionsPerBlock().get(toLong(blockNb)).getFirst().relBlock;
  }

  public static int getBlockCount(Hub hub) {
    return hub.blockdata().getInstructionsPerBlock().size();
  }

  public static List<Map<Address, FragmentFirstAndLast<AccountFragment>>>
      computeAccountFirstAndLastMapList(Hub hub) {

    final List<Map<Address, FragmentFirstAndLast<AccountFragment>>> accountFirstAndLastMapList =
        new ArrayList<>();

    final int txCount = getTxCount(hub);
    // We iterate over the transactions
    for (int txNb = 0; txNb < txCount; txNb++) {
      // We create an accountFirstAndLastMap for each transaction
      accountFirstAndLastMapList.add(new HashMap<>());
      // We retrieve the trace section list
      final List<TraceSection> traceSectionList =
          hub.state().getUserTransaction(txNb + 1).traceSections().trace();
      // For each trace section
      for (TraceSection traceSection : traceSectionList) {
        // We iterate over the fragments
        for (TraceFragment traceFragment : traceSection.fragments()) {
          // We cast them to AccountFragment
          // If an exception occurs, it means the Fragment is not an AccountFragment so we
          // disregard it and continue
          try {
            AccountFragment accountFragment = (AccountFragment) traceFragment;
            // We update the AccountFirstAndLastMap
            accountFirstAndLastMapList.set(
                txNb,
                updateAccountFirstAndLast(accountFragment, accountFirstAndLastMapList.get(txNb)));
          } catch (Exception e) {
            // ignore
          }
        }
      }
    }
    return accountFirstAndLastMapList;
  }

  public static List<Map<Map<Address, Bytes32>, FragmentFirstAndLast<StorageFragment>>>
      computeStorageFirstAndLastMapList(Hub hub) {
    final int txCount = getTxCount(hub);

    final List<Map<Map<Address, Bytes32>, FragmentFirstAndLast<StorageFragment>>>
        storageFirstAndLastMapList = new ArrayList<>(txCount);

    // We iterate over the transactions
    for (int txNb = 1; txNb <= txCount; txNb++) {
      // We create an storageFirstAndLastMap for each transaction
      storageFirstAndLastMapList.add(new HashMap<>());
      // We retrieve the trace section list
      final List<TraceSection> traceSectionList =
          hub.state().getUserTransaction(txNb).traceSections().trace();
      // For each trace section
      for (TraceSection traceSection : traceSectionList) {
        // We iterate over the fragments
        for (TraceFragment traceFragment : traceSection.fragments()) {
          // We cast them to StorageFragment
          // If an exception occurs, it means the Fragment is not a StorageFragment so we
          // disregard it and continue
          try {
            final StorageFragment storageFragment = (StorageFragment) traceFragment;
            // We update the storageFirstAndLastMapList
            final int index = txNb - 1; // txNb is 1-based, index is 0-based
            storageFirstAndLastMapList.set(
                index,
                updateStorageFirstAndLast(storageFragment, storageFirstAndLastMapList.get(index)));
          } catch (Exception e) {
            // ignore
          }
        }
      }
    }
    return storageFirstAndLastMapList;
  }

  public static Map<Address, FragmentFirstAndLast<AccountFragment>> updateAccountFirstAndLast(
      AccountFragment fragment,
      Map<Address, FragmentFirstAndLast<AccountFragment>> accountFirstAndLastMap) {
    // Setting the post transaction first and last value
    int dom = fragment.domSubStampsSubFragment().domStamp();
    int sub = fragment.domSubStampsSubFragment().subStamp();

    Address key = fragment.oldState().address();

    if (!accountFirstAndLastMap.containsKey(key)) {
      FragmentFirstAndLast<AccountFragment> txnFirstAndLast =
          new FragmentFirstAndLast<AccountFragment>(fragment, fragment, dom, sub, dom, sub);
      accountFirstAndLastMap.put(key, txnFirstAndLast);
    } else {
      FragmentFirstAndLast<AccountFragment> txnFirstAndLast = accountFirstAndLastMap.get(key);
      // Replace condition
      if (FragmentFirstAndLast.strictlySmallerStamps(
          txnFirstAndLast.getLastDom(), txnFirstAndLast.getLastSub(), dom, sub)) {
        txnFirstAndLast.setLast(fragment);
        txnFirstAndLast.setLastDom(dom);
        txnFirstAndLast.setLastSub(sub);
        accountFirstAndLastMap.put(key, txnFirstAndLast);
      }
    }
    return accountFirstAndLastMap;
  }

  public static Map<Map<Address, Bytes32>, FragmentFirstAndLast<StorageFragment>>
      updateStorageFirstAndLast(
          StorageFragment fragment,
          Map<Map<Address, Bytes32>, FragmentFirstAndLast<StorageFragment>>
              storageFirstAndLastMap) {
    // Setting the post transaction first and last value
    int dom = fragment.getDomSubStampsSubFragment().domStamp();
    int sub = fragment.getDomSubStampsSubFragment().subStamp();
    final Address address = fragment.getStorageSlotIdentifier().getAddress();
    final Bytes32 storageKey = fragment.getStorageSlotIdentifier().getStorageKey();
    Map<Address, Bytes32> key = Map.of(address, storageKey);
    if (!storageFirstAndLastMap.containsKey(key)) {
      FragmentFirstAndLast<StorageFragment> txnFirstAndLast =
          new FragmentFirstAndLast<StorageFragment>(fragment, fragment, dom, sub, dom, sub);
      storageFirstAndLastMap.put(key, txnFirstAndLast);
    } else {
      // the storage key has already been accessed for this account
      FragmentFirstAndLast<StorageFragment> txnFirstAndLast = storageFirstAndLastMap.get(key);
      // Replace condition
      if (FragmentFirstAndLast.strictlySmallerStamps(
          txnFirstAndLast.getLastDom(), txnFirstAndLast.getLastSub(), dom, sub)) {
        txnFirstAndLast.setLast(fragment);
        txnFirstAndLast.setLastDom(dom);
        txnFirstAndLast.setLastSub(sub);
        storageFirstAndLastMap.put(key, txnFirstAndLast);
      }
    }
    return storageFirstAndLastMap;
  }

  public static Map<Address, Map<Integer, FragmentFirstAndLast<AccountFragment>>>
      computeBlockMapAccount(
          Hub hub,
          List<Map<Address, FragmentFirstAndLast<AccountFragment>>> accountFirstAndLastMapList) {
    final int blockCount = getBlockCount(hub);
    final Map<Address, Map<Integer, FragmentFirstAndLast<AccountFragment>>> blockMapAccount =
        new HashMap<>(blockCount);

    final int txCount = getTxCount(hub);
    for (int i = 0; i < blockCount; i++) {
      final int relBlokNoFromBlock = getRelBlockNoFromBlock(hub, i);

      for (int txNb = 0; txNb < txCount; txNb++) {
        final int relBlokNoFromTx =
            hub.txStack().getByAbsoluteTransactionNumber(txNb + 1).getRelativeBlockNumber();
        if (relBlokNoFromTx == relBlokNoFromBlock) {
          final Map<Address, FragmentFirstAndLast<AccountFragment>> accountFirstAndLastMap =
              accountFirstAndLastMapList.get(txNb);
          for (var entry : accountFirstAndLastMap.entrySet()) {
            final Address addr = entry.getKey();
            final FragmentFirstAndLast<AccountFragment> localValueAccount = entry.getValue();

            if (!blockMapAccount.containsKey(addr)) {
              // the pair is not present in the map
              blockMapAccount.put(addr, new HashMap<>());
              blockMapAccount.get(addr).put(relBlokNoFromBlock, localValueAccount);
            } else if (!blockMapAccount.get(addr).containsKey(relBlokNoFromBlock)) {
              // the pair is present in the map, but the block is not present
              blockMapAccount.get(addr).put(relBlokNoFromBlock, localValueAccount);
            } else {
              FragmentFirstAndLast<AccountFragment> fetchedValue =
                  blockMapAccount.get(addr).get(relBlokNoFromBlock);
              // we make a copy that will be modified to not change the values already present in
              // the
              // transaction maps
              final FragmentFirstAndLast<AccountFragment> blockValue = fetchedValue.copy();
              // update the first part of the blockValue
              // Todo: Refactor and remove code duplication
              if (FragmentFirstAndLast.strictlySmallerStamps(
                  localValueAccount.getFirstDom(),
                  localValueAccount.getFirstSub(),
                  blockValue.getFirstDom(),
                  blockValue.getFirstSub())) {
                // chronologically checks that localValue.First is before blockValue.First
                // localValue comes chronologically before, and should be the first value of the
                // map.
                blockValue.setFirst(localValueAccount.getFirst());
                blockValue.setFirstDom(localValueAccount.getFirstDom());
                blockValue.setFirstSub(localValueAccount.getFirstSub());
              }

              // update the last part of the blockValue
              if (FragmentFirstAndLast.strictlySmallerStamps(
                  blockValue.getLastDom(),
                  blockValue.getLastSub(),
                  localValueAccount.getLastDom(),
                  localValueAccount.getLastSub())) {
                // chronologically checks that blockValue.Last is before localValue.Last
                // localValue comes chronologically after, and should be the final value of the map.
                blockValue.setLast(localValueAccount.getLast());
                blockValue.setLastDom(localValueAccount.getLastDom());
                blockValue.setLastSub(localValueAccount.getLastSub());
              }
              blockMapAccount.get(addr).put(relBlokNoFromBlock, blockValue);
            }
          }
        }
      }
    }
    return blockMapAccount;
  }

  public static Map<Map<Address, Bytes32>, Map<Integer, FragmentFirstAndLast<StorageFragment>>>
      computeBlockMapStorage(
          Hub hub,
          List<Map<Map<Address, Bytes32>, FragmentFirstAndLast<StorageFragment>>>
              storageFirstAndLastMapList) {
    Map<Map<Address, Bytes32>, Map<Integer, FragmentFirstAndLast<StorageFragment>>>
        blockMapStorage = new HashMap<>();

    int blockCount = getBlockCount(hub);
    int txCount = getTxCount(hub);
    for (int i = 0; i < blockCount; i++) {
      int relBlokNoFromBlock = getRelBlockNoFromBlock(hub, i);
      for (int txNb = 0; txNb < txCount; txNb++) {
        int relBlokNoFromTx =
            hub.txStack().getByAbsoluteTransactionNumber(txNb + 1).getRelativeBlockNumber();

        if (relBlokNoFromTx == relBlokNoFromBlock) {
          Map<Map<Address, Bytes32>, FragmentFirstAndLast<StorageFragment>> storageFirstAndLastMap =
              storageFirstAndLastMapList.get(txNb);

          // Update the block map for storage
          for (var entry : storageFirstAndLastMap.entrySet()) {
            Map<Address, Bytes32> addrStorageMapKey = entry.getKey();
            // localValue exists for sure because addr belongs to the keySet of the local map
            FragmentFirstAndLast<StorageFragment> localValueStorage = entry.getValue();

            if (!blockMapStorage.containsKey(addrStorageMapKey)) {
              // the pair is not present in the map
              blockMapStorage.put(addrStorageMapKey, new HashMap<>());
              blockMapStorage.get(addrStorageMapKey).put(relBlokNoFromBlock, localValueStorage);
            } else if (!blockMapStorage.get(addrStorageMapKey).containsKey(relBlokNoFromBlock)) {
              blockMapStorage.get(addrStorageMapKey).put(relBlokNoFromBlock, localValueStorage);
            } else {
              FragmentFirstAndLast<StorageFragment> fetchedValue =
                  blockMapStorage.get(addrStorageMapKey).get(relBlokNoFromBlock);
              // we make a copy that will be modified to not change the values already present in
              // the
              // transaction maps
              FragmentFirstAndLast<StorageFragment> blockValueStorage = fetchedValue.copy();
              // update the first part of the blockValue
              // Todo: Refactor and remove code duplication
              if (FragmentFirstAndLast.strictlySmallerStamps(
                  localValueStorage.getFirstDom(),
                  localValueStorage.getFirstSub(),
                  blockValueStorage.getFirstDom(),
                  blockValueStorage.getFirstSub())) {
                // chronologically checks that localValue.First is before blockValue.First
                // localValue comes chronologically before, and should be the first value of the
                // map.
                blockValueStorage.setFirst(localValueStorage.getFirst());
                blockValueStorage.setFirstDom(localValueStorage.getFirstDom());
                blockValueStorage.setFirstSub(localValueStorage.getFirstSub());
              }

              // update the last part of the blockValue
              if (FragmentFirstAndLast.strictlySmallerStamps(
                  blockValueStorage.getLastDom(),
                  blockValueStorage.getLastSub(),
                  localValueStorage.getLastDom(),
                  localValueStorage.getLastSub())) {
                // chronologically checks that blockValue.Last is before localValue.Last
                // localValue comes chronologically after, and should be the final value of the map.
                blockValueStorage.setLast(localValueStorage.getLast());
                blockValueStorage.setLastDom(localValueStorage.getLastDom());
                blockValueStorage.setLastSub(localValueStorage.getLastSub());
              }
              blockMapStorage.get(addrStorageMapKey).put(relBlokNoFromBlock, blockValueStorage);
            }
          }
        }
      }
    }
    return blockMapStorage;
  }

  public static Map<Address, FragmentFirstAndLast<AccountFragment>> computeConflationMapAccount(
      Hub hub,
      List<Map<Address, FragmentFirstAndLast<AccountFragment>>> accountFirstAndLastMapList,
      Map<Address, Map<Integer, FragmentFirstAndLast<AccountFragment>>> blockMapAccount) {

    final Map<Address, FragmentFirstAndLast<AccountFragment>> conflationMapAccount =
        new HashMap<>();

    final int txCount = getTxCount(hub);
    final int blockCount = getBlockCount(hub);
    final HashSet<Address> allAccounts = new HashSet<>();

    // We iterate over the transactions
    for (int txNb = 0; txNb < txCount; txNb++) {

      final Map<Address, FragmentFirstAndLast<AccountFragment>> txnMapAccount =
          accountFirstAndLastMapList.get(txNb);

      allAccounts.addAll(txnMapAccount.keySet());
    }

    for (Address addr : allAccounts) {
      FragmentFirstAndLast<AccountFragment> firstValue = null;
      // Update the first value of the conflation map for Account
      // We update the value of the conflation map with the earliest value of the block map
      // TODO: change transients.block().blockNumber()
      for (int i = 1; i <= blockCount; i++) {
        if (blockMapAccount.containsKey(addr) && blockMapAccount.get(addr).containsKey(i)) {
          firstValue = blockMapAccount.get(addr).get(i);
          conflationMapAccount.put(addr, firstValue);
          break;
        }
      }

      // Update the last value of the conflation map
      // We update the last value for the conflation map with the latest blockMap's last values,
      // if some address is not present in the last block, we ignore the corresponding account
      for (int i = blockCount; i >= 1; i--) {
        if (blockMapAccount.containsKey(addr) && blockMapAccount.get(addr).containsKey(i)) {
          final FragmentFirstAndLast<AccountFragment> blockValue = blockMapAccount.get(addr).get(i);

          final FragmentFirstAndLast<AccountFragment> updatedValue =
              new FragmentFirstAndLast<AccountFragment>(
                  firstValue.getFirst(),
                  blockValue.getLast(),
                  firstValue.getFirstDom(),
                  firstValue.getFirstSub(),
                  blockValue.getLastDom(),
                  blockValue.getLastSub());
          conflationMapAccount.put(addr, updatedValue);
          break;
        }
      }
    }
    return conflationMapAccount;
  }

  public static Map<Map<Address, Bytes32>, FragmentFirstAndLast<StorageFragment>>
      computeConflationMapStorage(
          Hub hub,
          List<Map<Map<Address, Bytes32>, FragmentFirstAndLast<StorageFragment>>>
              storageFirstAndLastMapList,
          Map<Map<Address, Bytes32>, Map<Integer, FragmentFirstAndLast<StorageFragment>>>
              blockMapStorage) {
    Map<Map<Address, Bytes32>, FragmentFirstAndLast<StorageFragment>> conflationMapStorage =
        new HashMap<>();

    int txCount = getTxCount(hub);
    int blockCount = getBlockCount(hub);
    HashSet<Map<Address, Bytes32>> allStorage = new HashSet<Map<Address, Bytes32>>();

    // We iterate over the transactions
    for (int txNb = 0; txNb < txCount; txNb++) {

      Map<Map<Address, Bytes32>, FragmentFirstAndLast<StorageFragment>> txnMapAccount =
          storageFirstAndLastMapList.get(txNb);

      allStorage.addAll(txnMapAccount.keySet());
    }

    for (Map<Address, Bytes32> addrStorageKeyPair : allStorage) {
      FragmentFirstAndLast<StorageFragment> firstValue = null;
      // Update the first value of the conflation map for Storage
      // We update the value of the conflation map with the earliest value of the block map
      for (int i = 1; i <= blockCount; i++) {
        if (blockMapStorage.containsKey(addrStorageKeyPair)
            && blockMapStorage.get(addrStorageKeyPair).containsKey(i)) {
          firstValue = blockMapStorage.get(addrStorageKeyPair).get(i);
          conflationMapStorage.put(addrStorageKeyPair, firstValue);
          break;
        }
      }
      // Update the last value of the conflation map
      // We update the last value for the conflation map with the latest blockMap's last values,
      // if some address is not present in the last block, we ignore the corresponding account
      for (int i = blockCount; i >= 1; i--) {
        if (blockMapStorage.containsKey(addrStorageKeyPair)
            && blockMapStorage.get(addrStorageKeyPair).containsKey(i)) {
          FragmentFirstAndLast<StorageFragment> blockValue =
              blockMapStorage.get(addrStorageKeyPair).get(i);

          FragmentFirstAndLast<StorageFragment> updatedValue =
              new FragmentFirstAndLast<StorageFragment>(
                  firstValue.getFirst(),
                  blockValue.getLast(),
                  firstValue.getFirstDom(),
                  firstValue.getFirstSub(),
                  blockValue.getLastDom(),
                  blockValue.getLastSub());
          conflationMapStorage.put(addrStorageKeyPair, updatedValue);
          break;
        }
      }
    }
    return conflationMapStorage;
  }
}
