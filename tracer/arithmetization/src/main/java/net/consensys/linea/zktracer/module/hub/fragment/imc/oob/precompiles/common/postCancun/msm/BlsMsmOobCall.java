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
import static net.consensys.linea.zktracer.module.tables.BlsRt.getMsmDiscount;

import java.math.BigInteger;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.common.CommonPrecompileOobCall;
import org.apache.tuweni.bytes.Bytes;

@Slf4j
public abstract class BlsMsmOobCall extends CommonPrecompileOobCall {
  protected BlsMsmOobCall(BigInteger calleeGas, int oobInst) {
    super(calleeGas, oobInst);
  }

  long precompileCost;

  abstract int minMsmSize();

  abstract int maxDiscount();

  abstract int msmMultiplicationCost();

  @Override
  public void setOutputs() {
    super.setOutputs();

    final Bytes remainder = getCds().mod(minMsmSize());
    final boolean cdsIsMultipleOfMinMsmSize = remainder.isZero();
    final int numInputs = getCds().toInt() / minMsmSize();
    final boolean validCds = !isCdsIsZero() && cdsIsMultipleOfMinMsmSize;
    final int discount = validCds ? getMsmDiscount(getOobInst(), numInputs) : 0;

    precompileCost =
        validCds
            ? BigInteger.valueOf(numInputs)
                .multiply(BigInteger.valueOf(msmMultiplicationCost()))
                .multiply(BigInteger.valueOf(discount))
                .divide(BigInteger.valueOf(PRC_BLS_MULTIPLICATION_MULTIPLIER))
                .longValueExact()
            : 0;

    setHubSuccess(validCds && precompileCost <= getCalleeGas().toLong());

    // Set returnGas
    final BigInteger returnGas =
        isHubSuccess()
            ? getCalleeGas().toUnsignedBigInteger().subtract(BigInteger.valueOf(precompileCost))
            : BigInteger.ZERO;
    setReturnGas(returnGas);
  }
}
