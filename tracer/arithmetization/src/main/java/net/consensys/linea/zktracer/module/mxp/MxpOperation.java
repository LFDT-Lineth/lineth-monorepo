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

package net.consensys.linea.zktracer.module.mxp;

import lombok.Getter;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.container.ModuleOperation;
import net.consensys.linea.zktracer.module.hub.fragment.imc.MxpCall;
import net.consensys.linea.zktracer.module.mxp.moduleCall.CancunMxpCall;
import net.consensys.linea.zktracer.opcode.OpCode;
import net.consensys.linea.zktracer.opcode.OpCodeData;
import net.consensys.linea.zktracer.types.UnsignedByte;
import org.apache.tuweni.bytes.Bytes;

@Getter
public final class MxpOperation extends ModuleOperation {

  /**
   * {@link CancunMxpOperation#MXP_FROM_CTMAX_TO_LINECOUNT} is used to convert <b>ctMax</b>, which
   * accounts only for computation rows, to the full line count of the MXP instruction, i.e. number
   * of rows of MXP instruction as a whole.
   *
   * <p>The conversion is
   *
   * <pre> nRows = ctMax + {@link CancunMxpOperation#MXP_FROM_CTMAX_TO_LINECOUNT} </pre>
   *
   * <p>{@link CancunMxpOperation#MXP_FROM_CTMAX_TO_LINECOUNT} is the sum of the following
   * components:
   *
   * <ul>
   *   <li>3 ≡ 1 decoder row + 1 macro row + 1 scenario row
   *   <li>1 ≡ to convert <b>ctMax</b> (of computation rows) to "<b>nComputationRows</b>"
   * </ul>
   */
  public static short MXP_FROM_CTMAX_TO_LINECOUNT = 4;

  private final CancunMxpCall mxpCall;
  private final int contextNumber;

  public MxpOperation(MxpCall mxpCall) {
    this.mxpCall = (CancunMxpCall) mxpCall;
    contextNumber = mxpCall.hub.currentFrame().contextNumber();
  }

  @Override
  protected int computeLineCount() {
    return mxpCall.ctMax() + MXP_FROM_CTMAX_TO_LINECOUNT;
  }

  public void trace(int stamp, Trace.Mxp trace) {
    traceDecoder(stamp, trace);
    traceMacro(stamp, trace);
    traceScenario(stamp, trace);
    traceComputation(stamp, trace);
  }

  private void traceShared(int stamp, Trace.Mxp trace) {
    trace.mxpStamp(stamp).cn(contextNumber);
  }

  private void traceDecoder(int stamp, Trace.Mxp trace) {
    final OpCodeData opCodeData = mxpCall.getOpCodeData();
    traceShared(stamp, trace);
    trace
        .decoder(true)
        .pDecoderInst(UnsignedByte.of(opCodeData.mnemonic().byteValue()))
        .pDecoderIsMsize(opCodeData.isMSize())
        .pDecoderIsReturn(opCodeData.isReturn())
        .pDecoderIsMcopy(opCodeData.isMCopy())
        .pDecoderIsFixedSize32(opCodeData.isFixedSize32())
        .pDecoderIsFixedSize1(opCodeData.isFixedSize1())
        .pDecoderIsSingleMaxOffset(opCodeData.isSingleOffset())
        .pDecoderIsDoubleMaxOffset(opCodeData.isDoubleOffset())
        .pDecoderIsWordPricing(opCodeData.isWordPricing())
        .pDecoderIsBytePricing(opCodeData.isBytePricing())
        .pDecoderGword(mxpCall.gWord)
        .pDecoderGbyte(mxpCall.gByte)
        .fillAndValidateRow();
  }

  private void traceMacro(int stamp, Trace.Mxp trace) {
    final OpCode opCode = mxpCall.getOpCodeData().mnemonic();
    traceShared(stamp, trace);
    trace
        .macro(true)
        .pMacroInst(UnsignedByte.of(opCode.byteValue()))
        .pMacroDeploying(mxpCall.isDeploys())
        .pMacroOffset1Hi(mxpCall.getOffset1().hi())
        .pMacroOffset1Lo(mxpCall.getOffset1().lo())
        .pMacroSize1Hi(mxpCall.getSize1().hi())
        .pMacroSize1Lo(mxpCall.getSize1().lo())
        .pMacroOffset2Hi(mxpCall.getOffset2().hi())
        .pMacroOffset2Lo(mxpCall.getOffset2().lo())
        .pMacroSize2Hi(mxpCall.getSize2().hi())
        .pMacroSize2Lo(mxpCall.getSize2().lo())
        .pMacroRes(mxpCall.isMSizeScenario() ? mxpCall.getMemorySizeInWords() : 0L)
        .pMacroMxpx(mxpCall.isMxpx())
        .pMacroGasMxp(Bytes.ofUnsignedLong(mxpCall.getGasMxp()))
        .pMacroS1Nznomxpx(!mxpCall.getSize1().isZero() && !mxpCall.isMxpx())
        .pMacroS2Nznomxpx(!mxpCall.getSize2().isZero() && !mxpCall.isMxpx())
        .fillAndValidateRow();
  }

  private void traceScenario(int stamp, Trace.Mxp trace) {
    traceShared(stamp, trace);
    trace
        .scenario(true)
        .pScenarioMsize(mxpCall.isMSizeScenario())
        .pScenarioTrivial(mxpCall.isTrivialScenario())
        .pScenarioMxpx(mxpCall.isMxpxScenario())
        .pScenarioStateUpdateWordPricing(mxpCall.isStateUpdateWordPricingScenario())
        .pScenarioStateUpdateBytePricing(mxpCall.isStateUpdateBytePricingScenario())
        .pScenarioWords(mxpCall.words)
        .pScenarioWordsNew(mxpCall.wordsNew)
        .pScenarioCmem(Bytes.ofUnsignedLong(mxpCall.cMem))
        .pScenarioCmemNew(Bytes.ofUnsignedLong(mxpCall.cMemNew))
        .fillAndValidateRow();
  }

  private void traceComputation(int stamp, Trace.Mxp trace) {
    final short ctMax = (short) mxpCall.ctMax();
    for (int ct = 0; ct <= ctMax; ct++) {
      traceShared(stamp, trace);
      trace
          .computation(true)
          .ct(ct)
          .ctMax(ctMax)
          .pComputationWcpFlag(mxpCall.exoCalls[ct].wcpFlag())
          .pComputationEucFlag(mxpCall.exoCalls[ct].eucFlag())
          .pComputationExoInst(mxpCall.exoCalls[ct].instruction())
          .pComputationArg1Hi(mxpCall.exoCalls[ct].arg1Hi())
          .pComputationArg1Lo(mxpCall.exoCalls[ct].arg1Lo())
          .pComputationArg2Hi(mxpCall.exoCalls[ct].arg2Hi())
          .pComputationArg2Lo(mxpCall.exoCalls[ct].arg2Lo())
          .pComputationResA(mxpCall.exoCalls[ct].resultA())
          .pComputationResB(mxpCall.exoCalls[ct].resultB())
          .fillAndValidateRow();
    }
  }
}
