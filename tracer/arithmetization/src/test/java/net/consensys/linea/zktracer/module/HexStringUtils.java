package net.consensys.linea.zktracer.module;

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

import java.math.BigInteger;

public class HexStringUtils {

  /**
   * Shifts the given hexadecimal value to the right by the specified number of bits.
   *
   * @param value the hexadecimal value to be shifted
   * @param n the number of bits to shift to the right
   * @return the resulting hexadecimal value after the shift
   */
  public static String rightShift(String value, int n) {
    return new BigInteger(value, 16).shiftRight(n).toString(16);
  }

  /**
   * Shifts the given hexadecimal value to the left by the specified number of bits.
   *
   * @param value the hexadecimal value to be shifted
   * @param n the number of bits to shift to the left
   * @return the resulting hexadecimal value after the shift
   */
  public static String leftShift(String value, int n) {
    return new BigInteger(value, 16).shiftLeft(n).toString(16);
  }

  /**
   * Performs a bitwise AND operation on the given hexadecimal value and mask.
   *
   * @param value the hexadecimal value
   * @param mask the hexadecimal mask
   * @return the resulting hexadecimal value after the AND operation
   */
  public static String and(String value, String mask) {
    return new BigInteger(value, 16).and(new BigInteger(mask, 16)).toString(16);
  }

  /**
   * Performs a bitwise XOR operation on the given hexadecimal value and mask.
   *
   * @param value the hexadecimal value
   * @param mask the hexadecimal mask
   * @return the resulting hexadecimal value after the XOR operation
   */
  public static String xor(String value, String mask) {
    return new BigInteger(value, 16).xor(new BigInteger(mask, 16)).toString(16);
  }

  /**
   * Generates a hexadecimal even value with the specified number of trailing zeros (other digits
   * are ones).
   *
   * @param nTrailingZeros the number of trailing zeros
   * @return the resulting hexadecimal value
   */
  public static String even(int nTrailingZeros) {
    return new BigInteger("1".repeat(256 - nTrailingZeros) + "0".repeat(nTrailingZeros), 2)
        .toString(16);
  }

  /**
   * Generates a hexadecimal odd value with the specified number of leading zeros (other digits are
   * zeros).
   *
   * @param nLeadingZeros the number of leading zeros
   * @return the resulting hexadecimal value
   */
  public static String odd(int nLeadingZeros) {
    return new BigInteger("0".repeat(nLeadingZeros) + "1".repeat(256 - nLeadingZeros), 2)
        .toString(16);
  }

  /**
   * Generates a hexadecimal string with `k` bytes, where the first `k-l` bytes are "00" and the
   * last `l` bytes are "ff".
   *
   * @param k the total number of bytes in the resulting string
   * @param l the number of trailing "ff" bytes
   * @return the resulting hexadecimal string
   */
  public static String trailingFF(int k, int l) {
    return "00".repeat(k - l) + "ff".repeat(l);
  }

  /**
   * Generates a hexadecimal string with `a` leading "00" bytes, followed by `b` "ff" bytes, and `c`
   * trailing "00" bytes.
   *
   * @param a the number of leading "00" bytes
   * @param b the number of "ff" bytes in the middle
   * @param c the number of trailing "00" bytes
   * @return the resulting hexadecimal string
   */
  public static String middleFF(int a, int b, int c) {
    return "00".repeat(a) + "ff".repeat(b) + "00".repeat(c);
  }
}
