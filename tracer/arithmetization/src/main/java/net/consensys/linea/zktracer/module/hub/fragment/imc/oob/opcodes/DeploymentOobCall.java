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

import static net.consensys.linea.zktracer.Trace.MAX_CODE_SIZE;
import static net.consensys.linea.zktracer.Trace.OOB_INST_DEPLOYMENT;
import static net.consensys.linea.zktracer.types.Conversions.*;

import lombok.EqualsAndHashCode;
import lombok.Getter;
import lombok.Setter;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.OobCall;
import net.consensys.linea.zktracer.types.EWord;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.evm.frame.MessageFrame;

@Getter
@Setter
@EqualsAndHashCode(onlyExplicitlyIncluded = true, callSuper = false)
public class DeploymentOobCall extends OobCall {

  private static final Bytes MAX_CODE_SIZE_BYTES = Bytes.ofUnsignedLong(MAX_CODE_SIZE);

  // Inputs
  @EqualsAndHashCode.Include EWord size;

  // Outputs
  boolean maxCodeSizeException;

  public DeploymentOobCall() {
    super();
  }

  @Override
  public void setInputs(Hub hub, MessageFrame frame) {
    setSize(EWord.of(frame.getStackItem(1)));
  }

  @Override
  public void setOutputs() {
    setMaxCodeSizeException(EWord.of(MAX_CODE_SIZE_BYTES).compareTo(size) < 0);
  }

  @Override
  public Trace.Oob traceOob(Trace.Oob trace) {
    return trace
        .inst(OOB_INST_DEPLOYMENT)
        .data1(size.hi())
        .data2(size.lo())
        .data7(booleanToBytes(maxCodeSizeException))
        .fillAndValidateRow();
  }

  @Override
  public Trace.Hub traceHub(Trace.Hub trace) {
    return trace
        .pMiscOobFlag(true)
        .pMiscOobInst(OOB_INST_DEPLOYMENT)
        .pMiscOobData1(size.hi())
        .pMiscOobData2(size.lo())
        .pMiscOobData7(booleanToBytes(maxCodeSizeException));
  }
}
