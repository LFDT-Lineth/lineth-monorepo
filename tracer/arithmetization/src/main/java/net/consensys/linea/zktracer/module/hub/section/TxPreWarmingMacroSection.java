/*
 * Copyright ConsenSys Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use hub file except in compliance with
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

package net.consensys.linea.zktracer.module.hub.section;

import static com.google.common.base.Preconditions.*;
import static net.consensys.linea.zktracer.module.hub.fragment.storage.StorageFragmentPurpose.PRE_WARMING;
import static net.consensys.linea.zktracer.types.AddressUtils.*;

import java.util.*;
import net.consensys.linea.zktracer.module.hub.AccountSnapshot;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.TransactionProcessingType;
import net.consensys.linea.zktracer.module.hub.fragment.DomSubStampsSubFragment;
import net.consensys.linea.zktracer.module.hub.fragment.TraceFragment;
import net.consensys.linea.zktracer.module.hub.fragment.storage.StorageFragment;
import net.consensys.linea.zktracer.module.hub.state.State;
import net.consensys.linea.zktracer.types.EWord;
import net.consensys.linea.zktracer.types.TransactionProcessingMetadata;
import org.apache.tuweni.bytes.Bytes32;
import org.apache.tuweni.units.bigints.UInt256;
import org.hyperledger.besu.datatypes.AccessListEntry;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.evm.worldstate.WorldView;

public class TxPreWarmingMacroSection {
  public TxPreWarmingMacroSection(
      Hub hub, WorldView world, Map<Address, AccountSnapshot> latestAccountSnapshots) {

    final TransactionProcessingMetadata txMetadata = hub.txStack().current();

    checkState(
        txMetadata.getBesuTransaction().getAccessList().isPresent(),
        "TX_WARM tracing only applies to transactions containing an access list");
    checkState(
        !txMetadata.getBesuTransaction().getAccessList().get().isEmpty(),
        "TX_WARM tracing only applies to transactions containing a nontrivial access list");

    final List<AccessListEntry> accessList = txMetadata.getBesuTransaction().getAccessList().get();
    final HashMap<Address, Set<Bytes32>> seenKeys = new HashMap<>();

    for (AccessListEntry entry : accessList) {
      final Address address = entry.address();

      checkArgument(
          !hub.deploymentStatusOf(address),
          "Deployment status during TX_INIT phase of any accountAddress should always be false");

      final AccountSnapshot preWarmingAccountSnapshot =
          (latestAccountSnapshots.containsKey(address))
              ? latestAccountSnapshots.get(address)
              : (world.get(address) == null)
                  ? AccountSnapshot.fromAddress(
                      address,
                      precompileAddressOsaka.contains(address),
                      hub.deploymentNumberOf(address),
                      hub.deploymentStatusOf(address),
                      hub.delegationNumberOf(address))
                  : AccountSnapshot.canonical(
                      hub, world, address, precompileAddressOsaka.contains(address));

      final AccountSnapshot postWarmingAccountSnapshot =
          preWarmingAccountSnapshot.deepCopy().turnOnWarmth();

      // we update the accountSnapshot map
      latestAccountSnapshots.put(address, postWarmingAccountSnapshot);

      final DomSubStampsSubFragment domSubStampsSubFragment =
          new DomSubStampsSubFragment(
              DomSubStampsSubFragment.DomSubType.STANDARD, hub.stamp() + 1, 0, 0, 0, 0, 0);

      new TxPrewarmingSection(
          hub,
          hub.factories()
              .accountFragment()
              .makeWithTrm(
                  preWarmingAccountSnapshot,
                  postWarmingAccountSnapshot,
                  address,
                  domSubStampsSubFragment,
                  TransactionProcessingType.USER));

      final List<Bytes32> keys = entry.storageKeys();
      for (Bytes32 k : keys) {

        final UInt256 key = UInt256.fromBytes(k);
        final EWord value =
            Optional.ofNullable(world.get(address))
                .map(account -> EWord.of(account.getStorageValue(key)))
                .orElse(EWord.ZERO);

        final State.StorageSlotIdentifier storageSlotIdentifier =
            new State.StorageSlotIdentifier(address, hub.deploymentNumberOf(address), k);

        final StorageFragment storageFragment =
            new StorageFragment(
                hub,
                new State.StorageSlotIdentifier(address, hub.deploymentNumberOf(address), key),
                value,
                value,
                value,
                seenKeys.computeIfAbsent(address, x -> new HashSet<>()).contains(key),
                true,
                DomSubStampsSubFragment.standardDomSubStamps(hub.stamp() + 1, 0),
                PRE_WARMING);

        new TxPrewarmingSection(hub, storageFragment);
        hub.state.updateOrInsertStorageSlotOccurrence(storageSlotIdentifier, storageFragment);

        seenKeys.get(address).add(key);
      }
    }

    final Transaction besuTx = txMetadata.getBesuTransaction();
    final Address senderAddress = besuTx.getSender();
    final Address recipientAddress = effectiveToAddress(besuTx);
    txMetadata.isSenderPreWarmed(latestAccountSnapshots.get(senderAddress).isWarm());
    txMetadata.isRecipientPreWarmed(latestAccountSnapshots.get(recipientAddress).isWarm());
    txMetadata.isCoinbasePreWarmed(latestAccountSnapshots.get(hub.coinbaseAddress()).isWarm());
  }

  public static class TxPrewarmingSection extends TraceSection {
    public TxPrewarmingSection(Hub hub, TraceFragment fragment) {
      super(hub, (short) 1);
      this.addFragments(fragment);
    }
  }
}
