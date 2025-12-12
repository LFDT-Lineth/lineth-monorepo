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

import static net.consensys.linea.zktracer.module.shakiradata.HashFunction.KECCAK;

import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.defer.PostOpcodeDefer;
import net.consensys.linea.zktracer.module.hub.fragment.imc.ImcFragment;
import net.consensys.linea.zktracer.module.hub.fragment.imc.MxpCall;
import net.consensys.linea.zktracer.module.hub.fragment.imc.mmu.MmuCall;
import net.consensys.linea.zktracer.module.hub.signals.Exceptions;
import net.consensys.linea.zktracer.module.shakiradata.ShakiraDataOperation;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.evm.frame.MessageFrame;
import org.hyperledger.besu.evm.internal.Words;
import org.hyperledger.besu.evm.operation.Operation;

public class KeccakSection extends TraceSection implements PostOpcodeDefer {

  private final boolean triggerMmu;
  private Bytes hashInput;

  public KeccakSection(Hub hub) {
    super(hub, (short) 3);

    final ImcFragment imcFragment = ImcFragment.empty(hub);
    this.addStackAndFragments(hub, imcFragment);

    final MxpCall mxpCall = MxpCall.newMxpCall(hub);

    imcFragment.callMxp(mxpCall);

    final boolean mayTriggerNonTrivialOperation = mxpCall.mayTriggerNontrivialMmuOperation;
    triggerMmu = mayTriggerNonTrivialOperation & Exceptions.none(hub.pch().exceptions());

    if (triggerMmu) {
      final long offset = Words.clampedToLong(hub.messageFrame().getStackItem(0));
      final long size = Words.clampedToLong(hub.messageFrame().getStackItem(1));
      hashInput = hub.currentFrame().frame().shadowReadMemory(offset, size);
      final MmuCall mmuCall = MmuCall.sha3(hub, hashInput);
      imcFragment.callMmu(mmuCall);
    }

    if (Exceptions.none(hub.pch().exceptions())) {
      hub.defers().scheduleForPostExecution(this);
    }
  }

  @Override
  public void resolvePostExecution(
      Hub hub, MessageFrame frame, Operation.OperationResult operationResult) {

    final Bytes32 hashResult = Bytes32.leftPad(frame.getStackItem(0));

    // retroactively set HASH_INFO_FLAG and HASH_INFO_KECCAK_HI, HASH_INFO_KECCAK_LO
    this.writeHashInfoResult(hashResult);

    if (triggerMmu) {
      final ShakiraDataOperation shakiraDataOperation =
          new ShakiraDataOperation(this.hubStamp(), KECCAK, hashInput, hashResult);
      hub.shakiraData().call(shakiraDataOperation);
    }
  }
}
