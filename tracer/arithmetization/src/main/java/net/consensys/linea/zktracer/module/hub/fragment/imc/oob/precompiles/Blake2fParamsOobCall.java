/*
 * Copyright ConsenSys AG.
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

package net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles;

import static java.lang.Byte.toUnsignedInt;
import static net.consensys.linea.zktracer.Trace.OOB_INST_BLAKE_PARAMS;
import static net.consensys.linea.zktracer.Trace.Oob.CT_MAX_BLAKE2F_PARAMS;
import static net.consensys.linea.zktracer.module.oob.OobExoCall.callToEQ;
import static net.consensys.linea.zktracer.module.oob.OobExoCall.callToLT;
import static net.consensys.linea.zktracer.types.Conversions.*;
import static org.hyperledger.besu.evm.internal.Words.clampedToLong;

import java.math.BigInteger;

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
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.evm.frame.MessageFrame;

@Getter
@Setter
@EqualsAndHashCode(onlyExplicitlyIncluded = true, callSuper = false)
public class Blake2fParamsOobCall extends OobCall {

  // Inputs
  @EqualsAndHashCode.Include final BigInteger calleeGas;
  @EqualsAndHashCode.Include BigInteger blakeR;
  @EqualsAndHashCode.Include BigInteger blakeF;

  // Outputs
  boolean ramSuccess;
  BigInteger returnGas;

  public Blake2fParamsOobCall(long calleeGas) {
    super();
    this.calleeGas = BigInteger.valueOf(calleeGas);
  }

  @Override
  public void setInputData(MessageFrame frame, Hub hub) {
    final OpCodeData opCode = hub.opCodeData(frame);
    final long argsOffset =
        clampedToLong(
            opCode.callHasValueArgument()
                ? hub.messageFrame().getStackItem(3)
                : hub.messageFrame().getStackItem(2));

    final Bytes callData = frame.shadowReadMemory(argsOffset, 213);
    final BigInteger blakeR = callData.slice(0, 4).toUnsignedBigInteger();
    final BigInteger blakeF = BigInteger.valueOf(toUnsignedInt(callData.get(212)));

    setBlakeR(blakeR);
    setBlakeF(blakeF);
  }

  @Override
  public void callExoModulesAndSetOutputs(Add add, Mod mod, Wcp wcp) {
    // row i
    final OobExoCall sufficientGasCall =
        callToLT(wcp, bigIntegerToBytes(calleeGas), bigIntegerToBytes(blakeR));
    exoCalls.add(sufficientGasCall);

    // row i + 1
    final OobExoCall fIsABitCall =
        callToEQ(wcp, bigIntegerToBytes(blakeF), bigIntegerToBytes(blakeF.multiply(blakeF)));
    exoCalls.add(fIsABitCall);

    // Set ramSuccess
    final boolean ramSuccess =
        !bytesToBoolean(sufficientGasCall.result()) && bytesToBoolean(fIsABitCall.result());
    setRamSuccess(ramSuccess);

    // Set returnGas
    final BigInteger returnGas = ramSuccess ? (getCalleeGas().subtract(blakeR)) : BigInteger.ZERO;
    setReturnGas(returnGas);
  }

  @Override
  public int ctMax() {
    return CT_MAX_BLAKE2F_PARAMS;
  }

  @Override
  public Trace.Oob trace(Trace.Oob trace) {
    return trace
        .isBlake2FParams(true)
        .oobInst(OOB_INST_BLAKE_PARAMS)
        .data1(bigIntegerToBytes(calleeGas))
        .data4(booleanToBytes(ramSuccess)) // Set after the constructor
        .data5(bigIntegerToBytes(returnGas)) // Set after the constructor
        .data6(bigIntegerToBytes(blakeR))
        .data7(bigIntegerToBytes(blakeF));
  }

  @Override
  public Trace.Hub trace(Trace.Hub trace) {
    return trace
        .pMiscOobFlag(true)
        .pMiscOobInst(OOB_INST_BLAKE_PARAMS)
        .pMiscOobData1(bigIntegerToBytes(calleeGas))
        .pMiscOobData4(booleanToBytes(ramSuccess)) // Set after the constructor
        .pMiscOobData5(bigIntegerToBytes(returnGas)) // Set after the constructor
        .pMiscOobData6(bigIntegerToBytes(blakeR))
        .pMiscOobData7(bigIntegerToBytes(blakeF));
  }
}
