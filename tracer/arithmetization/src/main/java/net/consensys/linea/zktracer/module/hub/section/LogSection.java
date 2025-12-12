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

import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.defer.EndTransactionDefer;
import net.consensys.linea.zktracer.module.hub.fragment.ContextFragment;
import net.consensys.linea.zktracer.module.hub.fragment.imc.ImcFragment;
import net.consensys.linea.zktracer.module.hub.fragment.imc.MxpCall;
import net.consensys.linea.zktracer.module.hub.fragment.imc.mmu.MmuCall;
import net.consensys.linea.zktracer.module.hub.signals.Exceptions;
import net.consensys.linea.zktracer.runtime.LogData;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.evm.worldstate.WorldView;

public class LogSection extends TraceSection implements EndTransactionDefer {

  private MmuCall mmuCall;

  public LogSection(Hub hub) {
    super(hub, maxNumberOfRows(hub));

    final short exceptions = hub.pch().exceptions();

    final ContextFragment currentContextFragment = ContextFragment.readCurrentContextData(hub);

    this.addStack(hub);
    this.addFragment(currentContextFragment);

    // Static Case
    if (hub.currentFrame().frame().isStatic()) {
      checkArgument(Exceptions.staticFault(exceptions), "STATICX not detected");
      return;
    }

    final ImcFragment imcFragment = ImcFragment.empty(hub);
    this.addFragment(imcFragment);

    final MxpCall mxpCall = MxpCall.newMxpCall(hub);
    imcFragment.callMxp(mxpCall);

    checkArgument(
        mxpCall.isMxpx() == Exceptions.memoryExpansionException(exceptions),
        "MXPX mismatch with MXP call");

    // MXPX case
    if (mxpCall.isMxpx()) {
      return;
    }

    // OOGX case
    if (Exceptions.outOfGasException(exceptions)) {
      return;
    }

    // the unexceptional case
    checkArgument(Exceptions.none(exceptions), "Unexpected exception code " + exceptions);

    hub.defers().scheduleForEndTransaction(this);

    final LogData logData = new LogData(hub);
    checkArgument(
        logData.nontrivialLog() == mxpCall.mayTriggerNontrivialMmuOperation,
        "non trivial LOG / MMU mismatch");
    mmuCall = (logData.nontrivialLog()) ? MmuCall.LogX(hub, logData) : null;

    if (mmuCall != null) {
      imcFragment.callMmu(mmuCall);
    }
  }

  private static short maxNumberOfRows(Hub hub) {
    return (short)
        (hub.opCodeData().numberOfStackRows()
            + (Exceptions.staticFault(hub.pch().exceptions()) ? 2 : 3));
  }

  @Override
  public void resolveAtEndTransaction(
      Hub hub, WorldView state, Transaction tx, boolean isSuccessful) {
    final boolean logReverted = commonValues.callFrame().willRevert();
    if (logReverted) {
      if (mmuCall != null) {
        mmuCall.dontTraceMe();
      }
    } else {
      final int logStamp = hub.state.stamps().incrementLogStamp();
      commonValues.logStamp(logStamp);
      if (mmuCall != null) {
        mmuCall.targetId(logStamp);
      }
    }
  }
}
