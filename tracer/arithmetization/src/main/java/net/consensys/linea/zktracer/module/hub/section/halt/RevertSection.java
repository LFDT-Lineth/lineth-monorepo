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
package net.consensys.linea.zktracer.module.hub.section.halt;

import static com.google.common.base.Preconditions.*;

import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.fragment.ContextFragment;
import net.consensys.linea.zktracer.module.hub.fragment.imc.ImcFragment;
import net.consensys.linea.zktracer.module.hub.fragment.imc.MxpCall;
import net.consensys.linea.zktracer.module.hub.fragment.imc.mmu.MmuCall;
import net.consensys.linea.zktracer.module.hub.section.TraceSection;
import net.consensys.linea.zktracer.module.hub.signals.Exceptions;
import net.consensys.linea.zktracer.runtime.callstack.CallFrame;
import net.consensys.linea.zktracer.types.MemoryRange;
import net.consensys.linea.zktracer.types.Range;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.evm.frame.MessageFrame;

public class RevertSection extends TraceSection {

  public static final short NB_ROWS_HUB_REVERT = 4; // 4 = 1 + 3

  final ImcFragment imcFragment;
  MmuCall mmuCall;

  public RevertSection(Hub hub, MessageFrame frame) {
    super(hub, NB_ROWS_HUB_REVERT);

    short exceptions = hub.pch().exceptions();

    imcFragment = ImcFragment.empty(hub);
    this.addStackAndFragments(hub, imcFragment);

    // triggerExp = false
    // triggerOob = false
    // triggerStp = false
    // triggerMxp = true
    final MxpCall mxpCall = MxpCall.newMxpCall(hub);
    imcFragment.callMxp(mxpCall);
    checkArgument(
        mxpCall.mxpx == Exceptions.memoryExpansionException(exceptions),
        "REVERT: mxp and hub disagree on MXPX");

    if (Exceptions.memoryExpansionException(exceptions)) {
      return;
    }

    if (Exceptions.outOfGasException(exceptions)) {
      return;
    }

    // The XAHOY = 0 case
    checkArgument(Exceptions.none(exceptions), "REVERT: unexpected exception %s", exceptions);

    final CallFrame callFrame = hub.currentFrame();
    final Bytes offset = frame.getStackItem(0);
    final Bytes size = frame.getStackItem(1);
    callFrame.outputDataRange(
        new MemoryRange(callFrame.contextNumber(), Range.fromOffsetAndSize(offset, size), frame));

    final boolean triggerMmu =
        (Exceptions.none(exceptions))
            && !hub.currentFrame().isRoot()
            && mxpCall.mayTriggerNontrivialMmuOperation // i.e. size ≠ 0 ∧ ¬MXPX
            && !hub.currentFrame().returnAtRange().isEmpty();

    if (triggerMmu) {
      mmuCall = MmuCall.revert(hub);
      imcFragment.callMmu(mmuCall);
    }

    final ContextFragment currentContext = ContextFragment.readCurrentContextData(hub);
    final ContextFragment updateCallerReturnData = ContextFragment.executionProvidesReturnData(hub);

    this.addFragments(currentContext, updateCallerReturnData);
  }
}
