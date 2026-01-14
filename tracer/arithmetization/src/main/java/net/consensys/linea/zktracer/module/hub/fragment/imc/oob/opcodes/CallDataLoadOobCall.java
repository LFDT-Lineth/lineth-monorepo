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

import static net.consensys.linea.zktracer.Trace.OOB_INST_CDL;
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
public class CallDataLoadOobCall extends OobCall {

  // Inputs
  @EqualsAndHashCode.Include EWord offset;
  @EqualsAndHashCode.Include EWord cds;

  // Outputs
  boolean cdlOutOfBounds;

  public CallDataLoadOobCall() {
    super();
  }

  @Override
  public void setInputs(Hub hub, MessageFrame frame) {
    setOffset(EWord.of(frame.getStackItem(0)));
    setCds(EWord.of(frame.getInputData().size()));
  }

  @Override
  public void setOutputs() {
    setCdlOutOfBounds(cds.compareTo(offset) <= 0);
  }

  @Override
  public Trace.Oob traceOob(Trace.Oob trace) {
    return trace
        .inst(OOB_INST_CDL)
        .data1(offset.hi())
        .data2(offset.lo())
        .data5(cds)
        .data7(booleanToBytes(cdlOutOfBounds))
        .fillAndValidateRow();
  }

  @Override
  public Trace.Hub traceHub(Trace.Hub trace) {
    return trace
        .pMiscOobFlag(true)
        .pMiscOobInst(OOB_INST_CDL)
        .pMiscOobData1(offset.hi())
        .pMiscOobData2(offset.lo())
        .pMiscOobData5(cds)
        .pMiscOobData7(booleanToBytes(cdlOutOfBounds));
  }
}
