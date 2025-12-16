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
package net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.modexp.pricingOobCall;

import static java.lang.Math.max;
import static net.consensys.linea.zktracer.TraceOsaka.GAS_CONST_MODEXP_EIP_7823;
import static net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.modexp.ModexpXbsCase.*;
import static net.consensys.linea.zktracer.module.oob.OobExoCall.*;
import static net.consensys.linea.zktracer.types.Conversions.bigIntegerToBytes;
import static net.consensys.linea.zktracer.types.Conversions.bytesToBoolean;

import java.math.BigInteger;
import net.consensys.linea.zktracer.module.add.Add;
import net.consensys.linea.zktracer.module.hub.precompiles.modexpMetadata.ModexpMetadata;
import net.consensys.linea.zktracer.module.mod.Mod;
import net.consensys.linea.zktracer.module.oob.OobExoCall;
import net.consensys.linea.zktracer.module.wcp.Wcp;
import org.apache.tuweni.bytes.Bytes;

public class OsakaModexpPricingOobCall extends LondonModexpPricingOobCall {
  public OsakaModexpPricingOobCall(ModexpMetadata metadata, long calleeGas) {
    super(metadata, calleeGas);
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
}
