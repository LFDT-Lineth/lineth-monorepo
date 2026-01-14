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
import static net.consensys.linea.zktracer.Trace.PRECOMPILE_CALL_DATA_UNIT_SIZE___BLS_PAIRING_CHECK;

import java.math.BigInteger;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.common.CommonPrecompileOobCall;
import net.consensys.linea.zktracer.types.EWord;
import org.apache.tuweni.bytes.Bytes;

public class BlsPairingCheckOobCall extends CommonPrecompileOobCall {
  public BlsPairingCheckOobCall(BigInteger calleeGas) {
    super(calleeGas, OOB_INST_BLS_PAIRING_CHECK);
  }

  @Override
  public void setOutputs() {
    super.setOutputs();

    final Bytes remainder = getCds().mod(PRECOMPILE_CALL_DATA_UNIT_SIZE___BLS_PAIRING_CHECK);
    final boolean cdsIsMultipleOfMinBlsPairingCheckSize = remainder.isZero();

    final EWord precompileCost =
        cdsIsMultipleOfMinBlsPairingCheckSize
            ? EWord.of(
                BigInteger.valueOf(GAS_CONST_BLS_PAIRING_CHECK)
                    .add(
                        BigInteger.valueOf(GAS_CONST_BLS_PAIRING_CHECK_PAIR)
                            .multiply(
                                getCds()
                                    .toUnsignedBigInteger()
                                    .divide(
                                        BigInteger.valueOf(
                                            PRECOMPILE_CALL_DATA_UNIT_SIZE___BLS_PAIRING_CHECK)))))
            : EWord.ZERO;

    final boolean validCds = !isCdsIsZero() && cdsIsMultipleOfMinBlsPairingCheckSize;
    final boolean sufficientGas = precompileCost.compareTo(getCalleeGas()) <= 0;

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
    trace.inst(OOB_INST_BLS_PAIRING_CHECK);
  }

  @Override
  protected void traceOobInstructionInHub(Trace.Hub trace) {
    trace.pMiscOobInst(OOB_INST_BLS_PAIRING_CHECK);
  }
}
