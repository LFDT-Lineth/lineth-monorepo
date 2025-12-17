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

import static java.lang.Math.max;
import static net.consensys.linea.zktracer.Trace.OOB_INST_MODEXP_PRICING;
import static net.consensys.linea.zktracer.Trace.Oob.CT_MAX_MODEXP_PRICING;
import static net.consensys.linea.zktracer.TraceOsaka.GAS_CONST_MODEXP_EIP_7823;
import static net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.modexp.ModexpXbsCase.MODEXP_XBS_CASE_BBS;
import static net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.modexp.ModexpXbsCase.MODEXP_XBS_CASE_MBS;
import static net.consensys.linea.zktracer.module.oob.OobExoCall.*;
import static net.consensys.linea.zktracer.module.oob.OobExoCall.callToLT;
import static net.consensys.linea.zktracer.module.oob.OobOperation.computeExponentLog;
import static net.consensys.linea.zktracer.types.Conversions.*;
import static net.consensys.linea.zktracer.types.Conversions.bytesToBoolean;
import static org.hyperledger.besu.evm.internal.Words.clampedToInt;

import java.math.BigInteger;
import lombok.EqualsAndHashCode;
import lombok.Getter;
import lombok.Setter;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.add.Add;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.OobCall;
import net.consensys.linea.zktracer.module.hub.precompiles.ModexpMetadata;
import net.consensys.linea.zktracer.module.mod.Mod;
import net.consensys.linea.zktracer.module.oob.OobExoCall;
import net.consensys.linea.zktracer.module.wcp.Wcp;
import net.consensys.linea.zktracer.opcode.OpCodeData;
import net.consensys.linea.zktracer.types.EWord;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.evm.frame.MessageFrame;

@Getter
@Setter
@EqualsAndHashCode(onlyExplicitlyIncluded = true, callSuper = false)
public final class ModexpPricingOobCall extends OobCall {
  public static final short NB_ROWS_OOB_MODEXP_PRICING = CT_MAX_MODEXP_PRICING + 1;

  // Inputs
  @EqualsAndHashCode.Include final ModexpMetadata metadata;
  @EqualsAndHashCode.Include final Bytes callGas;
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
    this.callGas = Bytes.ofUnsignedLong(calleeGas);
  }

  @Override
  public void setInputData(MessageFrame frame, Hub hub) {
    final OpCodeData opCode = hub.opCodeData(frame);
    setReturnAtCapacity(EWord.of(frame.getStackItem(opCode.callReturnAtCapacityStackIndex())));

    final int cds = clampedToInt(frame.getStackItem(opCode.callCdsStackIndex()));
    setExponentLog(BigInteger.valueOf(computeExponentLog(metadata, cds)));
  }

  @Override
  public void callExoModulesAndSetOutputs(Add add, Mod mod, Wcp wcp) {
    // row i + 0
    final OobExoCall returnAtCapacityIsZeroCall = callToIsZero(wcp, returnAtCapacity);
    exoCalls.add(returnAtCapacityIsZeroCall);
    final boolean returnAtCapacityIsZero = bytesToBoolean(returnAtCapacityIsZeroCall.result());

    // row i + 1
    final OobExoCall exponentLogIsZeroCall = callToIsZero(wcp, bigIntegerToBytes(exponentLog));
    exoCalls.add(exponentLogIsZeroCall);
    final boolean exponentLogIsZero = bytesToBoolean(exponentLogIsZeroCall.result());

    final int maxMbsBbsValue =
        (metadata.tracedIsWithinBounds(MODEXP_XBS_CASE_BBS)
                && metadata.tracedIsWithinBounds(MODEXP_XBS_CASE_MBS))
            ? max(metadata.mbsInt(), metadata.bbsInt())
            : 0;
    setMaxMbsBbs(maxMbsBbsValue);

    // row i + 2
    final OobExoCall ceilngOfMaxDividedBy8Call =
        callToDIV(mod, Bytes.ofUnsignedInt(maxMbsBbs + 7), Bytes.ofUnsignedInt(8));
    exoCalls.add(ceilngOfMaxDividedBy8Call);
    final BigInteger ceilingOfMaxDividedBy8 =
        ceilngOfMaxDividedBy8Call.result().toUnsignedBigInteger();
    final BigInteger fOfMax = ceilingOfMaxDividedBy8.multiply(ceilingOfMaxDividedBy8);
    final int fOfMaxInteger = fOfMax.intValue();

    // row i + 3
    final OobExoCall compareMaxMbsBbsTo32Call =
        callToLT(wcp, Bytes.ofUnsignedShort(32), Bytes.ofUnsignedShort(maxMbsBbs));
    exoCalls.add(compareMaxMbsBbsTo32Call);
    final boolean wordCostDominates = bytesToBoolean(compareMaxMbsBbsTo32Call.result());

    final int multiplicationComplexity = wordCostDominates ? 2 * fOfMaxInteger : 16;
    final int iterationCountOr1 = exponentLogIsZero ? 1 : exponentLog.intValue();
    final int rawCost = multiplicationComplexity * iterationCountOr1;

    // row i + 4
    final OobExoCall rawCostMinModexpCostComparisonCall =
        callToLT(wcp, Bytes.ofUnsignedInt(rawCost), Bytes.ofUnsignedInt(GAS_CONST_MODEXP_EIP_7823));
    exoCalls.add(rawCostMinModexpCostComparisonCall);
    final boolean rawCostIsLessThanMinModexpCost =
        bytesToBoolean(rawCostMinModexpCostComparisonCall.result());
    final int precompileCostInt =
        rawCostIsLessThanMinModexpCost ? GAS_CONST_MODEXP_EIP_7823 : rawCost;

    // row i + 5
    final Bytes precompileCost = Bytes.ofUnsignedInt(precompileCostInt);
    final OobExoCall ramSuccessCall = callToLT(wcp, callGas, precompileCost);
    exoCalls.add(ramSuccessCall);
    setRamSuccess(!bytesToBoolean(ramSuccessCall.result()));

    final BigInteger returnGas =
        ramSuccess
            ? callGas.toUnsignedBigInteger().subtract(precompileCost.toUnsignedBigInteger())
            : BigInteger.ZERO;
    setReturnGas(returnGas);

    setReturnAtCapacityNonZero(!returnAtCapacityIsZero);
  }

  @Override
  public Trace.Oob trace(Trace.Oob trace) {
    return trace
        .isModexpPricing(true)
        .oobInst(OOB_INST_MODEXP_PRICING)
        .data1(callGas)
        .data3(returnAtCapacity.trimLeadingZeros())
        .data4(booleanToBytes(ramSuccess))
        .data5(bigIntegerToBytes(returnGas))
        .data6(bigIntegerToBytes(exponentLog))
        .data7(Bytes.ofUnsignedInt(maxMbsBbs))
        .data8(booleanToBytes(returnAtCapacityNonZero));
  }

  @Override
  public Trace.Hub trace(Trace.Hub trace) {
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

  @Override
  public int ctMax() {
    return CT_MAX_MODEXP_PRICING;
  }
}
