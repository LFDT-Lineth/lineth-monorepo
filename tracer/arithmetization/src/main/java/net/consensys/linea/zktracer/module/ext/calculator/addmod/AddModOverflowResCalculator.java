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

import static net.consensys.linea.zktracer.module.Util.getBit;
import static net.consensys.linea.zktracer.module.Util.getOverflow;

import net.consensys.linea.zktracer.bytestheta.BaseBytes;
import org.apache.tuweni.units.bigints.UInt256;

/**
 * A utility class to calculate overflow values for the AddMod operation. The class provides methods
 * for calculating overflow values for two input arguments of the AddMod operation. The calculation
 * involves calculating the sum of the two input arguments and determining if the sum overflows the
 * bit size of the operands. The method returns a boolean array of size 2 that contains the overflow
 * values for lambda and mu respectively.
 */
public class AddModOverflowResCalculator {
  /**
   * Calculates the overflow values of the result for two input arguments of the AddMod operation.
   *
   * @param arg1 the first input argument of the AddMod operation.
   * @param arg2 the second input argument of the AddMod operation.
   * @return a boolean array of size 2 that contains the overflow values for lambda and mu
   *     respectively.
   */
  static boolean[] calculateOverflow(BaseBytes arg1, BaseBytes arg2) {
    boolean[] overflowRes = new boolean[8];

    long lambda = calculateLambda(arg1, arg2);

    long mu = calculateMu(lambda, arg1, arg2);

    overflowRes[0] = getBit(lambda, 0);
    overflowRes[1] = getBit(mu, 0);

    return overflowRes;
  }

  /**
   * Calculates the lambda value.
   *
   * @param arg1 the first input argument of the AddMod operation.
   * @param arg2 the second input argument of the AddMod operation.
   * @return the lambda value.
   */
  private static long calculateLambda(BaseBytes arg1, BaseBytes arg2) {
    UInt256 sum = UInt256.fromBytes(arg1.getLow()).add(UInt256.fromBytes(arg2.getLow()));

    return getOverflow(sum, 1, "lambda out of range (ADDMOD)");
  }

  /**
   * Calculates the mu value.
   *
   * @param lambda the lambda value.
   * @param arg1 the first input argument of the AddMod operation.
   * @param arg2 the second input argument of the AddMod operation.
   * @return the mu value.
   */
  private static long calculateMu(long lambda, BaseBytes arg1, BaseBytes arg2) {
    UInt256 sum = UInt256.valueOf(lambda);
    sum = sum.add(UInt256.fromBytes(arg1.getHigh()));
    sum = sum.add(UInt256.fromBytes(arg2.getHigh()));

    return getOverflow(sum, 3, "mu out of range (ADDMOD)");
  }
}
