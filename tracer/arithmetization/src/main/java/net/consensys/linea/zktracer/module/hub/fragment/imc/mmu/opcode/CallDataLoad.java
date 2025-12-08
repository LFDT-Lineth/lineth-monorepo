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

package net.consensys.linea.zktracer.module.hub.fragment.imc.mmu.opcode;

import static net.consensys.linea.zktracer.Trace.MMU_INST_RIGHT_PADDED_WORD_EXTRACTION;

import java.util.Optional;

import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.defer.PostOpcodeDefer;
import net.consensys.linea.zktracer.module.hub.fragment.imc.mmu.MmuCall;
import net.consensys.linea.zktracer.runtime.callstack.CallFrame;
import net.consensys.linea.zktracer.types.EWord;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.evm.frame.MessageFrame;
import org.hyperledger.besu.evm.operation.Operation;

public class CallDataLoad extends MmuCall implements PostOpcodeDefer {

  public CallDataLoad(final Hub hub) {
    super(hub, MMU_INST_RIGHT_PADDED_WORD_EXTRACTION);
    hub.defers().scheduleForPostExecution(this);

    final CallFrame currentFrame = hub.currentFrame();
    final long callDataSize = currentFrame.callDataRange().getRange().size();
    final long callDataOffset = currentFrame.callDataRange().getRange().offset();
    final EWord sourceOffset = EWord.of(currentFrame.frame().getStackItem(0));
    final long callDataCN = currentFrame.callDataRange().getContextNumber();
    final Bytes sourceBytes = hub.callStack().getFullMemoryOfCaller(hub);

    this.sourceId((int) callDataCN)
        .sourceRamBytes(Optional.of(sourceBytes))
        .sourceOffset(sourceOffset)
        .referenceOffset(callDataOffset)
        .referenceSize(callDataSize);
  }

  @Override
  public void resolvePostExecution(
      Hub hub, MessageFrame frame, Operation.OperationResult operationResult) {
    final EWord stack = EWord.of(frame.getStackItem(0));
    this.limb1(stack.hi());
    this.limb2(stack.lo());
  }
}
