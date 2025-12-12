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

import static net.consensys.linea.zktracer.Trace.OOB_INST_XCALL;
import static net.consensys.linea.zktracer.Trace.Oob.CT_MAX_XCALL;
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
import net.consensys.linea.zktracer.types.EWord;
import org.hyperledger.besu.evm.frame.MessageFrame;

@Getter
@Setter
@EqualsAndHashCode(onlyExplicitlyIncluded = true, callSuper = false)
public class XCallOobCall extends OobCall {
  // Inputs
  @EqualsAndHashCode.Include EWord value;

  // Outputs
  boolean valueIsNonzero;
  boolean valueIsZero;

  public XCallOobCall() {
    super();
  }

  @Override
  public void setInputData(MessageFrame frame, Hub hub) {
    setValue(EWord.of(frame.getStackItem(2)));
  }

  @Override
  public void callExoModulesAndSetOutputs(Add add, Mod mod, Wcp wcp) {
    // row i
    final OobExoCall valueIsZeroCall = callToIsZero(wcp, value);
    exoCalls.add(valueIsZeroCall);
    final boolean valueIsZero = bytesToBoolean(valueIsZeroCall.result());

    setValueIsNonzero(!valueIsZero);
    setValueIsZero(valueIsZero);
  }

  @Override
  public int ctMax() {
    return CT_MAX_XCALL;
  }

  @Override
  public Trace.Oob trace(Trace.Oob trace) {
    return trace
        .isXcall(true)
        .oobInst(OOB_INST_XCALL)
        .data1(value.hi())
        .data2(value.lo())
        .data7(booleanToBytes(valueIsNonzero))
        .data8(booleanToBytes(valueIsZero));
  }

  @Override
  public Trace.Hub trace(Trace.Hub trace) {
    return trace
        .pMiscOobFlag(true)
        .pMiscOobInst(OOB_INST_XCALL)
        .pMiscOobData1(value.hi())
        .pMiscOobData2(value.lo())
        .pMiscOobData7(booleanToBytes(valueIsNonzero))
        .pMiscOobData8(booleanToBytes(valueIsZero));
  }
}
