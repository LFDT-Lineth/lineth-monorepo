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
import static net.consensys.linea.zktracer.Trace.Oob.CT_MAX_CALL;
import static net.consensys.linea.zktracer.module.oob.OobExoCall.callToIsZero;
import static net.consensys.linea.zktracer.module.oob.OobExoCall.callToLT;
import static net.consensys.linea.zktracer.types.Conversions.*;

import lombok.EqualsAndHashCode;
import lombok.Getter;
import lombok.Setter;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.add.Add;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.OobCall;
import net.consensys.linea.zktracer.module.mod.Mod;
import net.consensys.linea.zktracer.module.oob.OobExoCall;
import net.consensys.linea.zktracer.module.wcp.Wcp;
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
  @EqualsAndHashCode.Include Bytes balance;
  @EqualsAndHashCode.Include Bytes callStackDepth;

  // Outputs
  boolean abortingCondition;

  public CallOobCall() {
    super();
  }

  @Override
  public void setInputData(MessageFrame frame, Hub hub) {
    final Account callerAccount = frame.getWorldUpdater().get(frame.getRecipientAddress());
    final OpCodeData opCode = hub.opCodeData(frame);

    // DELEGATECALL, STATICCALL can't trasfer value,
    // CALL, CALLCODE may transfer value
    final EWord value =
        opCode.callHasValueArgument() ? EWord.of(frame.getStackItem(2)) : EWord.ZERO;
    setValue(value);
    setBalance(bigIntegerToBytes(callerAccount.getBalance().toUnsignedBigInteger()));
    setCallStackDepth(Bytes.ofUnsignedInt(frame.getDepth()));
  }

  @Override
  public void callExoModulesAndSetOutputs(Add add, Mod mod, Wcp wcp) {
    // row i
    final OobExoCall insufficientBalanceAbortCall = callToLT(wcp, balance, value);
    exoCalls.add(insufficientBalanceAbortCall);
    final boolean insufficientBalanceAbort = bytesToBoolean(insufficientBalanceAbortCall.result());

    // row i + 1
    final OobExoCall callStackDepthAbortCall =
        callToLT(wcp, callStackDepth, MAX_CALL_STACK_DEPTH_BYTES);
    exoCalls.add(callStackDepthAbortCall);
    final boolean callStackDepthAbort = !bytesToBoolean(callStackDepthAbortCall.result());

    // row i + 2
    exoCalls.add(callToIsZero(wcp, value));

    setAbortingCondition(insufficientBalanceAbort || callStackDepthAbort);
  }

  @Override
  public int ctMax() {
    return CT_MAX_CALL;
  }

  @Override
  public Trace.Oob trace(Trace.Oob trace) {
    return trace
        .isCall(true)
        .oobInst(OOB_INST_CALL)
        .data1(value.hi())
        .data2(value.lo())
        .data3(balance)
        .data6(callStackDepth)
        .data7(booleanToBytes(!value.isZero()))
        .data8(booleanToBytes(abortingCondition));
  }

  @Override
  public Trace.Hub trace(Trace.Hub trace) {
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
