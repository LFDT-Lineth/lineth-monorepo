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
import static net.consensys.linea.zktracer.Trace.Oob.CT_MAX_CDL;
import static net.consensys.linea.zktracer.module.oob.OobExoCall.callToLT;
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
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.evm.frame.MessageFrame;

@Getter
@Setter
@EqualsAndHashCode(onlyExplicitlyIncluded = true, callSuper = false)
public class CallDataLoadOobCall extends OobCall {

  public static final short NB_ROWS_OOB_CDL = CT_MAX_CDL + 1;

  // Inputs
  @EqualsAndHashCode.Include EWord offset;
  @EqualsAndHashCode.Include Bytes cds;

  // Outputs
  boolean cdlOutOfBounds;

  public CallDataLoadOobCall() {
    super();
  }

  @Override
  public void setInputData(MessageFrame frame, Hub hub) {
    setOffset(EWord.of(frame.getStackItem(0)));
    setCds(Bytes.ofUnsignedLong(frame.getInputData().size()));
  }

  @Override
  public void callExoModulesAndSetOutputs(Add add, Mod mod, Wcp wcp) {
    // row i
    final OobExoCall touchesRamCall = callToLT(wcp, offset, cds);
    exoCalls.add(touchesRamCall);
    final boolean touchesRam = bytesToBoolean(touchesRamCall.result());

    setCdlOutOfBounds(!touchesRam);
  }

  @Override
  public int ctMax() {
    return CT_MAX_CDL;
  }

  @Override
  public Trace.Oob trace(Trace.Oob trace) {
    return trace
        .isCdl(true)
        .oobInst(OOB_INST_CDL)
        .data1(offset.hi())
        .data2(offset.lo())
        .data5(cds)
        .data7(booleanToBytes(cdlOutOfBounds));
  }

  @Override
  public Trace.Hub trace(Trace.Hub trace) {
    return trace
        .pMiscOobFlag(true)
        .pMiscOobInst(OOB_INST_CDL)
        .pMiscOobData1(offset.hi())
        .pMiscOobData2(offset.lo())
        .pMiscOobData5(cds)
        .pMiscOobData7(booleanToBytes(cdlOutOfBounds));
  }
}
