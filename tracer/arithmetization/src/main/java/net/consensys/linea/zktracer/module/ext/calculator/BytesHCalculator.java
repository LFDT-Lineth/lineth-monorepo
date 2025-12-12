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

package net.consensys.linea.zktracer.module.ext.calculator;

import static net.consensys.linea.zktracer.module.Util.getBit;
import static net.consensys.linea.zktracer.module.Util.getOverflow;
import static net.consensys.linea.zktracer.module.Util.multiplyRange;

import net.consensys.linea.zktracer.bytestheta.BaseTheta;
import net.consensys.linea.zktracer.bytestheta.BytesArray;
import org.apache.tuweni.units.bigints.UInt256;

/**
 * A utility class for computing the Hs and overflow values for extended modular arithmetic
 * operations.
 */
public class BytesHCalculator {

  /**
   * Sets the Hs array and returns the overflow values for the given arguments.
   *
   * @param aBytes the aBytes value.
   * @param bBytes the bBytes value.
   * @param hBytes the hBytes value.
   * @return the overflow values.
   */
  static boolean[] computeHsAndOverflowH(BytesArray aBytes, BytesArray bBytes, BytesArray hBytes) {
    boolean[] overflow = new boolean[8];

    // Calculate alpha
    long alpha = calculateAlpha(aBytes, bBytes, hBytes);
    overflow[0] = getBit(alpha, 0);

    // Calculate Beta
    long beta = calculateBeta(aBytes, bBytes, hBytes);
    overflow[1] = getBit(beta, 0);
    overflow[2] = getBit(beta, 1);

    // Calculate gamma
    long gamma = calculateGamma(aBytes, bBytes, hBytes);
    overflow[3] = getBit(gamma, 0);

    return overflow;
  }

  /**
   * Calculates the alpha value for the given aBytes, bBytes, and hBytes values.
   *
   * @param aBytes the aBytes value.
   * @param bBytes the bBytes value.
   * @param hBytes the hBytes value.
   * @return the alpha value.
   */
  private static long calculateAlpha(BytesArray aBytes, BytesArray bBytes, BytesArray hBytes) {
    UInt256 sum = multiplyRange(aBytes.getBytesRange(0, 1), bBytes.getBytesRange(0, 1));
    var truc = BaseTheta.fromBytes32(sum);
    hBytes.set(0, truc.get(0));
    hBytes.set(1, truc.get(1));
    return getOverflow(sum, 1, "alpha OOB");
  }

  /**
   * Calculates the beta value for the given aBytes, bBytes, and hBytes values.
   *
   * @param aBytes the aBytes value.
   * @param bBytes the bBytes value.
   * @param hBytes the hBytes value.
   * @return the beta value.
   */
  private static long calculateBeta(BytesArray aBytes, BytesArray bBytes, BytesArray hBytes) {
    UInt256 sum = multiplyRange(aBytes.getBytesRange(0, 3), bBytes.getBytesRange(0, 3));
    var truc = BaseTheta.fromBytes32(sum);
    hBytes.set(2, truc.get(0));
    hBytes.set(3, truc.get(1));
    return getOverflow(sum, 3, "beta OOB");
  }

  /**
   * Calculates the gamma value for the given aBytes, bBytes, and hBytes values.
   *
   * @param aBytes the aBytes value.
   * @param bBytes the bBytes value.
   * @param hBytes the hBytes value.
   * @return the gamma value.
   */
  private static long calculateGamma(BytesArray aBytes, BytesArray bBytes, BytesArray hBytes) {
    UInt256 sum = multiplyRange(aBytes.getBytesRange(2, 3), bBytes.getBytesRange(2, 3));
    var truc = BaseTheta.fromBytes32(sum);
    hBytes.set(4, truc.get(0));
    hBytes.set(5, truc.get(1));
    return getOverflow(sum, 1, "gamma OOB");
  }
}
