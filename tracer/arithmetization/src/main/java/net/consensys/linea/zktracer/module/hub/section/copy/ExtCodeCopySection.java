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

package net.consensys.linea.zktracer.module.hub.section.copy;

import static com.google.common.base.Preconditions.*;
import static net.consensys.linea.zktracer.module.hub.signals.Exceptions.outOfGasException;
import static net.consensys.linea.zktracer.types.AddressUtils.isAddressWarm;

import net.consensys.linea.zktracer.module.hub.AccountSnapshot;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.TransactionProcessingType;
import net.consensys.linea.zktracer.module.hub.defer.PostRollbackDefer;
import net.consensys.linea.zktracer.module.hub.fragment.DomSubStampsSubFragment;
import net.consensys.linea.zktracer.module.hub.fragment.account.AccountFragment;
import net.consensys.linea.zktracer.module.hub.fragment.imc.ImcFragment;
import net.consensys.linea.zktracer.module.hub.fragment.imc.MxpCall;
import net.consensys.linea.zktracer.module.hub.fragment.imc.mmu.MmuCall;
import net.consensys.linea.zktracer.module.hub.section.TraceSection;
import net.consensys.linea.zktracer.module.hub.signals.Exceptions;
import net.consensys.linea.zktracer.runtime.callstack.CallFrame;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.evm.account.Account;
import org.hyperledger.besu.evm.frame.MessageFrame;

public class ExtCodeCopySection extends TraceSection implements PostRollbackDefer {

  public static final short NB_ROWS_HUB_EXT_CODE_COPY = 4; // 4 = 1 + 3

  final Bytes rawAddress;
  final Address address;
  final int incomingDeploymentNumber;
  final boolean incomingDeploymentStatus;
  final boolean incomingWarmth;

  AccountSnapshot firstForeign;
  AccountSnapshot firstForeignNew;

  AccountSnapshot secondForeign;
  AccountSnapshot secondForeignNew;

  public ExtCodeCopySection(Hub hub, MessageFrame frame) {
    super(hub, NB_ROWS_HUB_EXT_CODE_COPY);

    rawAddress = frame.getStackItem(0);
    address = Address.extract(Bytes32.leftPad(rawAddress));
    incomingDeploymentNumber = hub.deploymentNumberOf(address);
    incomingDeploymentStatus = hub.deploymentStatusOf(address);
    incomingWarmth = isAddressWarm(hub.fork, frame, address);
    final ImcFragment imcFragment = ImcFragment.empty(hub);

    this.addStack(hub);
    this.addFragment(imcFragment);

    final MxpCall mxpCall = MxpCall.newMxpCall(hub);
    imcFragment.callMxp(mxpCall);

    final short exceptions = hub.pch().exceptions();
    checkArgument(
        mxpCall.mxpx == Exceptions.memoryExpansionException(exceptions),
        "EXTCODECOPY: mxp and hub disagree on MXPX");

    // The MXPX case
    if (mxpCall.mxpx) {
      return;
    }

    final Account foreignAccount = frame.getWorldUpdater().get(address);

    firstForeign =
        foreignAccount != null
            ? AccountSnapshot.canonical(hub, address)
            : AccountSnapshot.fromAddress(
                address,
                incomingWarmth,
                incomingDeploymentNumber,
                incomingDeploymentStatus,
                hub.delegationNumberOf(address));

    final DomSubStampsSubFragment doingDomSubStamps =
        DomSubStampsSubFragment.standardDomSubStamps(this.hubStamp(), 0);

    // The OOGX case
    if (outOfGasException(exceptions)) {
      // the last context row will be added automatically
      final AccountFragment accountReadingFragment =
          hub.factories()
              .accountFragment()
              .makeWithTrm(
                  firstForeign,
                  firstForeign,
                  rawAddress,
                  doingDomSubStamps,
                  TransactionProcessingType.USER);

      this.addFragment(accountReadingFragment);
      return;
    }

    // The unexceptional case
    checkArgument(Exceptions.none(exceptions), "EXTCODECOPY: unexpected exception");

    final boolean triggerMmu = mxpCall.mayTriggerNontrivialMmuOperation;
    if (triggerMmu) {
      final MmuCall mmuCall = MmuCall.extCodeCopy(hub);
      imcFragment.callMmu(mmuCall);
    }

    final boolean foreignAccountHasCode = foreignAccount != null && foreignAccount.hasCode();
    final boolean triggerRomLex = triggerMmu && foreignAccountHasCode;

    firstForeignNew = firstForeign.deepCopy().turnOnWarmth();

    final AccountFragment accountDoingFragment =
        hub.factories()
            .accountFragment()
            .makeWithTrm(
                firstForeign,
                firstForeignNew,
                rawAddress,
                doingDomSubStamps,
                TransactionProcessingType.USER);
    accountDoingFragment.requiresRomlex(triggerRomLex);
    if (triggerRomLex) {
      hub.romLex().callRomLex(frame);
    }
    this.addFragment(accountDoingFragment);

    // an EXTCODECOPY section is only scheduled
    // for rollback if it is unexceptional
    hub.defers().scheduleForPostRollback(this, hub.currentFrame());
  }

  @Override
  public void resolveUponRollback(Hub hub, MessageFrame messageFrame, CallFrame callFrame) {

    secondForeign = firstForeignNew.deepCopy().setDeploymentNumber(hub);
    secondForeignNew = firstForeign.deepCopy().setDeploymentNumber(hub);

    final DomSubStampsSubFragment undoingDomSubStamps =
        DomSubStampsSubFragment.revertWithCurrentDomSubStamps(
            this.hubStamp(), callFrame.revertStamp(), 1);

    final AccountFragment undoingAccountFragment =
        hub.factories()
            .accountFragment()
            .make(
                secondForeign,
                secondForeignNew,
                undoingDomSubStamps,
                TransactionProcessingType.USER);

    this.addFragment(undoingAccountFragment);
  }
}
