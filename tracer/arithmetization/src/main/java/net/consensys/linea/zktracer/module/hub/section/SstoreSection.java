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

package net.consensys.linea.zktracer.module.hub.section;

import static com.google.common.base.Preconditions.*;
import static net.consensys.linea.zktracer.module.hub.fragment.storage.StorageFragmentPurpose.SSTORE_DOING;
import static net.consensys.linea.zktracer.module.hub.fragment.storage.StorageFragmentPurpose.SSTORE_UNDOING;

import lombok.Getter;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.defer.PostRollbackDefer;
import net.consensys.linea.zktracer.module.hub.fragment.ContextFragment;
import net.consensys.linea.zktracer.module.hub.fragment.DomSubStampsSubFragment;
import net.consensys.linea.zktracer.module.hub.fragment.imc.ImcFragment;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.opcodes.SstoreOobCall;
import net.consensys.linea.zktracer.module.hub.fragment.storage.StorageFragment;
import net.consensys.linea.zktracer.module.hub.signals.Exceptions;
import net.consensys.linea.zktracer.module.hub.state.State;
import net.consensys.linea.zktracer.runtime.callstack.CallFrame;
import net.consensys.linea.zktracer.types.EWord;
import org.apache.tuweni.bytes.Bytes32;
import org.apache.tuweni.units.bigints.UInt256;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.evm.frame.MessageFrame;
import org.hyperledger.besu.evm.worldstate.WorldView;

@Getter
public class SstoreSection extends TraceSection implements PostRollbackDefer {

  public static final short NB_ROWS_HUB_STORAGE = 5;
  // 1 stack + 1 CON + 1 IMC + 1 STO + 1 STO (rollback) = 5 + potentially 1 CON if exception

  final WorldView world;
  final Address accountAddress;
  final int accountAddressDeploymentNumber;
  final Bytes32 storageKey;
  final boolean incomingWarmth;
  final EWord valueOriginal;
  final EWord valueCurrent;
  final EWord valueNext;
  final int hubStamp;

  public SstoreSection(Hub hub, WorldView worldView) {
    super(hub, (short) (NB_ROWS_HUB_STORAGE + (Exceptions.any(hub.pch().exceptions()) ? 1 : 0)));

    world = worldView;
    hubStamp = hub.stamp();
    accountAddress = hub.accountAddress();
    accountAddressDeploymentNumber = hub.deploymentNumberOfAccountAddress();
    storageKey = Bytes32.leftPad(hub.messageFrame().getStackItem(0));
    incomingWarmth = hub.messageFrame().getWarmedUpStorage().contains(accountAddress, storageKey);
    valueOriginal =
        EWord.of(
            worldView.get(accountAddress).getOriginalStorageValue(UInt256.fromBytes(storageKey)));
    valueCurrent =
        EWord.of(worldView.get(accountAddress).getStorageValue(UInt256.fromBytes(storageKey)));
    valueNext = EWord.of(hub.messageFrame().getStackItem(1));
    final short exceptions = hub.pch().exceptions();

    final boolean staticContextException = Exceptions.staticFault(exceptions);
    final boolean sstoreException = Exceptions.outOfSStore(exceptions);

    // CONTEXT fragment
    final ContextFragment readCurrentContext = ContextFragment.readCurrentContextData(hub);
    this.addStackAndFragments(hub, readCurrentContext);

    if (staticContextException) {
      return;
    }

    // MISC fragment
    final ImcFragment miscForSstore = ImcFragment.empty(hub);
    this.addFragment(miscForSstore);

    miscForSstore.callOob(new SstoreOobCall());

    if (sstoreException) {
      return;
    }

    checkArgument(
        Exceptions.outOfGasException(exceptions) || Exceptions.none(exceptions),
        "SSTORE may only throw STATICX, SSTOREX or OOGX exceptions, in that order of priority");

    hub.defers().scheduleForPostRollback(this, hub.currentFrame());

    // STORAGE fragment (for doing)
    final StorageFragment doingSstore = this.doingSstore(hub);
    this.addFragment(doingSstore);

    // Update the First Last time seen map of storage keys
    final State.StorageSlotIdentifier storageSlotIdentifier =
        new State.StorageSlotIdentifier(
            accountAddress,
            hub.transients().conflation().deploymentInfo().deploymentNumber(accountAddress),
            storageKey);
    hub.state.updateOrInsertStorageSlotOccurrence(storageSlotIdentifier, doingSstore);

    // set the refundDelta
    commonValues.refundDelta(
        hub.gasProjector
            .of(hub.currentFrame().frame(), hub.opCodeData())
            .refund()); // Note: we can't use Besu's refund value, as our is only for non-reverting
    // context
  }

  private StorageFragment doingSstore(Hub hub) {

    return new StorageFragment(
        hub,
        new State.StorageSlotIdentifier(accountAddress, accountAddressDeploymentNumber, storageKey),
        valueOriginal,
        valueCurrent,
        valueNext,
        incomingWarmth,
        true,
        DomSubStampsSubFragment.standardDomSubStamps(this.hubStamp(), 0),
        SSTORE_DOING);
  }

  @Override
  public void resolveUponRollback(Hub hub, MessageFrame messageFrame, CallFrame callFrame) {
    final DomSubStampsSubFragment undoingDomSubStamps =
        DomSubStampsSubFragment.revertWithCurrentDomSubStamps(hubStamp, callFrame.revertStamp(), 0);

    final StorageFragment undoingSstoreStorageFragment =
        new StorageFragment(
            hub,
            new State.StorageSlotIdentifier(
                accountAddress, accountAddressDeploymentNumber, storageKey),
            valueOriginal,
            valueNext,
            valueCurrent,
            true,
            incomingWarmth,
            undoingDomSubStamps,
            SSTORE_UNDOING);

    this.addFragment(undoingSstoreStorageFragment);

    // undo the refund
    commonValues.refundDelta(0);
  }
}
