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
import static net.consensys.linea.zktracer.module.hub.fragment.TransientFragment.tload;

import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.defer.PostOpcodeDefer;
import net.consensys.linea.zktracer.module.hub.fragment.ContextFragment;
import net.consensys.linea.zktracer.module.hub.section.TraceSection;
import net.consensys.linea.zktracer.module.hub.signals.Exceptions;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.evm.frame.MessageFrame;
import org.hyperledger.besu.evm.operation.Operation;

public class TLoadSection extends TraceSection implements PostOpcodeDefer {

  public static final short NB_ROWS_HUB_TLOAD = 3; // stack + con + trans

  final Bytes32 storageKey;

  public TLoadSection(Hub hub) {
    super(hub, NB_ROWS_HUB_TLOAD);
    final short exceptions = hub.pch().exceptions();
    final ContextFragment readCurrentContext = ContextFragment.readCurrentContextData(hub);

    this.addStackAndFragments(hub, readCurrentContext);

    if (Exceptions.any(exceptions)) {
      checkArgument(
          Exceptions.outOfGasException(exceptions),
          "TLOAD: The only possible (non stack) exception is OOGX");
      storageKey = Bytes32.ZERO; // OOGX, no transient row
      return;
    }

    // We are now unexceptional
    storageKey = Bytes32.leftPad(hub.messageFrame().getStackItem(0));
    hub.defers().scheduleForPostExecution(this);
  }

  @Override
  public void resolvePostExecution(
      Hub hub, MessageFrame frame, Operation.OperationResult operationResult) {
    final Address address = frame.getRecipientAddress();
    final Bytes32 valueCurr = Bytes32.leftPad(frame.getStackItem(0));
    this.addFragment(tload(hubStamp(), address, storageKey, valueCurr));
  }
}
