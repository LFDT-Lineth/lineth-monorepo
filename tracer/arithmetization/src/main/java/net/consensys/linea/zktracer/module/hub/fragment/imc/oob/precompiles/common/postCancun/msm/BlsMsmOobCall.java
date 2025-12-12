/*
 * Copyright ConsenSys Inc.
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

package net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.common.postCancun.msm;

import static net.consensys.linea.zktracer.Trace.PRC_BLS_MULTIPLICATION_MULTIPLIER;
import static net.consensys.linea.zktracer.module.oob.OobExoCall.callToBlsRefTable;
import static net.consensys.linea.zktracer.module.oob.OobExoCall.callToDIV;
import static net.consensys.linea.zktracer.module.oob.OobExoCall.callToGT;
import static net.consensys.linea.zktracer.module.oob.OobExoCall.callToIsZero;
import static net.consensys.linea.zktracer.module.oob.OobExoCall.callToLT;
import static net.consensys.linea.zktracer.module.oob.OobExoCall.callToMOD;
import static net.consensys.linea.zktracer.module.oob.OobExoCall.noCall;
import static net.consensys.linea.zktracer.types.Conversions.bigIntegerToBytes;
import static net.consensys.linea.zktracer.types.Conversions.bytesToBoolean;
import static net.consensys.linea.zktracer.types.Conversions.bytesToInt;

import java.math.BigInteger;

import net.consensys.linea.zktracer.module.add.Add;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.common.CommonPrecompileOobCall;
import net.consensys.linea.zktracer.module.mod.Mod;
import net.consensys.linea.zktracer.module.oob.OobExoCall;
import net.consensys.linea.zktracer.module.wcp.Wcp;
import org.apache.tuweni.bytes.Bytes;

public abstract class BlsMsmOobCall extends CommonPrecompileOobCall {
  protected BlsMsmOobCall(BigInteger calleeGas, int oobInst) {
    super(calleeGas, oobInst);
  }

  long precompileCost;

  abstract int minMsmSize();

  abstract int maxDiscount();

  abstract int msmMultiplicationCost();

  @Override
  public void callExoModulesAndSetOutputs(Add add, Mod mod, Wcp wcp) {
    super.callExoModulesAndSetOutputs(add, mod, wcp);

    // row i + 2
    final OobExoCall remainderCall =
        callToMOD(mod, getCds().toBytes(), Bytes.ofUnsignedLong(minMsmSize()));
    exoCalls.add(remainderCall);
    final Bytes remainder = remainderCall.result();

    // row i + 3
    final OobExoCall cdsIsMultipleOfMinMsmSizeCall = callToIsZero(wcp, remainder);
    exoCalls.add(cdsIsMultipleOfMinMsmSizeCall);
    final boolean cdsIsMultipleOfMinMsmSize =
        bytesToBoolean(cdsIsMultipleOfMinMsmSizeCall.result());

    final int numInputs = getCds().toInt() / minMsmSize();

    final boolean validCds = !isCdsIsZero() && cdsIsMultipleOfMinMsmSize;

    // i + 4
    boolean numInputsLeq128 = false;
    if (!validCds) {
      exoCalls.add(noCall());
    } else {
      final OobExoCall numInputsGt128Call =
          callToGT(wcp, Bytes.ofUnsignedLong(numInputs), Bytes.ofUnsignedInt(128));
      exoCalls.add(numInputsGt128Call);
      numInputsLeq128 = !bytesToBoolean(numInputsGt128Call.result());
    }

    // i + 5
    int discount = 0;
    if (!validCds) {
      exoCalls.add(noCall());
    } else {
      if (numInputsLeq128) {
        final OobExoCall discountCall = callToBlsRefTable(getOobInst(), numInputs);
        exoCalls.add(discountCall);
        discount = bytesToInt(discountCall.result());
      } else {
        exoCalls.add(noCall());
        discount = maxDiscount();
      }
    }

    // i + 6
    if (!validCds) {
      exoCalls.add(noCall());
    } else {
      final OobExoCall precompileCostIntegerDivisionCall =
          callToDIV(
              mod,
              bigIntegerToBytes(
                  BigInteger.valueOf(numInputs)
                      .multiply(BigInteger.valueOf(msmMultiplicationCost()))
                      .multiply(BigInteger.valueOf(discount))),
              Bytes.ofUnsignedLong(PRC_BLS_MULTIPLICATION_MULTIPLIER));
      exoCalls.add(precompileCostIntegerDivisionCall);
      precompileCost =
          precompileCostIntegerDivisionCall.result().toUnsignedBigInteger().longValue();
    }

    // i + 7
    boolean sufficientGas = false;
    if (!validCds) {
      exoCalls.add(noCall());
    } else {
      final OobExoCall insufficientGasCall =
          callToLT(wcp, getCalleeGas(), Bytes.ofUnsignedLong(precompileCost));
      exoCalls.add(insufficientGasCall);
      sufficientGas = !bytesToBoolean(insufficientGasCall.result());
    }

    // Set hubSuccess
    final boolean hubSuccess = validCds && sufficientGas;
    setHubSuccess(hubSuccess);

    // Set returnGas
    final BigInteger returnGas =
        hubSuccess
            ? getCalleeGas().toUnsignedBigInteger().subtract(BigInteger.valueOf(precompileCost))
            : BigInteger.ZERO;
    setReturnGas(returnGas);
  }
}
