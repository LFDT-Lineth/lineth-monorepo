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

package net.consensys.linea.zktracer.module.hub.fragment.imc;

import static com.google.common.base.Preconditions.checkState;

import java.util.ArrayList;
import java.util.List;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.defer.ContextEntryDefer;
import net.consensys.linea.zktracer.module.hub.defer.ContextReEntryDefer;
import net.consensys.linea.zktracer.module.hub.fragment.TraceFragment;
import net.consensys.linea.zktracer.module.hub.fragment.TraceSubFragment;
import net.consensys.linea.zktracer.module.hub.fragment.imc.exp.ExpCall;
import net.consensys.linea.zktracer.module.hub.fragment.imc.mmu.MmuCall;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.OobCall;
import net.consensys.linea.zktracer.runtime.callstack.CallFrame;
import net.consensys.linea.zktracer.types.TransactionProcessingMetadata;
import org.hyperledger.besu.evm.frame.MessageFrame;

/**
 * IMCFragments embed data required for Inter-Module Communication, i.e. data that are required to
 * correctly trigger other modules from the Hub.
 */
public class ImcFragment implements TraceFragment, ContextReEntryDefer, ContextEntryDefer {
  /** the list of modules to trigger withing this fragment. */
  private final List<TraceSubFragment> moduleCalls = new ArrayList<>(5);

  private final Hub hub;

  private boolean expIsSet = false;
  private boolean oobIsSet = false;
  private boolean mxpIsSet = false;
  private boolean mmuIsSet = false;
  private boolean stpIsSet = false;

  private CallFrame childFrame = null;

  private ImcFragment(final Hub hub) {
    this.hub = hub;
  }

  /**
   * Create an empty ImcFragment to be filled with specialized methods.
   *
   * @return an empty ImcFragment
   */
  public static ImcFragment empty(final Hub hub) {
    return new ImcFragment(hub);
  }

  /**
   * Create an ImcFragment to be used in the transaction initialization phase.
   *
   * @param hub the execution context
   * @return the ImcFragment for the TxInit phase
   */
  public static ImcFragment forTxInit(final Hub hub) {
    // isdeployment == false
    // non empty calldata
    final TransactionProcessingMetadata currentTx = hub.txStack().current();
    final boolean shouldCopyTxCallData = currentTx.copyTransactionCallData();

    final ImcFragment miscFragment = ImcFragment.empty(hub);

    return shouldCopyTxCallData ? miscFragment.callMmu(MmuCall.txInit(hub)) : miscFragment;
  }

  public OobCall callOob(OobCall f) {
    checkState(!oobIsSet, "OOB already called");
    oobIsSet = true;
    final OobCall oobCall = hub.oob().call(f, hub);
    moduleCalls.add(oobCall);
    return oobCall;
  }

  public ImcFragment callMmu(MmuCall f) {
    checkState(!mmuIsSet, "MMU already called");
    mmuIsSet = true;
    // Note: the triggering of the MMU is made by the creation of the MmuCall
    moduleCalls.add(f);
    return this;
  }

  public ImcFragment callExp(ExpCall f) {
    checkState(!expIsSet, "EXP already called");
    expIsSet = true;
    hub.exp().call(f);
    moduleCalls.add(f);
    return this;
  }

  public ImcFragment callMxp(MxpCall f) {
    checkState(!mxpIsSet, "MXP already called");
    mxpIsSet = true;
    hub.mxp().call(f);
    moduleCalls.add(f);
    return this;
  }

  public void callStp(StpCall f) {
    checkState(!stpIsSet, "STP already called");
    stpIsSet = true;
    hub.stp().call(f);
    moduleCalls.add(f);
  }

  @Override
  public Trace.Hub trace(Trace.Hub trace) {
    trace.peekAtMiscellaneous(true);

    for (TraceSubFragment subFragment : moduleCalls) {
      if (subFragment instanceof MmuCall) {
        final MmuCall mmuCall = (MmuCall) subFragment;
        if (mmuCall.traceMe()) {
          subFragment.traceHub(trace, hub.state);
        }
      } else {
        subFragment.traceHub(trace, hub.state);
      }
    }

    if (childFrame != null) {
      trace.pMiscCcrsStamp(childFrame.revertStamp()).pMiscCcsrFlag(childFrame.selfReverts());
    }

    return trace;
  }

  @Override
  public void resolveAtContextReEntry(Hub hub, CallFrame frame) {
    childFrame = hub.callStack().getById(frame.childFrameIds().getLast());
  }

  @Override
  public void resolveUponContextEntry(Hub hub, MessageFrame frame) {
    childFrame = hub.currentFrame();
  }
}
