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

package net.consensys.linea.zktracer.module;

import java.math.BigInteger;
import net.consensys.linea.zktracer.bytestheta.BaseTheta;
import org.apache.tuweni.bytes.Bytes32;
import org.apache.tuweni.units.bigints.UInt256;

public class UtilCalculator {
  /**
   * Calculates the product of arg1 and arg2, both of type Bytes32, and returns the result as a
   * BigInteger.
   *
   * @param arg1 The first Bytes32 argument.
   * @param arg2 The second Bytes32 argument.
   * @return The product of arg1 and arg2 as a BigInteger.
   */
  public static BigInteger calculateProduct(Bytes32 arg1, Bytes32 arg2) {
    BigInteger arg1UInt = arg1.toUnsignedBigInteger();
    BigInteger arg2UInt = arg2.toUnsignedBigInteger();

    // Compute the product of the two unsigned BigIntegers.
    return arg1UInt.multiply(arg2UInt);
  }

  /**
   * Calculates the quotient of the given BigInteger divided by arg3, a Bytes32 argument. Returns
   * the result as a BigInteger.
   *
   * @param prod The BigInteger to divide.
   * @param arg3 The third Bytes32 argument.
   * @return The quotient of the given BigInteger and arg3 as a BigInteger.
   */
  public static BigInteger calculateQuotient(BigInteger prod, Bytes32 arg3) {
    return prod.divide(arg3.toUnsignedBigInteger());
  }

  /**
   * Converts a given BigInteger to a BaseTheta representation.
   *
   * @param quotBigInteger The BigInteger to convert.
   * @return The given BigInteger as a BaseTheta representation.
   */
  public static BaseTheta convertToBaseTheta(BigInteger quotBigInteger) {
    return new BaseTheta(UInt256.valueOf(quotBigInteger));
  }

  public static UInt256 calculateSum(Bytes32 arg1, Bytes32 arg2) {
    // Compute the sum of the two unsigned BigIntegers.
    return UInt256.fromBytes(arg1).add(UInt256.fromBytes(arg2));
  }

  public static long allButOneSixtyFourth(final long value) {
    return value - value / 64;
  }
}
