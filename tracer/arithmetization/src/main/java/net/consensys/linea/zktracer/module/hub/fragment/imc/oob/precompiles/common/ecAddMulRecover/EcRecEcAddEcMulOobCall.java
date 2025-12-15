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

package net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.common.ecAddMulRecover;

import static net.consensys.linea.zktracer.module.oob.OobExoCall.callToLT;
import static net.consensys.linea.zktracer.types.Conversions.bytesToBoolean;

import java.math.BigInteger;

import net.consensys.linea.zktracer.module.add.Add;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.common.CommonPrecompileOobCall;
import net.consensys.linea.zktracer.module.mod.Mod;
import net.consensys.linea.zktracer.module.oob.OobExoCall;
import net.consensys.linea.zktracer.module.wcp.Wcp;
import org.apache.tuweni.bytes.Bytes;

public abstract class EcRecEcAddEcMulOobCall extends CommonPrecompileOobCall {
  protected EcRecEcAddEcMulOobCall(BigInteger calleeGas, int oobCall) {
    super(calleeGas, oobCall);
  }

  abstract long precompileLongCost();

  @Override
  public void callExoModulesAndSetOutputs(Add add, Mod mod, Wcp wcp) {
    super.callExoModulesAndSetOutputs(add, mod, wcp);
    // row i + 2
    final OobExoCall insufficientGasCall =
        callToLT(wcp, getCalleeGas(), Bytes.ofUnsignedLong(precompileLongCost()));
    exoCalls.add(insufficientGasCall);
    final boolean insufficientGas = bytesToBoolean(insufficientGasCall.result());

    // Set hubSuccess
    final boolean hubSuccess = !insufficientGas;
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
}
