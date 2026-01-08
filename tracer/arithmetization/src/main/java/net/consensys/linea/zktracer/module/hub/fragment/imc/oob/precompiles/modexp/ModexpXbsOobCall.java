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

import static net.consensys.linea.zktracer.Trace.OOB_INST_MODEXP_XBS;
import static net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.modexp.ModexpXbsCase.MODEXP_XBS_CASE_BBS;
import static net.consensys.linea.zktracer.types.Conversions.*;

import lombok.EqualsAndHashCode;
import lombok.Getter;
import lombok.Setter;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.OobCall;
import net.consensys.linea.zktracer.module.hub.precompiles.ModexpMetadata;
import net.consensys.linea.zktracer.types.EWord;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.evm.frame.MessageFrame;

@Getter
@Setter
@EqualsAndHashCode(onlyExplicitlyIncluded = true, callSuper = false)
public final class ModexpXbsOobCall extends OobCall {

  // Inputs
  @EqualsAndHashCode.Include final ModexpMetadata modexpMetadata;
  @EqualsAndHashCode.Include final ModexpXbsCase modexpXbsCase;

  public ModexpXbsOobCall(ModexpMetadata modexpMetaData, ModexpXbsCase modexpXbsCase) {
    super();
    this.modexpMetadata = modexpMetaData;
    this.modexpXbsCase = modexpXbsCase;
  }

  @Override
  public void setInputs(Hub hub, MessageFrame frame) {}

  @Override
  public void setOutputs() {}

  private EWord xbs() {
    return modexpMetadata.xbs(modexpXbsCase);
  }

  private short xbsNormalized() {
    return (short) modexpMetadata.normalize(modexpXbsCase).toInt();
  }

  private short ybsReNormalized() {
    return xbsIsWithinBounds() ? (short) ybsLo().toInt() : 0;
  }

  private short maxXbsYbs() {
    return computeMax() && xbsIsWithinBounds()
        ? (short) Math.max(xbsNormalized(), this.ybsReNormalized())
        : 0;
  }

  private boolean xbsNormalizedIsNonZero() {
    return xbsIsWithinBounds() && xbsNormalized() != 0;
  }

  private boolean xbsNormalizedIsNonZeroTracedValue() {
    return xbsNormalizedIsNonZero();
  }

  private boolean xbsIsWithinBounds() {
    return modexpMetadata.tracedIsWithinBounds(modexpXbsCase);
  }

  private boolean xbsIsOutOfBounds() {
    return modexpMetadata.tracedIsOutOfBounds(modexpXbsCase);
  }

  public Bytes ybsLo() {
    return switch (modexpXbsCase) {
      case MODEXP_XBS_CASE_BBS, MODEXP_XBS_CASE_EBS -> Bytes.EMPTY;
      case MODEXP_XBS_CASE_MBS -> modexpMetadata.normalizedBbs();
    };
  }

  private boolean computeMax() {
    return switch (modexpXbsCase) {
      case MODEXP_XBS_CASE_BBS, MODEXP_XBS_CASE_EBS -> false;
      case MODEXP_XBS_CASE_MBS -> modexpMetadata.tracedIsWithinBounds(MODEXP_XBS_CASE_BBS);
    };
  }

  @Override
  public Trace.Oob traceOob(Trace.Oob trace) {
    return trace
        .inst(OOB_INST_MODEXP_XBS)
        .data1(xbs().hi())
        .data2(xbs().lo())
        .data3(ybsLo())
        .data4(booleanToBytes(computeMax()))
        .data7(Bytes.ofUnsignedShort(maxXbsYbs()))
        .data8(booleanToBytes(xbsNormalizedIsNonZeroTracedValue()))
        .data9(booleanToBytes(xbsIsWithinBounds()))
        .data10(booleanToBytes(xbsIsOutOfBounds()))
        .fillAndValidateRow();
  }

  @Override
  public Trace.Hub traceHub(Trace.Hub trace) {
    return trace
        .pMiscOobFlag(true)
        .pMiscOobInst(OOB_INST_MODEXP_XBS)
        .pMiscOobData1(xbs().hi())
        .pMiscOobData2(xbs().lo())
        .pMiscOobData3(ybsLo())
        .pMiscOobData4(booleanToBytes(computeMax()))
        .pMiscOobData7(Bytes.ofUnsignedShort(maxXbsYbs()))
        .pMiscOobData8(booleanToBytes(xbsNormalizedIsNonZeroTracedValue()))
        .pMiscOobData9(booleanToBytes(xbsIsWithinBounds()))
        .pMiscOobData10(booleanToBytes(xbsIsOutOfBounds()));
  }
}
