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

import static net.consensys.linea.zktracer.Trace.OOB_INST_MODEXP_LEAD;
import static net.consensys.linea.zktracer.Trace.WORD_SIZE;
import static net.consensys.linea.zktracer.module.hub.precompiles.ModexpMetadata.BASE_MIN_OFFSET;
import static net.consensys.linea.zktracer.module.hub.precompiles.ModexpMetadata.EBS_MIN_OFFSET;
import static net.consensys.linea.zktracer.types.Conversions.*;

import java.math.BigInteger;
import lombok.EqualsAndHashCode;
import lombok.Getter;
import lombok.Setter;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.OobCall;
import net.consensys.linea.zktracer.module.hub.precompiles.ModexpMetadata;
import net.consensys.linea.zktracer.opcode.OpCodeData;
import net.consensys.linea.zktracer.types.EWord;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.evm.frame.MessageFrame;

@Getter
@Setter
@EqualsAndHashCode(onlyExplicitlyIncluded = true, callSuper = false)
public class ModexpLeadOobCall extends OobCall {

  // Inputs
  @EqualsAndHashCode.Include final ModexpMetadata metadata;
  @EqualsAndHashCode.Include EWord cds;

  // Outputs
  boolean loadLead;
  int cdsCutoff;
  int ebsCutoff;
  int subEbs32;

  public ModexpLeadOobCall(ModexpMetadata metadata) {
    super();
    this.metadata = metadata;
  }

  @Override
  public void setInputs(Hub hub, MessageFrame frame) {
    final OpCodeData opCode = hub.opCodeData(frame);
    cds = EWord.of(frame.getStackItem(opCode.callCdsStackIndex()));
  }

  @Override
  public void setOutputs() {
    final boolean ebsIsZero = metadata.normalizedEbs().isZero();
    final boolean ebsLessThan32 =
        EWord.of(metadata.normalizedEbs()).compareTo(EWord.of(EBS_MIN_OFFSET)) < 0;
    final boolean callDataContainsExponentBytes =
        EWord.of(metadata.normalizedBbs()).add(BASE_MIN_OFFSET).compareTo(cds) < 0;
    final boolean comp =
        callDataContainsExponentBytes
            && cds.subtract(BASE_MIN_OFFSET)
                    .subtract(metadata.normalizedBbsInt())
                    .compareTo(EWord.of(WORD_SIZE))
                < 0;

    final boolean loadLead = callDataContainsExponentBytes && !ebsIsZero;
    setLoadLead(loadLead);

    // Set cdsCutoff
    if (!callDataContainsExponentBytes) {
      setCdsCutoff(0);
    } else {
      setCdsCutoff(
          comp
              ? (cds.toUnsignedBigInteger()
                  .subtract(
                      BigInteger.valueOf(96).add(metadata.normalizedBbs().toUnsignedBigInteger()))
                  .intValue())
              : 32);
    }
    // Set ebsCutoff
    setEbsCutoff(ebsLessThan32 ? metadata.normalizedEbsInt() : 32);

    // Set subEbs32
    setSubEbs32(ebsLessThan32 ? 0 : metadata.normalizedEbsInt() - 32);
  }

  @Override
  public Trace.Oob traceOob(Trace.Oob trace) {
    return trace
        .inst(OOB_INST_MODEXP_LEAD)
        .data1(metadata.normalizedBbs())
        .data2(cds.trimLeadingZeros())
        .data3(metadata.normalizedEbs())
        .data4(booleanToBytes(loadLead))
        .data6(Bytes.ofUnsignedInt(cdsCutoff))
        .data7(Bytes.ofUnsignedInt(ebsCutoff))
        .data8(Bytes.ofUnsignedInt(subEbs32))
        .fillAndValidateRow();
  }

  @Override
  public Trace.Hub traceHub(Trace.Hub trace) {
    return trace
        .pMiscOobFlag(true)
        .pMiscOobInst(OOB_INST_MODEXP_LEAD)
        .pMiscOobData1(metadata.normalizedBbs())
        .pMiscOobData2(cds.trimLeadingZeros())
        .pMiscOobData3(metadata.normalizedEbs())
        .pMiscOobData4(booleanToBytes(loadLead))
        .pMiscOobData6(Bytes.ofUnsignedInt(cdsCutoff))
        .pMiscOobData7(Bytes.ofUnsignedInt(ebsCutoff))
        .pMiscOobData8(Bytes.ofUnsignedInt(subEbs32));
  }
}
