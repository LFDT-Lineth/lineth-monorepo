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

import static net.consensys.linea.zktracer.Trace.OOB_INST_BLAKE_CDS;
import static net.consensys.linea.zktracer.Trace.Oob.CT_MAX_BLAKE2F_CDS;
import static net.consensys.linea.zktracer.module.oob.OobExoCall.callToEQ;
import static net.consensys.linea.zktracer.module.oob.OobExoCall.callToIsZero;
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
import org.hyperledger.besu.evm.frame.MessageFrame;

@Getter
@Setter
@EqualsAndHashCode(onlyExplicitlyIncluded = true, callSuper = false)
public class Blake2fCallDataSizeOobCall extends OobCall {
  // Inputs
  @EqualsAndHashCode.Include EWord cds;
  @EqualsAndHashCode.Include EWord returnAtCapacity;

  // Outputs
  boolean hubSuccess;
  boolean returnAtCapacityNonZero;

  public Blake2fCallDataSizeOobCall() {
    super();
  }

  @Override
  public void setInputData(MessageFrame frame, Hub hub) {
    final OpCodeData opCode = hub.opCodeData(frame);
    final EWord cds = EWord.of(frame.getStackItem(opCode.callCdsStackIndex()));
    final EWord returnAtCapacity =
        EWord.of(frame.getStackItem(opCode.callReturnAtCapacityStackIndex()));
    setCds(cds);
    setReturnAtCapacity(returnAtCapacity);
  }

  @Override
  public void callExoModulesAndSetOutputs(Add add, Mod mod, Wcp wcp) {
    // row i
    final OobExoCall validCdsCall = callToEQ(wcp, cds, Bytes.of(213));
    exoCalls.add(validCdsCall);
    setHubSuccess(bytesToBoolean(validCdsCall.result()));

    // row i + 1
    final OobExoCall racIsZeroCall = callToIsZero(wcp, returnAtCapacity);
    exoCalls.add(racIsZeroCall);
    setReturnAtCapacityNonZero(!bytesToBoolean(racIsZeroCall.result()));
  }

  @Override
  public int ctMax() {
    return CT_MAX_BLAKE2F_CDS;
  }

  @Override
  public Trace.Oob trace(Trace.Oob trace) {
    return trace
        .isBlake2FCds(true)
        .oobInst(OOB_INST_BLAKE_CDS)
        .data2(cds.trimLeadingZeros())
        .data3(returnAtCapacity.trimLeadingZeros())
        .data4(booleanToBytes(hubSuccess)) // Set after the constructor
        .data8(booleanToBytes(returnAtCapacityNonZero)); // Set after the constructor
  }

  @Override
  public Trace.Hub trace(Trace.Hub trace) {
    return trace
        .pMiscOobFlag(true)
        .pMiscOobInst(OOB_INST_BLAKE_CDS)
        .pMiscOobData2(cds.trimLeadingZeros())
        .pMiscOobData3(returnAtCapacity.trimLeadingZeros())
        .pMiscOobData4(booleanToBytes(hubSuccess)) // Set after the constructor
        .pMiscOobData8(booleanToBytes(returnAtCapacityNonZero)); // Set after the constructor
  }
}
