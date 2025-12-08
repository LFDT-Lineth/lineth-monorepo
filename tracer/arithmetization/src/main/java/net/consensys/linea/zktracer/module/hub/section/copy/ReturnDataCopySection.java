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
import static net.consensys.linea.zktracer.module.hub.signals.Exceptions.OUT_OF_GAS_EXCEPTION;

import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.fragment.ContextFragment;
import net.consensys.linea.zktracer.module.hub.fragment.imc.ImcFragment;
import net.consensys.linea.zktracer.module.hub.fragment.imc.MxpCall;
import net.consensys.linea.zktracer.module.hub.fragment.imc.mmu.MmuCall;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.opcodes.ReturnDataCopyOobCall;
import net.consensys.linea.zktracer.module.hub.section.TraceSection;
import net.consensys.linea.zktracer.module.hub.signals.Exceptions;

public class ReturnDataCopySection extends TraceSection {

  public static final short NB_ROWS_HUB_RETURN_DATA_COPY = 4; // 4 = 1 + 3

  public ReturnDataCopySection(Hub hub) {
    super(hub, NB_ROWS_HUB_RETURN_DATA_COPY);

    final ContextFragment currentContext = ContextFragment.readCurrentContextData(hub);
    final ImcFragment imcFragment = ImcFragment.empty(hub);
    final ReturnDataCopyOobCall oobCall =
        (ReturnDataCopyOobCall) imcFragment.callOob(new ReturnDataCopyOobCall());

    this.addStack(hub);
    this.addFragment(imcFragment);
    this.addFragment(currentContext);

    final short exceptions = hub.pch().exceptions();
    final boolean returnDataCopyException = oobCall.isRdcx();
    checkArgument(
        returnDataCopyException == Exceptions.returnDataCopyFault(exceptions),
        "RETURN_DATA_COPY: oob and hub disagree on RDCX");

    // returnDataCopyException case
    if (returnDataCopyException) {
      return;
    }

    final MxpCall mxpCall = MxpCall.newMxpCall(hub);
    imcFragment.callMxp(mxpCall);

    checkArgument(
        mxpCall.mxpx == Exceptions.memoryExpansionException(exceptions),
        "RETURN_DATA_COPY: mxp and hub disagree on MXPX");

    // memoryExpansionException case
    if (mxpCall.mxpx) {
      return;
    }

    // outOfGasException case
    if (Exceptions.any(exceptions)) {
      checkArgument(
          exceptions == OUT_OF_GAS_EXCEPTION,
          "RETURN_DATA_COPY: unexpected exception, %s does not match %s",
          exceptions,
          OUT_OF_GAS_EXCEPTION);
      return;
    }

    // beyond this point unexceptional
    final boolean triggerMmu = mxpCall.mayTriggerNontrivialMmuOperation;
    if (triggerMmu) {
      final MmuCall mmuCall = MmuCall.returnDataCopy(hub);
      imcFragment.callMmu(mmuCall);
    }
  }
}
