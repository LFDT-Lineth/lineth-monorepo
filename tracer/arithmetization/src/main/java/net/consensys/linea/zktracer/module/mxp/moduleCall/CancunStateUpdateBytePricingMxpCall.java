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

import static net.consensys.linea.zktracer.types.Conversions.bigIntegerToBytes;
import static net.consensys.linea.zktracer.types.Conversions.booleanToBigInteger;

import java.math.BigInteger;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes;

public class CancunStateUpdateBytePricingMxpCall extends CancunStateUpdateMxpCall {

  public CancunStateUpdateBytePricingMxpCall(Hub hub) {
    super(hub);
    computeExtraGasCost();
    // if state has changed, an extra gas cost is incurred
    setGasMpxFromExtraGasCost();
  }

  @Override
  public boolean isStateUpdateBytePricingScenario() {
    return true;
  }

  private void computeExtraGasCost() {
    final OpCode opCode = this.opCodeData.mnemonic();
    final Bytes gasPerByte =
        (opCode == OpCode.RETURN)
            ? bigIntegerToBytes(
                booleanToBigInteger(this.deploys).multiply(BigInteger.valueOf(gByte)))
            : Bytes.of(this.gByte);
    final Bytes numberOfBytes = this.size1.lo();
    this.extraGasCost =
        numberOfBytes
            .toUnsignedBigInteger()
            .multiply(gasPerByte.toUnsignedBigInteger())
            .longValue();
  }
}
