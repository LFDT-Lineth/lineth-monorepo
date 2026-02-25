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

package net.consensys.linea.zktracer.module.hub.fragment.storage;

import static net.consensys.linea.zktracer.module.hub.fragment.storage.StorageFragmentPurpose.*;
import static net.consensys.linea.zktracer.types.AddressUtils.hiPart;
import static net.consensys.linea.zktracer.types.AddressUtils.loPart;

import lombok.Getter;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.defer.PostBlockDefer;
import net.consensys.linea.zktracer.module.hub.fragment.DomSubStampsSubFragment;
import net.consensys.linea.zktracer.module.hub.fragment.TraceFragment;
import net.consensys.linea.zktracer.module.hub.state.Block;
import net.consensys.linea.zktracer.module.hub.state.State;
import net.consensys.linea.zktracer.types.EWord;
import org.hyperledger.besu.datatypes.Address;

@Getter
public final class StorageFragment implements TraceFragment, PostBlockDefer {
  private final State hubState;
  private final State.StorageSlotIdentifier storageSlotIdentifier;
  private final EWord valueOriginal;
  private final EWord valueCurrent;
  private final EWord valueNext;
  private final boolean incomingWarmth;
  private final boolean outgoingWarmth;
  private final DomSubStampsSubFragment domSubStampsSubFragment;
  private final int blockNumber;
  private final StorageFragmentPurpose purpose;

  public StorageFragment(
      Hub hub,
      State.StorageSlotIdentifier storageId,
      EWord valueOriginal,
      EWord valueCurrent,
      EWord valueNext,
      boolean incomingWarmth,
      boolean outgoingWarmth,
      DomSubStampsSubFragment domSubSubFragment,
      StorageFragmentPurpose purpose) {
    hubState = hub.state;
    storageSlotIdentifier = storageId;
    this.valueOriginal = valueOriginal;
    this.valueCurrent = valueCurrent;
    this.valueNext = valueNext;
    this.incomingWarmth = incomingWarmth;
    this.outgoingWarmth = outgoingWarmth;
    domSubStampsSubFragment = domSubSubFragment;
    blockNumber = hub.blockStack().currentRelativeBlockNumber();
    this.purpose = purpose;

    // This allows us to keep track of account that are accessed by the HUB during the execution of
    // the block
    if (maybeNewStorageSlot(purpose)) {
      hub.defers().scheduleForPostBlock(this);
    }
  }

  public static StorageFragment systemTransactionStoring(
      Hub hub, Address address, EWord key, EWord currentValue, EWord newValue, int domOffset) {

    return new StorageFragment(
        hub,
        new State.StorageSlotIdentifier(address, hub.deploymentNumberOf(address), key),
        currentValue,
        currentValue,
        newValue,
        false,
        false,
        DomSubStampsSubFragment.standardDomSubStamps(hub.stamp(), domOffset),
        SSTORE_SYSTEM_TRANSACTION);
  }

  public Trace.Hub trace(Trace.Hub trace) {
    domSubStampsSubFragment.traceHub(trace);

    return trace
        .peekAtStorage(true)
        .pStorageAddressHi(hiPart(storageSlotIdentifier.getAddress()))
        .pStorageAddressLo(loPart(storageSlotIdentifier.getAddress()))
        .pStorageDeploymentNumber(storageSlotIdentifier.getDeploymentNumber())
        .pStorageStorageKeyHi(EWord.of(storageSlotIdentifier.getStorageKey()).hi())
        .pStorageStorageKeyLo(EWord.of(storageSlotIdentifier.getStorageKey()).lo())
        .pStorageValueOrigHi(valueOriginal.hi())
        .pStorageValueOrigLo(valueOriginal.lo())
        .pStorageValueCurrHi(valueCurrent.hi())
        .pStorageValueCurrLo(valueCurrent.lo())
        .pStorageValueNextHi(valueNext.hi())
        .pStorageValueNextLo(valueNext.lo())
        .pStorageWarmth(incomingWarmth)
        .pStorageWarmthNew(outgoingWarmth)
        .pStorageValueOrigIsZero(valueOriginal.isZero())
        .pStorageValueCurrIsOrig(valueCurrent.equals(valueOriginal))
        .pStorageValueCurrIsZero(valueCurrent.isZero())
        .pStorageValueNextIsCurr(valueNext.equals(valueCurrent))
        .pStorageValueNextIsZero(valueNext.isZero())
        .pStorageValueNextIsOrig(valueNext.equals(valueOriginal))
        .pStorageSloadOperation(
            purpose == StorageFragmentPurpose.SLOAD_DOING
                || purpose == StorageFragmentPurpose.SLOAD_UNDOING)
        .pStorageSstoreOperation(
            purpose == StorageFragmentPurpose.SSTORE_DOING
                || purpose == StorageFragmentPurpose.SSTORE_UNDOING
                || purpose == SSTORE_SYSTEM_TRANSACTION);
  }

  @Override
  public void resolvePostBlock(Hub hub) {
    final Block currentBlock = hub.blockStack().currentBlock();
    currentBlock.addStorageSeenByHub(
        storageSlotIdentifier.getAddress(), storageSlotIdentifier.getStorageKey());
  }
}
