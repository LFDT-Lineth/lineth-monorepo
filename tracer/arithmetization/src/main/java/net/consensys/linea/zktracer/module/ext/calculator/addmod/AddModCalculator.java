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

package net.consensys.linea.zktracer.module.ext.calculator.addmod;

import net.consensys.linea.zktracer.bytestheta.BaseBytes;
import net.consensys.linea.zktracer.bytestheta.BytesArray;
import net.consensys.linea.zktracer.module.ext.calculator.AbstractExtCalculator;
import org.apache.tuweni.bytes.Bytes32;
import org.apache.tuweni.units.bigints.UInt256;

/** Calculator for performing extended modular arithmetic operations. */
public class AddModCalculator extends AbstractExtCalculator {

  @Override
  public UInt256 computeResult(final Bytes32 arg1, final Bytes32 arg2, final Bytes32 arg3) {
    if (arg3.isZero()) {
      return UInt256.ZERO;
    }

    return UInt256.fromBytes(arg1).addMod(UInt256.fromBytes(arg2), UInt256.fromBytes(arg3));
  }

  @Override
  public BytesArray computeJs(final Bytes32 arg1, final Bytes32 arg2) {
    return AddModBytesJCalculator.computeJs(arg1, arg2);
  }

  @Override
  public BytesArray computeQs(final Bytes32 arg1, final Bytes32 arg2, final Bytes32 arg3) {
    return AddModBytesQCalculator.computeQs(arg1, arg2, arg3);
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
      final BaseBytes arg1,
      final BaseBytes arg2,
      final BytesArray aBytes,
      final BytesArray bBytes,
      final BytesArray hBytes,
      final UInt256 alpha,
      final UInt256 beta) {
    return AddModOverflowResCalculator.calculateOverflow(arg1, arg2);
  }
}
