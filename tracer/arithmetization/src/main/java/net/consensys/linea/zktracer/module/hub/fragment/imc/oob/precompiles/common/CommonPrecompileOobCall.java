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

package net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.common;

import static net.consensys.linea.zktracer.module.oob.OobExoCall.callToIsZero;
import static net.consensys.linea.zktracer.types.Conversions.*;

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
import net.consensys.linea.zktracer.types.EWord;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.evm.frame.MessageFrame;

@Getter
@Setter
@EqualsAndHashCode(onlyExplicitlyIncluded = true, callSuper = false)
public abstract class CommonPrecompileOobCall extends OobCall {
  // type of precompile
  @EqualsAndHashCode.Include final int oobInst;

  // Inputs
  @EqualsAndHashCode.Include final Bytes calleeGas;
  @EqualsAndHashCode.Include EWord cds;
  @EqualsAndHashCode.Include EWord returnAtCapacity;

  // Outputs
  boolean hubSuccess;
  BigInteger returnGas;
  boolean returnAtCapacityNonZero;
  boolean cdsIsZero; // Necessary to compute extractCallData and emptyCallData

  protected CommonPrecompileOobCall(final BigInteger calleeGas, final int oobInst) {
    this.calleeGas = bigIntegerToBytes(calleeGas);
    this.oobInst = oobInst;
  }

  @Override
  public void setInputData(MessageFrame frame, Hub hub) {
    final OpCodeData opCode = hub.opCodeData(frame);

    final EWord callDataSize = EWord.of(frame.getStackItem(opCode.callCdsStackIndex()));
    final EWord returnAtCapacity =
        EWord.of(frame.getStackItem(opCode.callReturnAtCapacityStackIndex()));

    setCds(callDataSize);
    setReturnAtCapacity(returnAtCapacity);
  }

  @Override
  public void callExoModulesAndSetOutputs(Add add, Mod mod, Wcp wcp) {
    // row i
    final OobExoCall cdsIsZeroCall = callToIsZero(wcp, cds);
    exoCalls.add(cdsIsZeroCall);
    final boolean cdsIsZero = bytesToBoolean(cdsIsZeroCall.result());

    // row i + 1
    final OobExoCall returnAtCapacityCall = callToIsZero(wcp, returnAtCapacity);
    exoCalls.add(returnAtCapacityCall);
    final boolean returnAtCapacityIsZero = bytesToBoolean(returnAtCapacityCall.result());

    setCdsIsZero(cdsIsZero);
    setReturnAtCapacityNonZero(!returnAtCapacityIsZero);
  }

  public boolean getExtractCallData() {
    return hubSuccess && getCdxFilter() && !cdsIsZero;
  }

  public boolean getCallDataIsEmpty() {
    return hubSuccess && getCdxFilter() && cdsIsZero;
  }

  public boolean getCdxFilter() {
    // This is override only in the case of P256_VERIFY
    return true;
  }

  @Override
  public Trace.Oob trace(Trace.Oob trace) {
    traceOobInstructionInOob(trace);
    return trace
        .data1(calleeGas)
        .data2(cds.trimLeadingZeros())
        .data3(returnAtCapacity.trimLeadingZeros())
        .data4(booleanToBytes(hubSuccess)) // Set after the constructor
        .data5(bigIntegerToBytes(returnGas)) // Set after the constructor
        .data6(booleanToBytes(getExtractCallData())) // Derived from other parameters
        .data7(booleanToBytes(getCallDataIsEmpty())) // Derived from other parameters
        .data8(booleanToBytes(returnAtCapacityNonZero)); // Set after the constructor
  }

  protected abstract void traceOobInstructionInOob(Trace.Oob trace);

  @Override
  public Trace.Hub trace(Trace.Hub trace) {
    traceOobInstructionInHub(trace);
    return trace
        .pMiscOobFlag(true)
        .pMiscOobData1(calleeGas)
        .pMiscOobData2(cds.trimLeadingZeros())
        .pMiscOobData3(returnAtCapacity.trimLeadingZeros())
        .pMiscOobData4(booleanToBytes(hubSuccess)) // Set after the constructor
        .pMiscOobData5(bigIntegerToBytes(returnGas)) // Set after the constructor
        .pMiscOobData6(booleanToBytes(getExtractCallData())) // Derived from other parameters
        .pMiscOobData7(booleanToBytes(getCallDataIsEmpty())) // Derived from other parameters
        .pMiscOobData8(booleanToBytes(returnAtCapacityNonZero)); // Set after the constructor
  }

  protected abstract void traceOobInstructionInHub(Trace.Hub trace);
}
