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

package net.consensys.linea.zktracer.types;

import static net.consensys.linea.zktracer.types.Checks.checkArgument;
import static net.consensys.linea.zktracer.types.Utils.leftPadToBytes16;

import java.math.BigInteger;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;

public class Conversions {
  public static final Bytes ZERO = Bytes.of(0);
  public static final Bytes ONE = Bytes.of(1);
  public static final BigInteger UNSIGNED_LONG_MASK =
      BigInteger.ONE.shiftLeft(Long.SIZE).subtract(BigInteger.ONE);

  public static Bytes bigIntegerToBytes(final BigInteger input) {
    Bytes bytes;
    if (input.equals(BigInteger.ZERO)) {
      bytes = Bytes.of(0x00);
    } else {
      byte[] byteArray = input.toByteArray();
      if (byteArray[0] == 0) {
        Bytes tmp = Bytes.wrap(byteArray);
        bytes = Bytes.wrap(tmp.slice(1, tmp.size() - 1));
      } else {
        bytes = Bytes.wrap(byteArray);
      }
    }
    return bytes;
  }

  public static Bytes32 bigIntegerToBytes32(final BigInteger input) {
    return Bytes32.leftPad(bigIntegerToBytes(input));
  }

  public static Bytes bigIntegerToBytes16(final BigInteger input) {
    return leftPadToBytes16(bigIntegerToBytes(input));
  }

  public static BigInteger booleanToBigInteger(final boolean input) {
    return input ? BigInteger.ONE : BigInteger.ZERO;
  }

  public static int booleanToInt(final boolean input) {
    return input ? 1 : 0;
  }

  public static long booleanToLong(final boolean input) {
    return input ? 1L : 0L;
  }

  public static boolean bigIntegerToBoolean(BigInteger n) {
    if (!n.equals(BigInteger.ONE) && !n.equals(BigInteger.ZERO)) {
      throw new IllegalArgumentException(
          "argument should be equal to BigInteger.ONE or BigInteger.ZERO");
    }
    return BigInteger.valueOf(1).equals(n);
  }

  public static BigInteger longToUnsignedBigInteger(final long input) {
    final BigInteger output = BigInteger.valueOf(input).and(UNSIGNED_LONG_MASK);
    if (output.bitLength() > 64) {
      throw new IllegalArgumentException(
          "a long can't be more than 64 bits long, and is" + output.bitLength());
    }
    return output;
  }

  /**
   * Convert the given {@link Bytes} to a signed {@link BigInteger}, even if the Bytes.toBigInteger
   * is overloaded to create unsigned BigInteger (e.g. in {@link
   * org.apache.tuweni.units.bigints.UInt256}.
   *
   * @param a a object implement {@link Bytes}
   * @return the signed BigInteger represented by a's bytes
   */
  public static BigInteger reallyToSignedBigInteger(Bytes a) {
    byte[] bs = a.toArrayUnsafe();
    return new BigInteger(bs, 0, bs.length);
  }

  public static Bytes32 longToBytes32(final long input) {
    return Bytes32.leftPad(longToBytes(input));
  }

  public static Bytes longToBytes(final long input) {
    return input == 0 ? Bytes.of(0) : Bytes.ofUnsignedLong(input).trimLeadingZeros();
  }

  public static Bytes booleanToBytes(boolean x) {
    return x ? ONE : Bytes.EMPTY;
  }

  public static long bytesToLong(final Bytes input) {
    final Bytes trimmedBytes = input.trimLeadingZeros();
    checkArgument(trimmedBytes.size() <= 8, "Input bytes must be at most 8 bytes long");
    return trimmedBytes.toUnsignedBigInteger().longValueExact();
  }

  public static short bytesToShort(final Bytes input) {
    final Bytes trimmedBytes = input.trimLeadingZeros();
    checkArgument(trimmedBytes.size() <= 2, "Input bytes must be at most 2 bytes long");
    return (short) trimmedBytes.toInt();
  }

  public static boolean bytesToBoolean(final Bytes input) {
    final int bitLength = input.bitLength();
    checkArgument(
        bitLength == 0 || bitLength == 1, String.format("Can't convert %s to boolean", input));
    return bitLength == 1;
  }

  public static String bytesToHex(byte[] bytes) {
    StringBuilder sb = new StringBuilder();
    for (byte b : bytes) {
      sb.append(String.format("%02X ", b));
    }
    return sb.toString().trim();
  }

  /**
   * This method expects a "small-ish" long value and returns the corresponding int value.
   *
   * @param value
   * @return
   * @throws ArithmeticException
   */
  public static int safeLongToInt(long value) throws ArithmeticException {
    if (value < 0 || value > Integer.MAX_VALUE) {
      throw new ArithmeticException(value + " cannot be cast to int without changing its value.");
    }
    return (int) value;
  }

  public static Bytes unsignedIntToBytes(int value) {
    return bigIntegerToBytes(BigInteger.valueOf(Integer.toUnsignedLong(value)));
  }
}
