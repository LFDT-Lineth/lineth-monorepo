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

package net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.common.postCancun.fixedSizeFixedGasCost;

import static net.consensys.linea.zktracer.module.oob.OobExoCall.callToEQ;
import static net.consensys.linea.zktracer.module.oob.OobExoCall.callToLT;
import static net.consensys.linea.zktracer.types.Conversions.bytesToBoolean;

import java.math.BigInteger;

import net.consensys.linea.zktracer.module.add.Add;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.common.CommonPrecompileOobCall;
import net.consensys.linea.zktracer.module.mod.Mod;
import net.consensys.linea.zktracer.module.oob.OobExoCall;
import net.consensys.linea.zktracer.module.wcp.Wcp;
import org.apache.tuweni.bytes.Bytes;

public abstract class FixedSizeFixedGasCostOobCall extends CommonPrecompileOobCall {
  protected FixedSizeFixedGasCostOobCall(BigInteger calleeGas, int oobInst) {
    super(calleeGas, oobInst);
  }

  abstract long precompileExpectedCds();

  abstract long precompileLongCost();

  @Override
  public void callExoModulesAndSetOutputs(Add add, Mod mod, Wcp wcp) {
    super.callExoModulesAndSetOutputs(add, mod, wcp);

    // row i + 2
    final OobExoCall validCdsCall =
        callToEQ(wcp, getCds().toBytes(), Bytes.ofUnsignedLong(precompileExpectedCds()));
    exoCalls.add(validCdsCall);
    final boolean validCds = bytesToBoolean(validCdsCall.result());

    // row i + 3
    final OobExoCall insufficientGasCall =
        callToLT(wcp, getCalleeGas(), Bytes.ofUnsignedLong(precompileLongCost()));
    exoCalls.add(insufficientGasCall);
    final boolean sufficientGas = !bytesToBoolean(insufficientGasCall.result());

    // Set hubSuccess
    final boolean hubSuccess = hubSuccess(sufficientGas, validCds);
    setHubSuccess(hubSuccess);

    // Set returnGas
    final BigInteger returnGas =
        hubSuccess
            ? getCalleeGas()
                .toUnsignedBigInteger()
                .subtract(BigInteger.valueOf(precompileLongCost()))
            : BigInteger.ZERO;
    setReturnGas(returnGas);
  }

  boolean hubSuccess(boolean sufficientGas, boolean validCds) {
    return sufficientGas && validCds;
  }
}
