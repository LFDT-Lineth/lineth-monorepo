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

import static net.consensys.linea.zktracer.Trace.OOB_INST_JUMP;
import static net.consensys.linea.zktracer.types.Conversions.*;

import lombok.EqualsAndHashCode;
import lombok.Getter;
import lombok.Setter;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.OobCall;
import net.consensys.linea.zktracer.types.EWord;
import org.hyperledger.besu.evm.frame.MessageFrame;

@Getter
@Setter
@EqualsAndHashCode(onlyExplicitlyIncluded = true, callSuper = false)
public class JumpOobCall extends OobCall {

  // Inputs
  @EqualsAndHashCode.Include EWord pcNew;
  @EqualsAndHashCode.Include EWord codeSize;

  // Outputs
  boolean jumpGuaranteedException;
  boolean jumpMustBeAttempted;

  public JumpOobCall() {
    super();
  }

  @Override
  public void setInputs(Hub hub, MessageFrame frame) {
    setPcNew(EWord.of(frame.getStackItem(0)));
    setCodeSize(EWord.of(frame.getCode().getSize()));
  }

  @Override
  public void setOutputs() {
    final boolean validPcNew = pcNew.compareTo(codeSize) < 0;

    setJumpGuaranteedException(!validPcNew);
    setJumpMustBeAttempted(validPcNew);
  }

  @Override
  public Trace.Oob traceOob(Trace.Oob trace) {
    return trace
        .inst(OOB_INST_JUMP)
        .data1(pcNew.hi())
        .data2(pcNew.lo())
        .data5(codeSize)
        .data7(booleanToBytes(jumpGuaranteedException))
        .data8(booleanToBytes(jumpMustBeAttempted))
        .fillAndValidateRow();
  }

  @Override
  public Trace.Hub traceHub(Trace.Hub trace) {
    return trace
        .pMiscOobFlag(true)
        .pMiscOobInst(OOB_INST_JUMP)
        .pMiscOobData1(pcNew.hi())
        .pMiscOobData2(pcNew.lo())
        .pMiscOobData5(codeSize)
        .pMiscOobData7(booleanToBytes(jumpGuaranteedException))
        .pMiscOobData8(booleanToBytes(jumpMustBeAttempted));
  }
}
