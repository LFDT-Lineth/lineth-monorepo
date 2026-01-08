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
import static net.consensys.linea.zktracer.types.Conversions.*;
import static org.hyperledger.besu.evm.internal.Words.clampedToLong;

import java.math.BigInteger;
import lombok.EqualsAndHashCode;
import lombok.Getter;
import lombok.Setter;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.OobCall;
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
  @EqualsAndHashCode.Include short blakeF;

  // Outputs
  boolean ramSuccess;
  BigInteger returnGas;

  public Blake2fParamsOobCall(long calleeGas) {
    super();
    this.calleeGas = BigInteger.valueOf(calleeGas);
  }

  @Override
  public void setInputs(Hub hub, MessageFrame frame) {
    final OpCodeData opCode = hub.opCodeData(frame);
    final long argsOffset =
        clampedToLong(
            opCode.callHasValueArgument()
                ? hub.messageFrame().getStackItem(3)
                : hub.messageFrame().getStackItem(2));

    final Bytes callData = frame.shadowReadMemory(argsOffset, 213);
    final BigInteger blakeR = callData.slice(0, 4).toUnsignedBigInteger();

    setBlakeR(blakeR);
    setBlakeF((short) toUnsignedInt(callData.get(212)));
  }

  @Override
  public void setOutputs() {
    final boolean sufficientGas = calleeGas.compareTo(blakeR) >= 0;
    final boolean fIsABit = blakeF == 0 || blakeF == 1;
    setRamSuccess(sufficientGas && fIsABit);

    // Set returnGas
    final BigInteger returnGas = ramSuccess ? (getCalleeGas().subtract(blakeR)) : BigInteger.ZERO;
    setReturnGas(returnGas);
  }

  @Override
  public Trace.Oob traceOob(Trace.Oob trace) {
    return trace
        .inst(OOB_INST_BLAKE_PARAMS)
        .data1(bigIntegerToBytes(calleeGas))
        .data4(booleanToBytes(ramSuccess))
        .data5(bigIntegerToBytes(returnGas))
        .data6(bigIntegerToBytes(blakeR))
        .data7(Bytes.minimalBytes(blakeF))
        .fillAndValidateRow();
  }

  @Override
  public Trace.Hub traceHub(Trace.Hub trace) {
    return trace
        .pMiscOobFlag(true)
        .pMiscOobInst(OOB_INST_BLAKE_PARAMS)
        .pMiscOobData1(bigIntegerToBytes(calleeGas))
        .pMiscOobData4(booleanToBytes(ramSuccess))
        .pMiscOobData5(bigIntegerToBytes(returnGas))
        .pMiscOobData6(bigIntegerToBytes(blakeR))
        .pMiscOobData7(Bytes.minimalBytes(blakeF));
  }
}
