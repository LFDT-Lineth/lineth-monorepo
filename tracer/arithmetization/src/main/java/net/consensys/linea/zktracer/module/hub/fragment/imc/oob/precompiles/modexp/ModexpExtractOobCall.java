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

import static net.consensys.linea.zktracer.Trace.OOB_INST_MODEXP_EXTRACT;
import static net.consensys.linea.zktracer.module.hub.precompiles.ModexpMetadata.BASE_MIN_OFFSET;
import static net.consensys.linea.zktracer.types.Conversions.*;

import java.math.BigInteger;
import lombok.EqualsAndHashCode;
import lombok.Getter;
import lombok.Setter;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.OobCall;
import net.consensys.linea.zktracer.module.hub.precompiles.ModexpMetadata;
import net.consensys.linea.zktracer.opcode.OpCodeData;
import net.consensys.linea.zktracer.types.EWord;
import org.hyperledger.besu.evm.frame.MessageFrame;

@Slf4j
@Getter
@Setter
@EqualsAndHashCode(onlyExplicitlyIncluded = true, callSuper = false)
public class ModexpExtractOobCall extends OobCall {

  // Inputs
  @EqualsAndHashCode.Include final ModexpMetadata metadata;
  @EqualsAndHashCode.Include EWord cds;

  // Outputs
  boolean extractBase;
  boolean extractExponent;
  boolean extractModulus;

  public ModexpExtractOobCall(ModexpMetadata metadata) {
    super();
    this.metadata = metadata;
  }

  @Override
  public void setInputs(Hub hub, MessageFrame frame) {
    final OpCodeData opCode = hub.opCodeData(frame);
    setCds(EWord.of(frame.getStackItem(opCode.callCdsStackIndex())));
  }

  @Override
  public void setOutputs() {
    final boolean bbsIsZero = metadata.bbs().isZero();
    final boolean ebsIsZero = metadata.ebs().isZero();
    final boolean mbsIsZero = metadata.mbs().isZero();
    final boolean callDataExtendsBeyondExponent =
        BigInteger.valueOf(BASE_MIN_OFFSET + metadata.bbsInt() + metadata.ebsInt())
                .compareTo(cds.toUnsignedBigInteger())
            < 0;

    setExtractModulus(callDataExtendsBeyondExponent && !mbsIsZero);
    setExtractBase(extractModulus && !bbsIsZero);
    setExtractExponent(extractModulus && !ebsIsZero);
  }

  @Override
  public Trace.Oob traceOob(Trace.Oob trace) {
    return trace
        .inst(OOB_INST_MODEXP_EXTRACT)
        .data2(cds.trimLeadingZeros())
        .data3(metadata.bbs().trimLeadingZeros())
        .data4(metadata.ebs().trimLeadingZeros())
        .data5(metadata.mbs().trimLeadingZeros())
        .data6(booleanToBytes(extractBase))
        .data7(booleanToBytes(extractExponent))
        .data8(booleanToBytes(extractModulus))
        .fillAndValidateRow();
  }

  @Override
  public Trace.Hub traceHub(Trace.Hub trace) {
    return trace
        .pMiscOobFlag(true)
        .pMiscOobInst(OOB_INST_MODEXP_EXTRACT)
        .pMiscOobData2(cds.trimLeadingZeros())
        .pMiscOobData3(metadata.bbs().trimLeadingZeros())
        .pMiscOobData4(metadata.ebs().trimLeadingZeros())
        .pMiscOobData5(metadata.mbs().trimLeadingZeros())
        .pMiscOobData6(booleanToBytes(extractBase))
        .pMiscOobData7(booleanToBytes(extractExponent))
        .pMiscOobData8(booleanToBytes(extractModulus));
  }
}
