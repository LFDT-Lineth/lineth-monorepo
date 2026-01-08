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

package net.consensys.linea.zktracer.module.hub.fragment.imc;

import static net.consensys.linea.zktracer.module.mxp.MxpUtils.*;

import lombok.Getter;
import lombok.Setter;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.fragment.TraceSubFragment;
import net.consensys.linea.zktracer.module.hub.state.State;
import net.consensys.linea.zktracer.module.mxp.moduleCall.*;
import net.consensys.linea.zktracer.opcode.OpCode;
import net.consensys.linea.zktracer.opcode.OpCodeData;
import net.consensys.linea.zktracer.opcode.gas.BillingRate;
import net.consensys.linea.zktracer.types.EWord;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.evm.frame.MessageFrame;

/**
 * This is the parent class for all MXP Calls. The fork dependent classes extending this are located
 * in Mxp module (LondonMxpCall, CancunMxpCall, ...).
 */
public abstract class MxpCall implements TraceSubFragment {

  public final Hub hub;

  /** The following properties will be filled in by MXP module * */
  /** - don't necessitate computation * */
  @Getter public OpCodeData opCodeData;

  @Getter public boolean deploys;
  @Getter public long memorySizeInWords;
  @Getter public EWord offset1;
  @Getter public EWord size1;
  @Getter public EWord offset2;
  @Getter public EWord size2;

  /** - filled after computation by the module */
  @Getter @Setter public boolean mayTriggerNontrivialMmuOperation;

  /** mxpx is short of Memory eXPansion eXception */
  @Getter @Setter public boolean mxpx;

  @Getter @Setter public long gasMxp;

  public MxpCall(Hub hub) {
    this.hub = hub;
    final MessageFrame frame = hub.messageFrame();
    opCodeData = this.hub.opCodeData();
    deploys = opCodeData.mnemonic() == OpCode.RETURN && hub.currentFrame().isDeployment();
    memorySizeInWords = hub.messageFrame().memoryWordSize();
    final EWord[] sizesAndOffsets = getSizesAndOffsets(frame, opCodeData);
    size1 = sizesAndOffsets[0];
    offset1 = sizesAndOffsets[1];
    size2 = sizesAndOffsets[2];
    offset2 = sizesAndOffsets[3];
  }

  public static MxpCall newMxpCall(Hub hub) {
    return generateMxpCall(hub);
  }

  /**
   * User from Cancun fork - Get the correct Mxp scenarii: CancunMSizeMxpCall, CancunTrivialMxpCall,
   * CancunMxpxMxpCall, CancunStateUpdateWordPricingMxpCall or CancunStateUpdateBytePricingMxpCall.
   *
   * @param hub instance of Hub used to create the CancunMxpCall
   * @return CancunMxpCall instance corresponding to the Mxp scenario
   */
  public static CancunMxpCall generateMxpCall(Hub hub) {
    final OpCodeData opCodeData = hub.opCodeData();
    if (opCodeData.isMSize()) {
      return new CancunMSizeMxpCall(hub);
    }
    final EWord[] sizesAndOffsets = getSizesAndOffsets(hub.messageFrame(), opCodeData);
    final EWord size1 = sizesAndOffsets[0];
    final EWord size2 = sizesAndOffsets[2];
    if (size1.isZero() && size2.isZero()) {
      return new CancunTrivialMxpCall(hub);
    }
    final CancunNotMSizeNorTrivialMxpCall cancunNotMSizeNorTrivialMxpCall =
        new CancunNotMSizeNorTrivialMxpCall(hub);
    if (cancunNotMSizeNorTrivialMxpCall.mxpx) {
      return new CancunMxpxMxpCall(hub, cancunNotMSizeNorTrivialMxpCall.mxpx);
    } else {
      if (opCodeData.isWordPricing()) {
        return new CancunStateUpdateWordPricingMxpCall(hub);
      }
      return new CancunStateUpdateBytePricingMxpCall(hub);
    }
  }

  public boolean getSize1NonZeroNoMxpx() {
    return !this.mxpx && !this.size1.isZero();
  }

  public boolean getSize2NonZeroNoMxpx() {
    return !this.mxpx && !this.size2.isZero();
  }

  public int getCostBy(BillingRate billingRate) {
    return getOpCodeData().billing().billingRate() == billingRate
        ? getOpCodeData().billing().perUnit().cost()
        : 0;
  }

  protected void setMayTriggerNontrivialMmuOperation() {}

  @Override
  public Trace.Hub traceHub(Trace.Hub trace, State hubState) {
    hubState.incrementMxpStamp();
    return trace
        .pMiscMxpFlag(true)
        .pMiscMxpInst(opCodeData.value())
        .pMiscMxpDeploys(deploys)
        .pMiscMxpOffset1Hi(offset1.hi())
        .pMiscMxpOffset1Lo(offset1.lo())
        .pMiscMxpSize1Hi(size1.hi())
        .pMiscMxpSize1Lo(size1.lo())
        .pMiscMxpOffset2Hi(offset2.hi())
        .pMiscMxpOffset2Lo(offset2.lo())
        .pMiscMxpSize2Hi(size2.hi())
        .pMiscMxpSize2Lo(size2.lo())
        .pMiscMxpSize1NonzeroNoMxpx(getSize1NonZeroNoMxpx())
        .pMiscMxpSize2NonzeroNoMxpx(getSize2NonZeroNoMxpx())
        .pMiscMxpMxpx(mxpx)
        .pMiscMxpGasMxp(Bytes.ofUnsignedLong(gasMxp))
        .pMiscMxpWords(
            opCodeData.isMSize() ? Bytes.ofUnsignedLong(memorySizeInWords) : Bytes.EMPTY);
  }
}
