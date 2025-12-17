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

package net.consensys.linea.zktracer.module.hub.fragment.imc.oob.opcodes.create;

import static net.consensys.linea.zktracer.Trace.*;
import static net.consensys.linea.zktracer.Trace.Oob.CT_MAX_XCREATE;
import static net.consensys.linea.zktracer.module.oob.OobExoCall.callToLT;
import static net.consensys.linea.zktracer.module.txndata.transactions.UserTransaction.MAX_INIT_CODE_SIZE_BYTES;

import lombok.EqualsAndHashCode;
import lombok.Getter;
import lombok.Setter;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.add.Add;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.OobCall;
import net.consensys.linea.zktracer.module.mod.Mod;
import net.consensys.linea.zktracer.module.wcp.Wcp;
import net.consensys.linea.zktracer.types.EWord;
import org.hyperledger.besu.evm.frame.MessageFrame;

@Getter
@Setter
@EqualsAndHashCode(onlyExplicitlyIncluded = true, callSuper = false)
public class XCreateOobCall extends OobCall {
  // Inputs
  @EqualsAndHashCode.Include EWord codeSize;

  public XCreateOobCall() {
    super();
  }

  @Override
  public void setInputData(MessageFrame frame, Hub hub) {
    setCodeSize(EWord.of(frame.getStackItem(2)));
  }

  @Override
  public void callExoModulesAndSetOutputs(Add add, Mod mod, Wcp wcp) {
    exoCalls.add(callToLT(wcp, MAX_INIT_CODE_SIZE_BYTES, codeSize));
  }

  @Override
  public int ctMax() {
    return CT_MAX_XCREATE;
  }

  @Override
  public Trace.Oob trace(Trace.Oob trace) {
    return trace
        .isXcreate(true)
        .oobInst(OOB_INST_XCREATE)
        .data1(codeSize.hi())
        .data2(codeSize.lo());
  }

  @Override
  public Trace.Hub trace(Trace.Hub trace) {
    return trace
        .pMiscOobFlag(true)
        .pMiscOobInst(OOB_INST_XCREATE)
        .pMiscOobData1(codeSize.hi())
        .pMiscOobData2(codeSize.lo());
  }
}
