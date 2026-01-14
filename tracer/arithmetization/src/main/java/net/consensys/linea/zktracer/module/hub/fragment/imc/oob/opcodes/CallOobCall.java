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

package net.consensys.linea.zktracer.module.hub.fragment.imc.oob.opcodes;

import static net.consensys.linea.zktracer.Trace.OOB_INST_CALL;
import static net.consensys.linea.zktracer.types.Conversions.*;

import lombok.EqualsAndHashCode;
import lombok.Getter;
import lombok.Setter;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.OobCall;
import net.consensys.linea.zktracer.opcode.OpCodeData;
import net.consensys.linea.zktracer.types.EWord;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.evm.account.Account;
import org.hyperledger.besu.evm.frame.MessageFrame;

@Getter
@Setter
@EqualsAndHashCode(onlyExplicitlyIncluded = true, callSuper = false)
public class CallOobCall extends OobCall {
  public static final Bytes MAX_CALL_STACK_DEPTH_BYTES = Bytes.ofUnsignedInt(1024);

  // Inputs
  @EqualsAndHashCode.Include public EWord value;
  @EqualsAndHashCode.Include EWord balance;
  @EqualsAndHashCode.Include EWord callStackDepth;

  // Outputs
  boolean abortingCondition;

  public CallOobCall() {
    super();
  }

  @Override
  public void setInputs(Hub hub, MessageFrame frame) {
    final OpCodeData opcode = hub.opCodeData();
    final Account callerAccount = frame.getWorldUpdater().get(frame.getRecipientAddress());

    // DELEGATECALL, STATICCALL can't trasfer value,
    // CALL, CALLCODE may transfer value
    final EWord value =
        opcode.callHasValueArgument() ? EWord.of(frame.getStackItem(2)) : EWord.ZERO;
    setValue(value);
    setBalance(EWord.of(callerAccount.getBalance().toUnsignedBigInteger()));
    setCallStackDepth(EWord.of(frame.getDepth()));
  }

  @Override
  public void setOutputs() {
    final boolean insufficientBalanceAbort = balance.compareTo(value) < 0;
    final boolean callStackDepthAbort =
        callStackDepth.compareTo(EWord.of(MAX_CALL_STACK_DEPTH_BYTES)) >= 0;
    setAbortingCondition(insufficientBalanceAbort || callStackDepthAbort);
  }

  @Override
  public Trace.Oob traceOob(Trace.Oob trace) {
    return trace
        .inst(OOB_INST_CALL)
        .data1(value.hi())
        .data2(value.lo())
        .data3(balance)
        .data6(callStackDepth)
        .data7(booleanToBytes(!value.isZero()))
        .data8(booleanToBytes(abortingCondition))
        .fillAndValidateRow();
  }

  @Override
  public Trace.Hub traceHub(Trace.Hub trace) {
    return trace
        .pMiscOobFlag(true)
        .pMiscOobInst(OOB_INST_CALL)
        .pMiscOobData1(value.hi())
        .pMiscOobData2(value.lo())
        .pMiscOobData3(balance)
        .pMiscOobData6(callStackDepth)
        .pMiscOobData7(booleanToBytes(!value.isZero()))
        .pMiscOobData8(booleanToBytes(abortingCondition));
  }
}
