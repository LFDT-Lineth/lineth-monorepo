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

import static com.google.common.base.Preconditions.checkArgument;
import static net.consensys.linea.zktracer.types.Utils.rightPadTo;

import java.math.BigInteger;
import net.consensys.linea.zktracer.types.UnsignedByte;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.units.bigints.UInt256;
import org.hyperledger.besu.datatypes.TransactionType;

/** Utility class that provides various helper methods. */
public class Util {
  /**
   * Converts the bits of an unsigned byte into an array of Booleans.
   *
   * @param b The unsigned byte to be converted.
   * @return A Boolean array containing the bits of the input byte.
   */
  public static Boolean[] byteBits(final UnsignedByte b) {
    final Boolean[] bits = new Boolean[8];
    for (int i = 0; i < 8; i++) {
      bits[7 - i] = b.shiftRight(i).mod(2).toInteger() == 1;
    }
    return bits;
  }

  /**
   * Checks if the shifted UInt256 argument overflows a given maximum value, and throws an exception
   * with an error message if it does.
   *
   * @param arg The UInt256 argument to be checked.
   * @param maxVal The maximum value the shifted argument should not exceed.
   * @param err The error message to be included in the exception.
   * @return The overflow value if it doesn't exceed the maximum value.
   */
  public static long getOverflow(UInt256 arg, long maxVal, String err) {
    UInt256 shiftedArg = arg.shiftRight(128);

    if (!shiftedArg.fitsLong()) {
      throw new RuntimeException("getOverflow expects a small high part");
    }
    long overflow = shiftedArg.toUnsignedBigInteger().longValue();
    if (overflow > maxVal) {
      throw new RuntimeException(err);
    }
    return overflow;
  }

  /**
   * Checks if the k'th bit of a given long value is 1.
   *
   * @param x The long value to be checked.
   * @param k The index of the bit to be checked.
   * @return True if the k'th bit of x is 1, otherwise false.
   */
  public static boolean getBit(long x, int k) {
    return (x >> k) % 2 == 1;
  }

  /**
   * Converts a boolean value to an integer (1 for true and 0 for false).
   *
   * @param bool The boolean value to be converted.
   * @return An integer representing the input boolean value.
   */
  public static int boolToInt(boolean bool) {
    return bool ? 1 : 0;
  }

  /**
   * Converts a 64-bit unsigned integer into an 8-byte array.
   *
   * @param x The 64-bit unsigned integer to be converted.
   * @return An 8-byte array representing the input value.
   */
  public static byte[] uInt64ToBytes(long x) {
    byte[] xBytes = new byte[8];
    for (int k = 0; k < 8; k++) {
      xBytes[7 - k] = (byte) (x % 256);
      x >>= 8;
    }
    return xBytes;
  }

  /**
   * Checks if a given BigInteger value is a valid UInt256.
   *
   * @param number The BigInteger value to be checked.
   * @return True if the input value is a valid UInt256, otherwise false.
   */
  public static boolean isUInt256(BigInteger number) {
    return number.bitLength() <= 256;
  }

  /**
   * Multiplies the elements of two byte array ranges and returns the sum of their UInt256 products.
   *
   * @param range1 The first byte array range.
   * @param range2 The second byte array range.
   * @return The UInt256 sum of the products of the elements in the two input ranges.
   */
  public static UInt256 multiplyRange(Bytes[] range1, Bytes[] range2) {
    checkArgument(range1.length == range2.length, "Ranges must be of the same length");
    UInt256 sum = UInt256.ZERO;
    for (int i = 0; i < range1.length; i++) {
      UInt256 prod =
          UInt256.fromBytes(range1[i]).multiply(UInt256.fromBytes(range2[range2.length - i - 1]));
      sum = sum.add(prod);
    }
    return sum;
  }

  /**
   * Converts a boolean value to a byte (1 for true and 0 for false).
   *
   * @param b The boolean value to be converted.
   * @return A byte representing the input boolean value.
   */
  public static byte boolToByte(boolean b) {
    return (byte) (b ? 1 : 0);
  }

  /**
   * Returns the maximum of two BigInteger values.
   *
   * @param x the first BigInteger to compare
   * @param y the second BigInteger to compare
   * @return the maximum of x and y. If x is less than y, it returns y, otherwise it returns x.
   */
  public static BigInteger max(final BigInteger x, final BigInteger y) {
    if (x.compareTo(y) < 0) {
      return y;
    }
    return x;
  }

  /**
   * Return the type of transaction as an int
   *
   * @param txType
   * @return transaction type
   */
  public static short getTxTypeAsInt(TransactionType txType) {
    return switch (txType) {
      case FRONTIER -> 0;
      case ACCESS_LIST -> 1;
      case EIP1559 -> 2;
      case BLOB -> 3;
      case DELEGATE_CODE -> 4;
      default -> throw new RuntimeException("Transaction type not supported:" + txType);
    };
  }

  /**
   * Extracts {@code size} many bytes from {@code data} starting at {@code offset}. If {@code data}
   * ``runs out'' it substitutes the missing bytes with 0's.
   *
   * @param size Bytes of
   * @param data, right-padded with 0's if needed, starting from
   * @param offset
   */
  public static Bytes rightPaddedSlice(Bytes data, int offset, int size) {

    checkArgument(offset >= 0, "Offset must be non-negative");
    checkArgument(size >= 0, "Size must be non-negative");

    final int dataSize = data.size();

    // pure padding
    if (offset >= dataSize) {
      return Bytes.repeat((byte) 0x0, size);
    }

    // pure data
    if ((offset + size) <= dataSize) {
      return data.slice(offset, size);
    }

    // data followed by padding
    return rightPadTo(data.slice(offset, dataSize - offset), size);
  }
}
