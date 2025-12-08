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
import static net.consensys.linea.zktracer.Trace.Oob.G_QUADDIVISOR;
import static net.consensys.linea.zktracer.module.oob.OobExoCall.*;
import static net.consensys.linea.zktracer.types.Conversions.*;

import java.math.BigInteger;

import net.consensys.linea.zktracer.module.add.Add;
import net.consensys.linea.zktracer.module.hub.precompiles.modexpMetadata.ModexpMetadata;
import net.consensys.linea.zktracer.module.mod.Mod;
import net.consensys.linea.zktracer.module.oob.OobExoCall;
import net.consensys.linea.zktracer.module.wcp.Wcp;
import org.apache.tuweni.bytes.Bytes;

public class LondonModexpPricingOobCall extends ModexpPricingOobCall {

  public LondonModexpPricingOobCall(ModexpMetadata metadata, long calleeGas) {
    super(metadata, calleeGas);
  }

  @Override
  public void callExoModulesAndSetOutputs(Add add, Mod mod, Wcp wcp) {
    final int maxMbsBbs = max(metadata.mbsInt(), metadata.bbsInt());
    setMaxMbsBbs(maxMbsBbs);

    // row i
    final OobExoCall returnAtCapacityIsZeroCall = callToIsZero(wcp, returnAtCapacity);
    exoCalls.add(returnAtCapacityIsZeroCall);
    final boolean returnAtCapacityIsZero = bytesToBoolean(returnAtCapacityIsZeroCall.result());

    // row i + 1
    final OobExoCall exponentLogIsZeroCall = callToIsZero(wcp, bigIntegerToBytes(exponentLog));
    exoCalls.add(exponentLogIsZeroCall);
    final boolean exponentLogIsZero = bytesToBoolean(exponentLogIsZeroCall.result());

    // row i + 2
    final OobExoCall ceilngOfMaxDividedBy8Call =
        callToDIV(mod, Bytes.ofUnsignedInt(maxMbsBbs + 7), Bytes.ofUnsignedInt(8));
    exoCalls.add(ceilngOfMaxDividedBy8Call);
    final BigInteger ceilingOfMaxDividedBy8 =
        ceilngOfMaxDividedBy8Call.result().toUnsignedBigInteger();
    final BigInteger fOfMax = ceilingOfMaxDividedBy8.multiply(ceilingOfMaxDividedBy8);

    // row i + 3
    final BigInteger bigNumerator = exponentLogIsZero ? fOfMax : fOfMax.multiply(exponentLog);
    final OobExoCall bigQuotientCall =
        callToDIV(mod, bigIntegerToBytes(bigNumerator), Bytes.ofUnsignedInt(G_QUADDIVISOR));
    exoCalls.add(bigQuotientCall);
    final Bytes bigQuotient = bigQuotientCall.result();

    // row i + 4
    final OobExoCall bigQuotientLT200Call = callToLT(wcp, bigQuotient, Bytes.ofUnsignedInt(200));
    exoCalls.add(bigQuotientLT200Call);
    final boolean bigQuotientLT200 = bytesToBoolean(bigQuotientLT200Call.result());

    // row i + 5
    final Bytes precompileCost = bigQuotientLT200 ? Bytes.ofUnsignedInt(200) : bigQuotient;
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
