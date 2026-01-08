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

import static net.consensys.linea.zktracer.Trace.OOB_INST_BLS_G2_MSM;
import static net.consensys.linea.zktracer.Trace.PRC_BLS_G2_MSM_MAX_DISCOUNT;
import static net.consensys.linea.zktracer.Trace.PRC_BLS_G2_MSM_MULTIPLICATION_COST;
import static net.consensys.linea.zktracer.Trace.PRECOMPILE_CALL_DATA_UNIT_SIZE___BLS_G2_MSM;

import java.math.BigInteger;
import net.consensys.linea.zktracer.Trace;

public class BlsG2MsmOobCall extends BlsMsmOobCall {
  public BlsG2MsmOobCall(BigInteger calleeGas) {
    super(calleeGas, OOB_INST_BLS_G2_MSM);
  }

  @Override
  protected void traceOobInstructionInOob(Trace.Oob trace) {
    trace.inst(OOB_INST_BLS_G2_MSM);
  }

  @Override
  protected void traceOobInstructionInHub(Trace.Hub trace) {
    trace.pMiscOobInst(OOB_INST_BLS_G2_MSM);
  }

  @Override
  int minMsmSize() {
    return PRECOMPILE_CALL_DATA_UNIT_SIZE___BLS_G2_MSM;
  }

  @Override
  int maxDiscount() {
    return PRC_BLS_G2_MSM_MAX_DISCOUNT;
  }

  @Override
  int msmMultiplicationCost() {
    return PRC_BLS_G2_MSM_MULTIPLICATION_COST;
  }
}
