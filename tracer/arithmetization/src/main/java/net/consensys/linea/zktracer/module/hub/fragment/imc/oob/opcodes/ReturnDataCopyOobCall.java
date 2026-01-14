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

import static net.consensys.linea.zktracer.Trace.OOB_INST_RDC;
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
public class ReturnDataCopyOobCall extends OobCall {

  // Inputs
  @EqualsAndHashCode.Include EWord offset;
  @EqualsAndHashCode.Include EWord size;
  @EqualsAndHashCode.Include EWord rds;

  // Outputs
  boolean rdcx;

  public ReturnDataCopyOobCall() {
    super();
  }

  @Override
  public void setInputs(Hub hub, MessageFrame frame) {
    setOffset(EWord.of(frame.getStackItem(1)));
    setSize(EWord.of(frame.getStackItem(2)));
    setRds(EWord.of(frame.getReturnData().size()));
  }

  @Override
  public void setOutputs() {
    final EWord sum = offset.add(size);
    // check whether rdc is "ridiculously out-of-bounds (ROOB)".
    final boolean rdcRoob = !offset.hi().isZero() || !size.hi().isZero();
    // check whether rdc "sum is out-of-bounds (SOOB)"
    final boolean rdcSoob = sum.compareTo(rds) > 0;
    setRdcx(rdcRoob || rdcSoob);
  }

  @Override
  public Trace.Oob traceOob(Trace.Oob trace) {
    return trace
        .inst(OOB_INST_RDC)
        .data1(offset.hi())
        .data2(offset.lo())
        .data3(size.hi())
        .data4(size.lo())
        .data5(rds)
        .data7(booleanToBytes(rdcx))
        .fillAndValidateRow();
  }

  @Override
  public Trace.Hub traceHub(Trace.Hub trace) {
    return trace
        .pMiscOobFlag(true)
        .pMiscOobInst(OOB_INST_RDC)
        .pMiscOobData1(offset.hi())
        .pMiscOobData2(offset.lo())
        .pMiscOobData3(size.hi())
        .pMiscOobData4(size.lo())
        .pMiscOobData5(rds)
        .pMiscOobData7(booleanToBytes(rdcx));
  }
}
