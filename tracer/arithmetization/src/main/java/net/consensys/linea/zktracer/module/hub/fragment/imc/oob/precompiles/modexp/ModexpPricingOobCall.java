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
package net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.modexp;

import static com.google.common.math.BigIntegerMath.log2;
import static java.lang.Math.max;
import static java.lang.Math.min;
import static net.consensys.linea.zktracer.Trace.OOB_INST_MODEXP_PRICING;
import static net.consensys.linea.zktracer.Trace.WORD_SIZE;
import static net.consensys.linea.zktracer.TraceOsaka.GAS_CONST_MODEXP_EIP_7823;
import static net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.modexp.ModexpXbsCase.MODEXP_XBS_CASE_BBS;
import static net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.modexp.ModexpXbsCase.MODEXP_XBS_CASE_MBS;
import static net.consensys.linea.zktracer.module.hub.precompiles.ModexpMetadata.BASE_MIN_OFFSET;
import static net.consensys.linea.zktracer.types.Conversions.*;
import static net.consensys.linea.zktracer.types.Utils.rightPadTo;
import static org.hyperledger.besu.evm.internal.Words.clampedToInt;

import java.math.BigInteger;
import java.math.RoundingMode;
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
public final class ModexpPricingOobCall extends OobCall {

  // Inputs
  @EqualsAndHashCode.Include final ModexpMetadata metadata;
  @EqualsAndHashCode.Include final EWord callGas;
  @EqualsAndHashCode.Include EWord returnAtCapacity;
  @EqualsAndHashCode.Include BigInteger exponentLog;

  // Outputs
  boolean ramSuccess;
  int maxMbsBbs;
  BigInteger returnGas;
  boolean returnAtCapacityNonZero;

  public ModexpPricingOobCall(ModexpMetadata metadata, long calleeGas) {
    super();
    this.metadata = metadata;
    this.callGas = EWord.of(calleeGas);
  }

  @Override
  public void setInputs(Hub hub, MessageFrame frame) {
    final OpCodeData opCode = hub.opCodeData(frame);
    setReturnAtCapacity(EWord.of(frame.getStackItem(opCode.callReturnAtCapacityStackIndex())));

    final int cds = clampedToInt(frame.getStackItem(opCode.callCdsStackIndex()));
    setExponentLog(BigInteger.valueOf(computeExponentLog(metadata, cds)));
  }

  @Override
  public void setOutputs() {
    final boolean returnAtCapacityIsZero = returnAtCapacity.isZero();
    final boolean exponentLogIsZero = exponentLog.equals(BigInteger.ZERO);

    final int maxMbsBbsValue =
        (metadata.tracedIsWithinBounds(MODEXP_XBS_CASE_BBS)
                && metadata.tracedIsWithinBounds(MODEXP_XBS_CASE_MBS))
            ? max(metadata.mbsInt(), metadata.bbsInt())
            : 0;
    setMaxMbsBbs(maxMbsBbsValue);

    final BigInteger ceilingOfMaxDividedBy8 =
        BigInteger.valueOf(maxMbsBbs + 7).divide(BigInteger.valueOf(8));
    final BigInteger fOfMax = ceilingOfMaxDividedBy8.multiply(ceilingOfMaxDividedBy8);
    final int fOfMaxInteger = fOfMax.intValue();

    final boolean wordCostDominates = WORD_SIZE < maxMbsBbs;
    final int multiplicationComplexity = wordCostDominates ? 2 * fOfMaxInteger : 16;
    final int iterationCountOr1 = exponentLogIsZero ? 1 : exponentLog.intValue();
    final int rawCost = multiplicationComplexity * iterationCountOr1;
    final boolean rawCostIsLessThanMinModexpCost = rawCost < GAS_CONST_MODEXP_EIP_7823;
    final int precompileCostInt =
        rawCostIsLessThanMinModexpCost ? GAS_CONST_MODEXP_EIP_7823 : rawCost;
    final EWord precompileCost = EWord.of(precompileCostInt);
    setRamSuccess(callGas.compareTo(precompileCost) >= 0);

    final BigInteger returnGas =
        ramSuccess
            ? callGas.toUnsignedBigInteger().subtract(precompileCost.toUnsignedBigInteger())
            : BigInteger.ZERO;
    setReturnGas(returnGas);

    setReturnAtCapacityNonZero(!returnAtCapacityIsZero);
  }

  @Override
  public Trace.Oob traceOob(Trace.Oob trace) {
    return trace
        .inst(OOB_INST_MODEXP_PRICING)
        .data1(callGas)
        .data3(returnAtCapacity.trimLeadingZeros())
        .data4(booleanToBytes(ramSuccess))
        .data5(bigIntegerToBytes(returnGas))
        .data6(bigIntegerToBytes(exponentLog))
        .data7(Bytes.ofUnsignedInt(maxMbsBbs))
        .data8(booleanToBytes(returnAtCapacityNonZero))
        .fillAndValidateRow();
  }

  @Override
  public Trace.Hub traceHub(Trace.Hub trace) {
    return trace
        .pMiscOobFlag(true)
        .pMiscOobInst(OOB_INST_MODEXP_PRICING)
        .pMiscOobData1(callGas)
        .pMiscOobData3(returnAtCapacity.trimLeadingZeros())
        .pMiscOobData4(booleanToBytes(ramSuccess))
        .pMiscOobData5(bigIntegerToBytes(returnGas))
        .pMiscOobData6(bigIntegerToBytes(exponentLog))
        .pMiscOobData7(Bytes.ofUnsignedInt(maxMbsBbs))
        .pMiscOobData8(booleanToBytes(returnAtCapacityNonZero));
  }

  // Support method for MODEXP
  public static int computeExponentLog(ModexpMetadata metadata, int cds) {
    final short bbs = metadata.normalizedBbsInt();
    final short ebs = metadata.normalizedEbsInt();
    return computeExponentLog(metadata, cds, bbs, ebs);
  }

  public static int computeExponentLog(ModexpMetadata modexpMetadata, int cds, int bbs, int ebs) {
    return computeExponentLog(modexpMetadata.callData(), 16, cds, bbs, ebs);
  }

  public static int computeExponentLog(Bytes callData, int multiplier, int cds, int bbs, int ebs) {
    // pad callData to 96 + bbs + ebs
    final Bytes paddedCallData =
        cds < BASE_MIN_OFFSET + bbs + ebs
            ? rightPadTo(callData, BASE_MIN_OFFSET + bbs + ebs)
            : callData;

    final BigInteger leadingBytesOfExponent =
        paddedCallData.slice(BASE_MIN_OFFSET + bbs, min(ebs, WORD_SIZE)).toUnsignedBigInteger();

    final int bitContribution =
        (leadingBytesOfExponent.signum() != 0)
            ? log2(leadingBytesOfExponent, RoundingMode.FLOOR)
            : 0;
    final int byteContribution = (ebs > WORD_SIZE) ? multiplier * (ebs - WORD_SIZE) : 0;

    return bitContribution + byteContribution;
  }
}
