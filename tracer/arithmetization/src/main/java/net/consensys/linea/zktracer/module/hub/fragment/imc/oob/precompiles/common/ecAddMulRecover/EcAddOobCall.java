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

import static net.consensys.linea.zktracer.Trace.OOB_INST_ECADD;

import java.math.BigInteger;
import net.consensys.linea.zktracer.Trace;

public class EcAddOobCall extends EcRecEcAddEcMulOobCall {
  public EcAddOobCall(BigInteger calleeGas) {
    super(calleeGas, OOB_INST_ECADD);
  }

  @Override
  long precompileLongCost() {
    return 150L;
  }

  @Override
  protected void traceOobInstructionInOob(Trace.Oob trace) {
    trace.inst(OOB_INST_ECADD);
  }

  @Override
  protected void traceOobInstructionInHub(Trace.Hub trace) {
    trace.pMiscOobInst(OOB_INST_ECADD);
  }
}
