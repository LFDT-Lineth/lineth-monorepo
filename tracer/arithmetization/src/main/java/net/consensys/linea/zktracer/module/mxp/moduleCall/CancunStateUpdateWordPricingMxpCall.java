/*
 * Copyright Consensys Software Inc.
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

package net.consensys.linea.zktracer.module.mxp.moduleCall;

import static net.consensys.linea.zktracer.TraceOsaka.Mxp.CT_MAX_UPDT_W;
import static net.consensys.linea.zktracer.module.mxp.MxpOperation.MXP_FROM_CTMAX_TO_LINECOUNT;
import static net.consensys.linea.zktracer.types.Conversions.unsignedIntToBytes;

import java.math.BigInteger;
import net.consensys.linea.zktracer.module.euc.Euc;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.mxp.MxpExoCall;
import org.apache.tuweni.bytes.Bytes;

public class CancunStateUpdateWordPricingMxpCall extends CancunStateUpdateMxpCall {

  public static final short NB_ROWS_MXP_UPDT_W =
      (short) (CT_MAX_UPDT_W + MXP_FROM_CTMAX_TO_LINECOUNT);

  public CancunStateUpdateWordPricingMxpCall(Hub hub) {
    super(hub);
    exoCalls[10] = MxpExoCall.builder().build(); // Row i + 11, initialized to default values
    computeExtraGasCost(hub.euc());
    // if state has changed, an extra gas cost is incurred
    setGasMpxFromExtraGasCost();
  }

  @Override
  public boolean isStateUpdateWordPricingScenario() {
    return true;
  }

  private void computeExtraGasCost(Euc euc) {
    // Row i + 11
    exoCalls[10] = MxpExoCall.callToEUC(euc, this.size1.lo(), unsignedIntToBytes(32));
    final Bytes numberOfWords = exoCalls[10].resultB(); // result of row i + 11
    this.extraGasCost =
        numberOfWords.toUnsignedBigInteger().multiply(BigInteger.valueOf(this.gWord)).longValue();
  }

  @Override
  public int ctMax() {
    return CT_MAX_UPDT_W;
  }
}
