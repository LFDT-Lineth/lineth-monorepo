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

package net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.modexp.xbsOobCall;

import static net.consensys.linea.zktracer.Trace.OOB_INST_MODEXP_XBS;
import static net.consensys.linea.zktracer.Trace.Oob.CT_MAX_MODEXP_XBS;
import static net.consensys.linea.zktracer.module.oob.OobExoCall.callToIsZero;
import static net.consensys.linea.zktracer.module.oob.OobExoCall.callToLT;
import static net.consensys.linea.zktracer.types.Conversions.*;

import lombok.EqualsAndHashCode;
import lombok.Getter;
import lombok.Setter;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.add.Add;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.OobCall;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.modexp.ModexpXbsCase;
import net.consensys.linea.zktracer.module.hub.precompiles.modexpMetadata.ModexpMetadata;
import net.consensys.linea.zktracer.module.mod.Mod;
import net.consensys.linea.zktracer.module.oob.OobExoCall;
import net.consensys.linea.zktracer.module.wcp.Wcp;
import net.consensys.linea.zktracer.types.EWord;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.evm.frame.MessageFrame;

@Getter
@Setter
@EqualsAndHashCode(onlyExplicitlyIncluded = true, callSuper = false)
public abstract class ModexpXbsOobCall extends OobCall {

  public static final short NB_ROWS_OOB_MODEXP_XBS = CT_MAX_MODEXP_XBS + 1;

  // Inputs
  @EqualsAndHashCode.Include final ModexpMetadata modexpMetadata;
  @EqualsAndHashCode.Include final ModexpXbsCase modexpXbsCase;

  public ModexpXbsOobCall(ModexpMetadata modexpMetaData, ModexpXbsCase modexpXbsCase) {
    super();
    this.modexpMetadata = modexpMetaData;
    this.modexpXbsCase = modexpXbsCase;
  }

  protected abstract ModexpMetadata getForkAppropriateModexpMetadata();

  public abstract int modexpComponentByteSize();

  @Override
  public void setInputData(MessageFrame frame, Hub hub) {}

  @Override
  public void callExoModulesAndSetOutputs(Add add, Mod mod, Wcp wcp) {
    // row i
    final OobExoCall xbsVsModexpComponentByteSize =
        callToLT(wcp, xbs(), Bytes.ofUnsignedInt(modexpComponentByteSize() + 1));
    exoCalls.add(xbsVsModexpComponentByteSize);

    // row i + 1
    final OobExoCall compareXbsYbsCall =
        callToLT(
            wcp, Bytes.ofUnsignedShort(xbsNormalized()), Bytes.ofUnsignedShort(ybsReNormalized()));
    exoCalls.add(compareXbsYbsCall);

    // row i + 2
    final OobExoCall xbsIszeroCall = callToIsZero(wcp, Bytes.ofUnsignedShort(xbsNormalized()));
    exoCalls.add(xbsIszeroCall);
  }

  @Override
  public int ctMax() {
    return CT_MAX_MODEXP_XBS;
  }

  protected EWord xbs() {
    return modexpMetadata.xbs(modexpXbsCase);
  }

  abstract short xbsNormalized();

  abstract short ybsReNormalized();

  abstract short maxXbsYbs();

  abstract boolean xbsNormalizedIsNonZero();

  abstract boolean xbsNormalizedIsNonZeroTracedValue();

  protected abstract boolean xbsIsWithinBounds();

  protected abstract boolean xbsIsOutOfBounds();

  public Bytes ybsLo() {
    return switch (modexpXbsCase) {
      case MODEXP_XBS_CASE_BBS, MODEXP_XBS_CASE_EBS -> Bytes.EMPTY;
      case MODEXP_XBS_CASE_MBS -> getForkAppropriateModexpMetadata().normalizedBbs();
    };
  }

  protected abstract boolean computeMax();

  @Override
  public Trace.Oob trace(Trace.Oob trace) {
    return trace
        .isModexpXbs(true)
        .oobInst(OOB_INST_MODEXP_XBS)
        .data1(xbs().hi())
        .data2(xbs().lo())
        .data3(ybsLo())
        .data4(booleanToBytes(computeMax()))
        .data7(Bytes.ofUnsignedShort(maxXbsYbs()))
        .data8(booleanToBytes(xbsNormalizedIsNonZeroTracedValue()))
        .data9(booleanToBytes(xbsIsWithinBounds()))
        .data10(booleanToBytes(xbsIsOutOfBounds()));
  }

  @Override
  public Trace.Hub trace(Trace.Hub trace) {
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
