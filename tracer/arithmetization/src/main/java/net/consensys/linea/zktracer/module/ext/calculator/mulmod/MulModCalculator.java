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

package net.consensys.linea.zktracer.module.ext.calculator.mulmod;

import net.consensys.linea.zktracer.bytestheta.BaseBytes;
import net.consensys.linea.zktracer.bytestheta.BytesArray;
import net.consensys.linea.zktracer.module.ext.calculator.AbstractExtCalculator;
import org.apache.tuweni.bytes.Bytes32;
import org.apache.tuweni.units.bigints.UInt256;

public class MulModCalculator extends AbstractExtCalculator {
  @Override
  public UInt256 computeResult(Bytes32 arg1, Bytes32 arg2, Bytes32 modulo) {
    if (modulo.isZero()) {
      return UInt256.ZERO;
    }

    return UInt256.fromBytes(arg1).multiplyMod(UInt256.fromBytes(arg2), UInt256.fromBytes(modulo));
  }

  /**
   * Computes the product of two Bytes32 arguments and returns the result as a BytesArray.
   *
   * @param arg1 The first Bytes32 argument.
   * @param arg2 The second Bytes32 argument.
   * @return The product of the two arguments as a BytesArray.
   */
  @Override
  public BytesArray computeJs(Bytes32 arg1, Bytes32 arg2) {
    return MulModBytesJCalculator.computeJs(arg1, arg2);
  }

  /**
   * Computes the quotient of the product of arg1 and arg2 divided by arg3, all of Bytes32 type, and
   * returns the result as a BytesArray.
   *
   * @param arg1 The first Bytes32 argument.
   * @param arg2 The second Bytes32 argument.
   * @param arg3 The third Bytes32 argument.
   * @return The quotient of the product of arg1 and arg2 divided by arg3 as a BytesArray.
   */
  @Override
  public BytesArray computeQs(Bytes32 arg1, Bytes32 arg2, Bytes32 arg3) {
    return MulModBytesQCalculator.computeQs(arg1, arg2, arg3);
  }

  /**
   * Computes the overflow result for the given arguments.
   *
   * @param arg1 the arg1 value.
   * @param arg2 the arg2 value.
   * @param aBytes the aBytes value.
   * @param bBytes the bBytes value.
   * @param hBytes the hBytes value.
   * @param alpha the alpha value.
   * @param beta the beta value.
   * @return the overflow result.
   */
  @Override
  public boolean[] computeOverflowRes(
      BaseBytes arg1,
      BaseBytes arg2,
      BytesArray aBytes,
      BytesArray bBytes,
      BytesArray hBytes,
      UInt256 alpha,
      UInt256 beta) {
    return MulModOverflowResCalculator.calculateOverflow(aBytes, bBytes, hBytes, alpha, beta);
  }
}
