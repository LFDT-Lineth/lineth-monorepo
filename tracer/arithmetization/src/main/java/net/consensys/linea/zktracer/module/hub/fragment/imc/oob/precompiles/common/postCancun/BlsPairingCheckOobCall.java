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

package net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.common.postCancun;

import static net.consensys.linea.zktracer.Trace.GAS_CONST_BLS_PAIRING_CHECK;
import static net.consensys.linea.zktracer.Trace.GAS_CONST_BLS_PAIRING_CHECK_PAIR;
import static net.consensys.linea.zktracer.Trace.OOB_INST_BLS_PAIRING_CHECK;
import static net.consensys.linea.zktracer.Trace.Oob.CT_MAX_BLS_PAIRING_CHECK;
import static net.consensys.linea.zktracer.Trace.PRECOMPILE_CALL_DATA_UNIT_SIZE___BLS_PAIRING_CHECK;
import static net.consensys.linea.zktracer.module.oob.OobExoCall.callToIsZero;
import static net.consensys.linea.zktracer.module.oob.OobExoCall.callToLT;
import static net.consensys.linea.zktracer.module.oob.OobExoCall.callToMOD;
import static net.consensys.linea.zktracer.module.oob.OobExoCall.noCall;
import static net.consensys.linea.zktracer.types.Conversions.bigIntegerToBytes;
import static net.consensys.linea.zktracer.types.Conversions.bytesToBoolean;

import java.math.BigInteger;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.add.Add;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.common.CommonPrecompileOobCall;
import net.consensys.linea.zktracer.module.mod.Mod;
import net.consensys.linea.zktracer.module.oob.OobExoCall;
import net.consensys.linea.zktracer.module.wcp.Wcp;
import org.apache.tuweni.bytes.Bytes;

public class BlsPairingCheckOobCall extends CommonPrecompileOobCall {
  public BlsPairingCheckOobCall(BigInteger calleeGas) {
    super(calleeGas, OOB_INST_BLS_PAIRING_CHECK);
  }

  @Override
  public void callExoModulesAndSetOutputs(Add add, Mod mod, Wcp wcp) {
    super.callExoModulesAndSetOutputs(add, mod, wcp);

    // row i + 2
    final OobExoCall remainderCall =
        callToMOD(
            mod,
            getCds(),
            Bytes.ofUnsignedLong(PRECOMPILE_CALL_DATA_UNIT_SIZE___BLS_PAIRING_CHECK));
    exoCalls.add(remainderCall);
    final Bytes remainder = remainderCall.result();

    // row i + 3
    final OobExoCall cdsIsMultipleOfMinBlsPairingCheckSizeCall = callToIsZero(wcp, remainder);
    exoCalls.add(cdsIsMultipleOfMinBlsPairingCheckSizeCall);
    final boolean cdsIsMultipleOfMinBlsPairingCheckSize =
        bytesToBoolean(cdsIsMultipleOfMinBlsPairingCheckSizeCall.result());

    final Bytes precompileCost =
        cdsIsMultipleOfMinBlsPairingCheckSize
            ? bigIntegerToBytes(
                BigInteger.valueOf(GAS_CONST_BLS_PAIRING_CHECK)
                    .add(
                        BigInteger.valueOf(GAS_CONST_BLS_PAIRING_CHECK_PAIR)
                            .multiply(
                                getCds()
                                    .toUnsignedBigInteger()
                                    .divide(
                                        BigInteger.valueOf(
                                            PRECOMPILE_CALL_DATA_UNIT_SIZE___BLS_PAIRING_CHECK)))))
            : Bytes.of(0);

    final boolean validCds = !isCdsIsZero() && cdsIsMultipleOfMinBlsPairingCheckSize;

    // row i + 4
    final OobExoCall insufficientGasCall =
        validCds ? callToLT(wcp, getCalleeGas(), precompileCost) : noCall();
    exoCalls.add(insufficientGasCall);
    final boolean sufficientGas = !bytesToBoolean(insufficientGasCall.result());

    // Set hubSuccess
    final boolean hubSuccess = validCds && sufficientGas;
    setHubSuccess(hubSuccess);

    // Set returnGas
    final BigInteger returnGas =
        hubSuccess
            ? getCalleeGas().toUnsignedBigInteger().subtract(precompileCost.toUnsignedBigInteger())
            : BigInteger.ZERO;
    setReturnGas(returnGas);
  }

  @Override
  protected void traceOobInstructionInOob(Trace.Oob trace) {
    trace.isBlsPairingCheck(true).oobInst(OOB_INST_BLS_PAIRING_CHECK);
  }

  @Override
  protected void traceOobInstructionInHub(Trace.Hub trace) {
    trace.pMiscOobInst(OOB_INST_BLS_PAIRING_CHECK);
  }

  @Override
  public int ctMax() {
    return CT_MAX_BLS_PAIRING_CHECK;
  }
}
