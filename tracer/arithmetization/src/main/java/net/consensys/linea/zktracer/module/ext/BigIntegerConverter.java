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

package net.consensys.linea.zktracer.module.ext;

import java.math.BigInteger;
import java.nio.ByteBuffer;
import java.util.Arrays;

/** Supports operations of {@link BigInteger} conversions. */
public class BigIntegerConverter {

  /**
   * Converts {@link BigInteger} to a long array.
   *
   * @param bigInteger a {@link BigInteger}
   * @return a long array consisting of decomposed {@link BigInteger} bytes.
   */
  public static long[] toLongArray(BigInteger bigInteger) {
    // Convert the BigInteger to a byte array
    byte[] inputBytes = bigInteger.toByteArray();

    // If inputBytes.length is greater than 64 (512 bits), use only the least significant 64 bytes
    if (inputBytes.length > 64) {
      inputBytes = Arrays.copyOfRange(inputBytes, inputBytes.length - 64, inputBytes.length);
    }

    // Pad the inputBytes to 64 bytes (512 bits)
    byte[] byteArray = new byte[64];
    int start = byteArray.length - inputBytes.length;
    System.arraycopy(inputBytes, 0, byteArray, start, inputBytes.length);

    // Convert the byte array to an array of 8 longs
    long[] longArray = new long[8];
    ByteBuffer buffer = ByteBuffer.wrap(byteArray);
    for (int i = 0; i < 8; i++) {
      longArray[7 - i] = buffer.getLong(i * 8);
    }

    return longArray;
  }

  /**
   * Converts an array of longs to a {@link BigInteger}.
   *
   * @param longArray an array of longs.
   * @return a {@link BigInteger} number.
   */
  public static BigInteger fromLongArray(long[] longArray) {
    // Convert the array of 8 longs to a byte array
    ByteBuffer buffer = ByteBuffer.allocate(64);
    for (int i = 0; i < longArray.length; i++) {
      buffer.putLong(56 - (i * 8), longArray[i]);
    }
    byte[] byteArray = buffer.array();

    // Remove any leading zeros from the byte array
    int i = 0;
    while (i < byteArray.length && byteArray[i] == 0) {
      i++;
    }
    if (i == byteArray.length) {
      return BigInteger.ZERO;
    }
    byte[] trimmedByteArray = Arrays.copyOfRange(byteArray, i, byteArray.length);

    // Convert the byte array to a BigInteger
    return new BigInteger(trimmedByteArray);
  }
}
