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

package net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.common.shaRipId;

import static net.consensys.linea.zktracer.Trace.*;

import java.math.BigInteger;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.common.CommonPrecompileOobCall;

public abstract class ShaRipIdOobCall extends CommonPrecompileOobCall {
  protected ShaRipIdOobCall(BigInteger calleeGas, int oobInst) {
    super(calleeGas, oobInst);
  }

  abstract long factor();

  @Override
  public void setOutputs() {
    super.setOutputs();

    // TODO:WTF there are no ceil method in BigInt ??
    final BigInteger q = getCds().toUnsignedBigInteger().divide(BigInteger.valueOf(WORD_SIZE));
    final short r = getCds().toUnsignedBigInteger().mod(BigInteger.valueOf(WORD_SIZE)).shortValue();
    final BigInteger ceil = r == 0 ? q : q.add(BigInteger.ONE);
    final BigInteger precompileCost =
        (BigInteger.valueOf(5).add(ceil)).multiply(BigInteger.valueOf(factor()));

    // Set hubSuccess
    setHubSuccess(precompileCost.compareTo(getCalleeGas().toUnsignedBigInteger()) <= 0);

    // Set returnGas
    final BigInteger returnGas =
        isHubSuccess()
            ? getCalleeGas().toUnsignedBigInteger().subtract(precompileCost)
            : BigInteger.ZERO;
    setReturnGas(returnGas);
  }
}
