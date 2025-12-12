/*
 * Copyright ConsenSys Inc.
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
package net.consensys.linea.zktracer.lt;

public class Util {

  static int bitWidthOf(long value) {
    if (value < 2L) {
      return 1;
    } else if (value < 4L) {
      return 2;
    } else if (value < 16L) {
      return 4;
    } else if (value < 256L) {
      return 8;
    } else if (value < 65536L) {
      return 16;
    } else if (value < 4294967296L) {
      return 32;
    } else {
      throw new IllegalArgumentException("invalid value for byte width: " + value);
    }
  }

  /**
   * Count how many blocks of contiguous values there are. For example, in the array [1,2,2,3] we
   * have three blocks, whilst [2,2,2,3,3] has only two.
   *
   * @param data Data containing blocks to be counted.
   * @return Number of blocks within the original data.
   */
  static int countNumberOfBlocks(int[] data) {
    if (data.length == 0) {
      return 0;
    }
    //
    int count = 1;
    int last = data[0];
    //
    for (int i = 1; i < data.length; ++i) {
      if (data[i] != last) {
        count++;
        last = data[i];
      }
    }
    //
    return count;
  }

  /**
   * Determine the size of the largest block of contiguous within the given data.
   *
   * @param data Data containing blocks to be considered.
   * @return Number of rows in the largest block.
   */
  static int determineLargestBlock(int[] data) {
    if (data.length == 0) {
      return 0;
    }
    //
    int max = 0;
    int last = data[0];
    int lastIndex = 0;
    //
    for (int i = 1; i < data.length; ++i) {
      if (data[i] != last) {
        max = Math.max(max, i - lastIndex);
        last = data[i];
        lastIndex = i;
      }
    }
    // Include final block.
    max = Math.max(max, data.length - lastIndex);
    //
    return max;
  }

  /**
   * Compute the maximum value in an array of ints, returning 0 for the empty array.
   *
   * @param data
   * @return
   */
  static long maxValue(int[] data) {
    long max = 0;

    for (int i = 0; i < data.length; i++) {
      max = Math.max(max, data[i] & 0xFFFF_FFFFL);
    }

    return max;
  }

  /**
   * Compute the minimum value in an array of (non-negative) ints, returning Integer.MAX_VALUE for
   * the empty array.
   *
   * @param data
   * @return
   */
  static long minValue(int[] data) {
    long min = Integer.MAX_VALUE;

    for (int i = 0; i < data.length; i++) {
      min = Math.min(min, data[i] & 0xFFFF_FFFFL);
    }

    return min;
  }

  /**
   * Convert a given bitwidth into a bytewidth. For example, a bitwidth of 1 becomes a bytewidth of
   * 1 whilst a bitwidth of 9 becomes a bytewidth of 2, etc.
   *
   * @param bitwidth
   * @return
   */
  static int byteWidth(int bitwidth) {
    int byteWidth = bitwidth / 8;
    //
    if ((bitwidth % 8) != 0) {
      byteWidth++;
    }
    //
    return byteWidth;
  }

  /**
   * Convert a long value into an array of at most 8 bytes which are truncated (i.e. no leading
   * zeros). Thus, the value 0 is returned as the empty array, etc.
   *
   * @param value
   * @return
   */
  static byte[] long2TruncatedBytes(long value) {
    final byte b7 = (byte) (value >> 56);
    final byte b6 = (byte) (value >> 48);
    final byte b5 = (byte) (value >> 40);
    final byte b4 = (byte) (value >> 32);
    final byte b3 = (byte) (value >> 24);
    final byte b2 = (byte) (value >> 16);
    final byte b1 = (byte) (value >> 8);
    final byte b0 = (byte) value;
    // Determine length
    int len = 0;
    while (value != 0) {
      value = value >>> 8;
      len++;
    }
    // Create array
    byte[] bytes = new byte[len];
    int index = 0;
    switch (len) {
      case 8:
        bytes[index++] = b7;
      case 7:
        bytes[index++] = b6;
      case 6:
        bytes[index++] = b5;
      case 5:
        bytes[index++] = b4;
      case 4:
        bytes[index++] = b3;
      case 3:
        bytes[index++] = b2;
      case 2:
        bytes[index++] = b1;
      case 1:
        bytes[index++] = b0;
    }
    // Done
    return bytes;
  }

  /**
   * Determine the minimal number of bits required to store the value held in a given set of bytes
   * (assuming a big endian layout). For example, a value of 0x0AFF (binary 0b00001010_11111111).
   * has a bit length of 12.
   *
   * @param bytes the bytes (stored in big endian form) whose bitlength is being determined.
   * @return
   */
  static int bitLengthOf(byte[] bytes) {
    int n = 0;
    // Skip forward
    while (n < bytes.length && bytes[n] == 0) {
      n++;
    }
    // Determine width
    if (n == bytes.length) {
      return 0;
    } else {
      // n > 0
      int m = bytes.length - n - 1;
      return bitLengthOf(bytes[n]) + (m * 8);
    }
  }

  /**
   * Determine the minimal number of bits required to store the value held in a given byte. For
   * example, 0x0A (binary 0b00001010) has a bit length of 4.
   *
   * @param b the byte whose bitlength is being determined.
   * @return
   */
  static int bitLengthOf(byte b) {
    // Convert into unsigned representation
    int val = b & 0xff;
    // NOTE: we could further improve performance by turning this into one big lookup table.
    if (val >= 16) {
      return bits[val >> 4] + 4;
    } else {
      return bits[val];
    }
  }

  private static final int[] bits = {0, 1, 2, 2, 3, 3, 3, 3, 4, 4, 4, 4, 4, 4, 4, 4};
}
