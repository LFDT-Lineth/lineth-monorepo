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
 * A utility class for computing the Is and overflow values for extended modular arithmetic
 * operations.
 */
public class BytesICalculator {
  /**
   * Computes the Is array and overflow values for the given arguments.
   *
   * @param qBytes the aBytes value.
   * @param cBytes the bBytes value.
   * @param iBytes the hBytes value.
   * @return the overflow values.
   */
  static boolean[] computeIsAndOverflowI(BytesArray qBytes, BytesArray cBytes, BytesArray iBytes) {
    boolean[] overflowI = new boolean[8];

    setLastByte(qBytes, cBytes, iBytes);

    long sigma = calculateSigma(qBytes, cBytes, iBytes);
    overflowI[0] = getBit(sigma, 0);

    long tau = calculateTau(qBytes, cBytes, iBytes);
    overflowI[1] = getBit(tau, 0);
    overflowI[2] = getBit(tau, 1);

    long rho = calculateRho(qBytes, cBytes, iBytes);
    overflowI[3] = getBit(rho, 0);
    overflowI[4] = getBit(rho, 1);

    return overflowI;
  }

  /**
   * Calculates the value of sigma for the given set of BytesArrays.
   *
   * @param qBytes the qBytes value.
   * @param cBytes the cBytes value.
   * @param iBytes the iBytes value.
   */
  private static long calculateSigma(BytesArray qBytes, BytesArray cBytes, BytesArray iBytes) {
    UInt256 sumSigma = multiplyRange(qBytes.getBytesRange(0, 1), cBytes.getBytesRange(0, 1));
    BaseTheta thetaSigma = BaseTheta.fromBytes32(sumSigma);
    iBytes.set(0, thetaSigma.get(0));
    iBytes.set(1, thetaSigma.get(1));
    return getOverflow(sumSigma, 1, "sigma OOB");
  }

  /**
   * Calculates the value of tau for the given set of BytesArrays.
   *
   * @param qBytes the qBytes value.
   * @param cBytes the cBytes value.
   * @param iBytes the iBytes value.
   * @return The computed value of tau
   */
  private static long calculateTau(BytesArray qBytes, BytesArray cBytes, BytesArray iBytes) {
    UInt256 sumTau = multiplyRange(qBytes.getBytesRange(0, 3), cBytes.getBytesRange(0, 3));
    BaseTheta thetaTau = BaseTheta.fromBytes32(sumTau);
    iBytes.set(2, thetaTau.get(0));
    iBytes.set(3, thetaTau.get(1));
    return getOverflow(UInt256.fromBytes(sumTau), 3, "tau OOB");
  }

  /**
   * Calculates the value of rho for the given set of BytesArrays.
   *
   * @param qBytes the qBytes value.
   * @param cBytes the cBytes value.
   * @param iBytes the iBytes value.
   * @return The computed value of rho
   */
  private static long calculateRho(BytesArray qBytes, BytesArray cBytes, BytesArray iBytes) {

    UInt256 sumRho = multiplyRange(qBytes.getBytesRange(2, 5), cBytes.getBytesRange(0, 3));
    BaseTheta thetaRho = BaseTheta.fromBytes32(sumRho);
    iBytes.set(4, thetaRho.get(0));
    iBytes.set(5, thetaRho.get(1));
    return getOverflow(sumRho, 3, "rho OOB");
  }

  /**
   * Set the value of the last byte for the given set of BytesArrays.
   *
   * @param qBytes the qBytes value.
   * @param cBytes the cBytes value.
   * @param iBytes the iBytes value.
   */
  private static void setLastByte(BytesArray qBytes, BytesArray cBytes, BytesArray iBytes) {
    UInt256 lastSum = multiplyRange(qBytes.getBytesRange(4, 7), cBytes.getBytesRange(0, 3));
    BaseTheta lastTheta = BaseTheta.fromBytes32(lastSum);
    iBytes.set(6, lastTheta.get(0));
  }
}
