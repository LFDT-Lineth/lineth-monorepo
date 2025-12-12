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

package net.consensys.linea.zktracer.module.mxp.moduleOperation;

import static net.consensys.linea.zktracer.module.mxp.MxpUtils.*;

import lombok.Getter;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.hub.fragment.imc.MxpCall;
import net.consensys.linea.zktracer.module.mxp.moduleCall.*;
import net.consensys.linea.zktracer.opcode.OpCode;
import net.consensys.linea.zktracer.opcode.OpCodeData;
import net.consensys.linea.zktracer.types.UnsignedByte;
import org.apache.tuweni.bytes.Bytes;

@Getter
public class CancunMxpOperation extends MxpOperation {

  private final int contextNumber;
  private final CancunMxpCall cancunMxpCall;

  public CancunMxpOperation(final MxpCall mxpCall) {
    super(mxpCall);
    // Setting of global variables
    this.contextNumber = mxpCall.hub.currentFrame().contextNumber();

    this.cancunMxpCall = (CancunMxpCall) mxpCall;
  }

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

  @Override
  protected int computeLineCount() {
    return cancunMxpCall.ctMax() + MXP_FROM_CTMAX_TO_LINECOUNT;
  }

  @Override
  public final void trace(int stamp, Trace.Mxp trace) {
    traceDecoder(stamp, trace);
    traceMacro(stamp, trace);
    traceScenario(stamp, trace);
    traceComputation(stamp, trace);
  }

  private void traceShared(int stamp, Trace.Mxp trace) {
    trace.mxpStamp(stamp).cn(contextNumber);
  }

  private void traceDecoder(int stamp, Trace.Mxp trace) {
    final OpCodeData opCodeData = cancunMxpCall.getOpCodeData();
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
        .pDecoderGword(cancunMxpCall.gWord)
        .pDecoderGbyte(cancunMxpCall.gByte)
        .fillAndValidateRow();
  }

  private void traceMacro(int stamp, Trace.Mxp trace) {
    final OpCode opCode = cancunMxpCall.getOpCodeData().mnemonic();
    traceShared(stamp, trace);
    trace
        .macro(true)
        .pMacroInst(UnsignedByte.of(opCode.byteValue()))
        .pMacroDeploying(cancunMxpCall.isDeploys())
        .pMacroOffset1Hi(cancunMxpCall.getOffset1().hi())
        .pMacroOffset1Lo(cancunMxpCall.getOffset1().lo())
        .pMacroSize1Hi(cancunMxpCall.getSize1().hi())
        .pMacroSize1Lo(cancunMxpCall.getSize1().lo())
        .pMacroOffset2Hi(cancunMxpCall.getOffset2().hi())
        .pMacroOffset2Lo(cancunMxpCall.getOffset2().lo())
        .pMacroSize2Hi(cancunMxpCall.getSize2().hi())
        .pMacroSize2Lo(cancunMxpCall.getSize2().lo())
        .pMacroRes(cancunMxpCall.isMSizeScenario() ? cancunMxpCall.getMemorySizeInWords() : 0L)
        .pMacroMxpx(cancunMxpCall.isMxpx())
        .pMacroGasMxp(Bytes.ofUnsignedLong(cancunMxpCall.getGasMxp()))
        .pMacroS1Nznomxpx(!cancunMxpCall.getSize1().isZero() && !cancunMxpCall.isMxpx())
        .pMacroS2Nznomxpx(!cancunMxpCall.getSize2().isZero() && !cancunMxpCall.isMxpx())
        .fillAndValidateRow();
  }

  private void traceScenario(int stamp, Trace.Mxp trace) {
    traceShared(stamp, trace);
    trace
        .scenario(true)
        .pScenarioMsize(cancunMxpCall.isMSizeScenario())
        .pScenarioTrivial(cancunMxpCall.isTrivialScenario())
        .pScenarioMxpx(cancunMxpCall.isMxpxScenario())
        .pScenarioStateUpdateWordPricing(cancunMxpCall.isStateUpdateWordPricingScenario())
        .pScenarioStateUpdateBytePricing(cancunMxpCall.isStateUpdateBytePricingScenario())
        .pScenarioWords(cancunMxpCall.words)
        .pScenarioWordsNew(cancunMxpCall.wordsNew)
        .pScenarioCmem(Bytes.ofUnsignedLong(cancunMxpCall.cMem))
        .pScenarioCmemNew(Bytes.ofUnsignedLong(cancunMxpCall.cMemNew))
        .fillAndValidateRow();
  }

  private void traceComputation(int stamp, Trace.Mxp trace) {
    final short ctMax = (short) cancunMxpCall.ctMax();
    for (int ct = 0; ct <= ctMax; ct++) {
      traceShared(stamp, trace);
      trace
          .computation(true)
          .ct(ct)
          .ctMax(ctMax)
          .pComputationWcpFlag(cancunMxpCall.exoCalls[ct].wcpFlag())
          .pComputationEucFlag(cancunMxpCall.exoCalls[ct].eucFlag())
          .pComputationExoInst(cancunMxpCall.exoCalls[ct].instruction())
          .pComputationArg1Hi(cancunMxpCall.exoCalls[ct].arg1Hi())
          .pComputationArg1Lo(cancunMxpCall.exoCalls[ct].arg1Lo())
          .pComputationArg2Hi(cancunMxpCall.exoCalls[ct].arg2Hi())
          .pComputationArg2Lo(cancunMxpCall.exoCalls[ct].arg2Lo())
          .pComputationResA(cancunMxpCall.exoCalls[ct].resultA())
          .pComputationResB(cancunMxpCall.exoCalls[ct].resultB())
          .fillAndValidateRow();
    }
  }
}
