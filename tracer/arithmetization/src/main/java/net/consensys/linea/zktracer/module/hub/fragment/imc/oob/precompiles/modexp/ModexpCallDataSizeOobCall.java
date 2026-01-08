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

package net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.modexp;

import static net.consensys.linea.zktracer.Trace.OOB_INST_MODEXP_CDS;
import static net.consensys.linea.zktracer.module.hub.precompiles.ModexpMetadata.*;
import static net.consensys.linea.zktracer.types.Conversions.*;

import lombok.EqualsAndHashCode;
import lombok.Getter;
import lombok.Setter;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.OobCall;
import net.consensys.linea.zktracer.module.hub.precompiles.ModexpMetadata;
import net.consensys.linea.zktracer.opcode.OpCodeData;
import net.consensys.linea.zktracer.types.EWord;
import org.hyperledger.besu.evm.frame.MessageFrame;

@Getter
@Setter
@EqualsAndHashCode(onlyExplicitlyIncluded = true, callSuper = false)
public class ModexpCallDataSizeOobCall extends OobCall {

  // Inputs
  @EqualsAndHashCode.Include final ModexpMetadata modexpMetadata;
  @EqualsAndHashCode.Include EWord cds;

  public ModexpCallDataSizeOobCall(final ModexpMetadata modexpMetadata) {
    super();
    this.modexpMetadata = modexpMetadata;
  }

  @Override
  public void setInputs(Hub hub, MessageFrame frame) {
    final OpCodeData opCode = hub.opCodeData(frame);
    cds = EWord.of(frame.getStackItem(opCode.callCdsStackIndex()));
  }

  @Override
  public void setOutputs() {}

  @Override
  public Trace.Oob traceOob(Trace.Oob trace) {
    return trace
        .inst(OOB_INST_MODEXP_CDS)
        .data2(cds.trimLeadingZeros())
        .data3(booleanToBytes(modexpMetadata.extractBbs()))
        .data4(booleanToBytes(modexpMetadata.extractEbs()))
        .data5(booleanToBytes(modexpMetadata.extractMbs()))
        .fillAndValidateRow();
  }

  @Override
  public Trace.Hub traceHub(Trace.Hub trace) {
    return trace
        .pMiscOobFlag(true)
        .pMiscOobInst(OOB_INST_MODEXP_CDS)
        .pMiscOobData2(cds.trimLeadingZeros())
        .pMiscOobData3(booleanToBytes(modexpMetadata.extractBbs()))
        .pMiscOobData4(booleanToBytes(modexpMetadata.extractEbs()))
        .pMiscOobData5(booleanToBytes(modexpMetadata.extractMbs()));
  }
}
