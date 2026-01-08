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

import static net.consensys.linea.zktracer.Trace.GAS_CONST_P256_VERIFY;
import static net.consensys.linea.zktracer.Trace.OOB_INST_P256_VERIFY;
import static net.consensys.linea.zktracer.Trace.PRECOMPILE_CALL_DATA_SIZE___P256_VERIFY;

import java.math.BigInteger;
import net.consensys.linea.zktracer.Trace;

public class P256VerifyOobCall extends FixedSizeFixedGasCostOobCall {
  public P256VerifyOobCall(BigInteger calleeGas) {
    super(calleeGas, OOB_INST_P256_VERIFY);
  }

  long precompileExpectedCds() {
    return PRECOMPILE_CALL_DATA_SIZE___P256_VERIFY;
  }

  long precompileLongCost() {
    return GAS_CONST_P256_VERIFY;
  }

  @Override
  protected void traceOobInstructionInOob(Trace.Oob trace) {
    trace.inst(OOB_INST_P256_VERIFY);
  }

  @Override
  protected void traceOobInstructionInHub(Trace.Hub trace) {
    trace.pMiscOobInst(OOB_INST_P256_VERIFY);
  }

  @Override
  public boolean getCdxFilter() {
    return getCds().toInt() == PRECOMPILE_CALL_DATA_SIZE___P256_VERIFY;
  }

  @Override
  boolean hubSuccess(boolean sufficientGas, boolean validCds) {
    return sufficientGas;
  }
}
