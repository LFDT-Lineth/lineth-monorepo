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
import static net.consensys.linea.zktracer.Trace.Oob.CT_MAX_RDC;
import static net.consensys.linea.zktracer.module.oob.OobExoCall.*;
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
public class ReturnDataCopyOobCall extends OobCall {

  public static final short NB_ROWS_OOB_RDC = CT_MAX_RDC + 1;

  // Inputs
  @EqualsAndHashCode.Include EWord offset;
  @EqualsAndHashCode.Include EWord size;
  @EqualsAndHashCode.Include Bytes rds;

  // Outputs
  boolean rdcx;

  public ReturnDataCopyOobCall() {
    super();
  }

  @Override
  public void setInputData(MessageFrame frame, Hub hub) {
    setOffset(EWord.of(frame.getStackItem(1)));
    setSize(EWord.of(frame.getStackItem(2)));
    setRds(Bytes.ofUnsignedLong(frame.getReturnData().size()));
  }

  @Override
  public void callExoModulesAndSetOutputs(Add add, Mod mod, Wcp wcp) {
    // row i
    final OobExoCall rdcOobCall = callToIsZero(wcp, Bytes.concatenate(offset.hi(), size.hi()));
    exoCalls.add(rdcOobCall);
    final boolean rdcRoob = !bytesToBoolean(rdcOobCall.result());

    // row i + 1
    final OobExoCall secondCall = rdcRoob ? noCall() : callToADD(add, offset, size);
    exoCalls.add(secondCall);
    final Bytes sum = secondCall.result();

    // row i + 2
    final OobExoCall thirdCall = rdcRoob ? noCall() : callToGT(wcp, sum, rds);
    exoCalls.add(thirdCall);
    final boolean rdcSoob = bytesToBoolean(thirdCall.result());

    setRdcx(rdcRoob || rdcSoob);
  }

  @Override
  public int ctMax() {
    return CT_MAX_RDC;
  }

  @Override
  public Trace.Oob trace(Trace.Oob trace) {
    return trace
        .isRdc(true)
        .oobInst(OOB_INST_RDC)
        .data1(offset.hi())
        .data2(offset.lo())
        .data3(size.hi())
        .data4(size.lo())
        .data5(rds)
        .data7(booleanToBytes(rdcx));
  }

  @Override
  public Trace.Hub trace(Trace.Hub trace) {
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
