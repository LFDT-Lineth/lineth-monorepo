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

package net.consensys.linea.zktracer.module.hub.section.transients;

import static com.google.common.base.Preconditions.checkArgument;
import static net.consensys.linea.zktracer.module.hub.fragment.TransientFragment.tstoreDoing;
import static net.consensys.linea.zktracer.module.hub.fragment.TransientFragment.tstoreUndoing;

import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.defer.PostRollbackDefer;
import net.consensys.linea.zktracer.module.hub.fragment.ContextFragment;
import net.consensys.linea.zktracer.module.hub.fragment.TransientFragment;
import net.consensys.linea.zktracer.module.hub.section.TraceSection;
import net.consensys.linea.zktracer.module.hub.signals.Exceptions;
import net.consensys.linea.zktracer.runtime.callstack.CallFrame;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.evm.frame.MessageFrame;

public class TStoreSection extends TraceSection implements PostRollbackDefer {

  public static final short NB_ROWS_HUB_TSTORE = 4; // stack + con + 2 STO

  TransientFragment tstoreDoing;

  public TStoreSection(Hub hub) {
    super(hub, NB_ROWS_HUB_TSTORE);

    final short exceptions = hub.pch().exceptions();
    final ContextFragment readCurrentContext = ContextFragment.readCurrentContextData(hub);

    this.addStackAndFragments(hub, readCurrentContext);

    if (Exceptions.any(exceptions)) {
      checkArgument(
          Exceptions.staticFault(exceptions) || Exceptions.outOfGasException(exceptions),
          "TSTORE: may only throw STATICX and OOGX exceptions");
      return;
    }

    // We are now unexceptional
    final CallFrame currentFrame = hub.currentFrame();
    hub.defers().scheduleForPostRollback(this, currentFrame);

    final Address address = currentFrame.frame().getRecipientAddress();
    final Bytes32 storageKey = Bytes32.leftPad(currentFrame.frame().getStackItem(0));
    final Bytes32 valueCurr =
        Bytes32.leftPad(currentFrame.frame().getTransientStorageValue(address, storageKey));
    final Bytes32 valueNext = Bytes32.leftPad(currentFrame.frame().getStackItem(1));

    tstoreDoing = tstoreDoing(hubStamp(), address, storageKey, valueCurr, valueNext);
    this.addFragment(tstoreDoing);
  }

  @Override
  public void resolveUponRollback(Hub hub, MessageFrame messageFrame, CallFrame callFrame) {
    this.addFragment(tstoreUndoing(hubStamp(), revertStamp(), tstoreDoing));
  }
}
