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

package net.consensys.linea.zktracer.module.hub.section;

import static com.google.common.base.Preconditions.checkArgument;
import static net.consensys.linea.zktracer.module.hub.fragment.imc.mmu.MmuCall.mcopyCopy;

import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.fragment.imc.ImcFragment;
import net.consensys.linea.zktracer.module.hub.fragment.imc.MxpCall;
import net.consensys.linea.zktracer.module.hub.fragment.imc.mmu.MmuCall;
import net.consensys.linea.zktracer.module.hub.signals.Exceptions;
import net.consensys.linea.zktracer.runtime.callstack.CallFrame;

public class McopySection extends TraceSection {

  public static final short NB_ROWS_HUB_MCOPY = 3;

  public McopySection(Hub hub) {
    super(hub, NB_ROWS_HUB_MCOPY);

    final MxpCall mxpCall = MxpCall.newMxpCall(hub);
    final ImcFragment firstImcFragment = ImcFragment.empty(hub).callMxp(mxpCall);
    this.addStackAndFragments(hub, firstImcFragment);

    final short exception = hub.pch().exceptions();

    if (Exceptions.any(exception)) {
      checkArgument(
          Exceptions.outOfGasException(exception) || Exceptions.memoryExpansionException(exception),
          "Mcopy can only trigger OOG or MXPX exceptions, got: " + exception);
      return;
    }

    // We are now unexceptional
    final boolean triggerMmu = mxpCall.getSize1NonZeroNoMxpx();

    if (!triggerMmu) {
      // If size == 0, we stop here, nothing to copy
      return;
    }

    // We are now in the non-trivial case, we need to call twice the MMU
    final CallFrame callFrame = hub.currentFrame();
    final MmuCall copy = mcopyCopy(hub, callFrame);
    firstImcFragment.callMmu(copy);

    final MmuCall paste = MmuCall.mcopyPaste(hub, callFrame);
    final ImcFragment secondImcFragment = ImcFragment.empty(hub).callMmu(paste);
    this.addFragment(secondImcFragment);
  }
}
