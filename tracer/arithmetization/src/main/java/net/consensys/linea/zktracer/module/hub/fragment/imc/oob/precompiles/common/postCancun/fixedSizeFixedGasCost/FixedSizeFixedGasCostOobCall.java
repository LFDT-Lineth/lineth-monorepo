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

import java.math.BigInteger;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.common.CommonPrecompileOobCall;
import net.consensys.linea.zktracer.types.EWord;

public abstract class FixedSizeFixedGasCostOobCall extends CommonPrecompileOobCall {
  protected FixedSizeFixedGasCostOobCall(BigInteger calleeGas, int oobInst) {
    super(calleeGas, oobInst);
  }

  abstract long precompileExpectedCds();

  abstract long precompileLongCost();

  @Override
  public void setOutputs() {
    super.setOutputs();

    final boolean validCds = getCds().compareTo(EWord.of(precompileExpectedCds())) == 0;
    final boolean sufficientGas = getCalleeGas().compareTo(EWord.of(precompileLongCost())) >= 0;

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
