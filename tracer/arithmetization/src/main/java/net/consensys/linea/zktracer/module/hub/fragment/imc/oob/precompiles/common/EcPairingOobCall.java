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

package net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.common;

import static net.consensys.linea.zktracer.Trace.OOB_INST_ECPAIRING;
import static net.consensys.linea.zktracer.Trace.Oob.CT_MAX_ECPAIRING;
import static net.consensys.linea.zktracer.module.oob.OobExoCall.*;
import static net.consensys.linea.zktracer.types.Conversions.bigIntegerToBytes;
import static net.consensys.linea.zktracer.types.Conversions.bytesToBoolean;

import java.math.BigInteger;

import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.add.Add;
import net.consensys.linea.zktracer.module.mod.Mod;
import net.consensys.linea.zktracer.module.oob.OobExoCall;
import net.consensys.linea.zktracer.module.wcp.Wcp;
import org.apache.tuweni.bytes.Bytes;

public class EcPairingOobCall extends CommonPrecompileOobCall {
  public EcPairingOobCall(BigInteger calleeGas) {
    super(calleeGas, OOB_INST_ECPAIRING);
  }

  @Override
  public void callExoModulesAndSetOutputs(Add add, Mod mod, Wcp wcp) {
    super.callExoModulesAndSetOutputs(add, mod, wcp);

    // row i + 2
    final OobExoCall remainderCall = callToMOD(mod, cds, Bytes.ofUnsignedLong(192));
    exoCalls.add(remainderCall);
    final Bytes remainder = remainderCall.result();

    // row i + 3
    final OobExoCall isMultipleOf192Call = callToIsZero(wcp, remainder);
    exoCalls.add(isMultipleOf192Call);
    final boolean isMultipleOf192 = bytesToBoolean(isMultipleOf192Call.result());

    final Bytes precompileCost =
        isMultipleOf192
            ? bigIntegerToBytes(
                BigInteger.valueOf(45000)
                    .add(
                        BigInteger.valueOf(34000)
                            .multiply(cds.toUnsignedBigInteger().divide(BigInteger.valueOf(192)))))
            : Bytes.of(0);

    // row i + 4
    final OobExoCall insufficientGasCall =
        isMultipleOf192 ? callToLT(wcp, calleeGas, precompileCost) : noCall();
    exoCalls.add(insufficientGasCall);
    final boolean insufficientGas = bytesToBoolean(insufficientGasCall.result());

    // Set hubSuccess
    final boolean hubSuccess = isMultipleOf192 && !insufficientGas;
    setHubSuccess(hubSuccess);

    // Set returnGas
    final BigInteger returnGas =
        hubSuccess
            ? calleeGas.toUnsignedBigInteger().subtract(precompileCost.toUnsignedBigInteger())
            : BigInteger.ZERO;
    setReturnGas(returnGas);
  }

  @Override
  protected void traceOobInstructionInOob(Trace.Oob trace) {
    trace.isEcpairing(true).oobInst(OOB_INST_ECPAIRING);
  }

  @Override
  protected void traceOobInstructionInHub(Trace.Hub trace) {
    trace.pMiscOobInst(OOB_INST_ECPAIRING);
  }

  @Override
  public int ctMax() {
    return CT_MAX_ECPAIRING;
  }
}
