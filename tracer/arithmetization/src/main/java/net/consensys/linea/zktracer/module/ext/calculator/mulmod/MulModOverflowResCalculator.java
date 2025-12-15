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

import static net.consensys.linea.zktracer.module.Util.getBit;
import static net.consensys.linea.zktracer.module.Util.getOverflow;
import static net.consensys.linea.zktracer.module.Util.multiplyRange;

import net.consensys.linea.zktracer.bytestheta.BytesArray;
import org.apache.tuweni.units.bigints.UInt256;

/**
 * A utility class for computing the Result overflow value for extended modular arithmetic
 * operations. (MulMod)
 */
public class MulModOverflowResCalculator {
  static boolean[] calculateOverflow(
      BytesArray aBytes, BytesArray bBytes, BytesArray hBytes, UInt256 alpha, UInt256 beta) {

    boolean[] overflowRes = new boolean[8];
    // Calculate lambda
    long lambda = calculateLambda(aBytes, bBytes, hBytes);

    // Calculate mu
    long mu = calculateMu(lambda, aBytes, bBytes, hBytes, alpha);
    // Store results in the boolean array
    overflowRes[0] = getBit(lambda, 0);
    overflowRes[1] = getBit(mu, 0);
    overflowRes[2] = getBit(mu, 1);

    // Calculate nu
    long nu = calculateNu(mu, aBytes, bBytes, hBytes, beta);
    // Store results in the boolean array
    overflowRes[3] = getBit(nu, 0);
    overflowRes[4] = getBit(nu, 1);

    // Return the boolean array containing the overflow results
    return overflowRes;
  }

  /**
   * Calculates lambda based on input parameters.
   *
   * @param aBytes the first input parameter as a BaseTheta object
   * @param bBytes the second input parameter as a BaseTheta object
   * @param hBytes the third input parameter as a BytesArray object
   * @return the calculated lambda value as a long
   */
  private static long calculateLambda(BytesArray aBytes, BytesArray bBytes, BytesArray hBytes) {
    var sum = multiplyRange(aBytes.getBytesRange(0, 0), bBytes.getBytesRange(0, 0));
    sum = sum.add(UInt256.valueOf(hBytes.get(0).toUnsignedBigInteger().shiftLeft(64)));
    return getOverflow(sum, 1, "lambda out of range (MULMOD)");
  }

  /**
   * Calculates mu based on input parameters.
   *
   * @param lambda the lambda value as a long
   * @param aBytes the first input parameter as a BaseTheta object
   * @param bBytes the second input parameter as a BaseTheta object
   * @param hBytes the third input parameter as a BytesArray object
   * @param alpha the fourth input parameter as a UInt256 object
   * @return the calculated mu value as a long
   */
  private static long calculateMu(
      long lambda, BytesArray aBytes, BytesArray bBytes, BytesArray hBytes, UInt256 alpha) {
    var sum = UInt256.valueOf(lambda);
    sum = sum.add(UInt256.fromBytes(hBytes.get(1)));
    sum = sum.add(UInt256.valueOf(alpha.toUnsignedBigInteger().shiftLeft(64)));
    sum = sum.add((UInt256.fromBytes(aBytes.get(2)).multiply(UInt256.fromBytes(bBytes.get(0)))));
    sum = sum.add((UInt256.fromBytes(aBytes.get(1)).multiply(UInt256.fromBytes(bBytes.get(1)))));
    sum = sum.add((UInt256.fromBytes(aBytes.get(0)).multiply(UInt256.fromBytes(bBytes.get(2)))));
    sum = sum.add(UInt256.valueOf(hBytes.get(2).toUnsignedBigInteger().shiftLeft(64)));
    return getOverflow(sum, 3, "mu out of range (MULMOD)");
  }

  /**
   * Calculates nu based on input parameters.
   *
   * @param mu the mu value as a long
   * @param aBytes the first input parameter as a BaseTheta object
   * @param bBytes the second input parameter as a BaseTheta object
   * @param hBytes the third input parameter as a BytesArray object
   * @param beta the fifth input parameter as a UInt256 object
   * @return the calculated nu value as a long
   */
  private static long calculateNu(
      long mu, BytesArray aBytes, BytesArray bBytes, BytesArray hBytes, UInt256 beta) {
    var sum = UInt256.valueOf(mu);
    sum = sum.add(UInt256.fromBytes(hBytes.get(3)));
    sum = sum.add(UInt256.valueOf(beta.toUnsignedBigInteger().shiftLeft(64)));
    sum = sum.add(multiplyRange(bBytes.getBytesRange(1, 3), aBytes.getBytesRange(1, 3)));
    sum = sum.add(UInt256.valueOf(hBytes.get(4).toUnsignedBigInteger().shiftLeft(64)));
    return getOverflow(sum, 3, "nu out of range (MULMOD)");
  }
}
