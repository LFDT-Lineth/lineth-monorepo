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

import java.nio.ByteBuffer;

public record Encoding(int encoding, byte[] data) {
  public static final byte ENCODING_CONSTANT = 0;
  public static final byte ENCODING_STATIC_DENSE = 1;
  public static final byte ENCODING_STATIC_SPARSE8 = 2;
  public static final byte ENCODING_STATIC_SPARSE16 = 3;
  public static final byte ENCODING_STATIC_SPARSE24 = 4; // not supported
  public static final byte ENCODING_STATIC_SPARSE32 = 5;
  public static final byte ENCODING_POOL_CONSTANT = 6;
  public static final byte ENCODING_POOL1_DENSE = 7;
  public static final byte ENCODING_POOL2_DENSE = 8;
  public static final byte ENCODING_POOL4_DENSE = 9;
  public static final byte ENCODING_POOL8_DENSE = 10;
  public static final byte ENCODING_POOL16_DENSE = 11;
  public static final byte ENCODING_POOL32_DENSE = 12;
  public static final byte ENCODING_POOL8_SPARSE8 = 13;
  public static final byte ENCODING_POOL8_SPARSE16 = 14;
  public static final byte ENCODING_POOL8_SPARSE24 = 15; // not supported
  public static final byte ENCODING_POOL8_SPARSE32 = 16;
  public static final byte ENCODING_POOL16_SPARSE8 = 17;
  public static final byte ENCODING_POOL16_SPARSE16 = 18;
  public static final byte ENCODING_POOL16_SPARSE24 = 19; // not supported
  public static final byte ENCODING_POOL16_SPARSE32 = 20;
  public static final byte ENCODING_POOL32_SPARSE8 = 21;
  public static final byte ENCODING_POOL32_SPARSE16 = 22;
  public static final byte ENCODING_POOL32_SPARSE24 = 23; // not supported
  public static final byte ENCODING_POOL32_SPARSE32 = 24;

  public Encoding(boolean pooled, int constant, byte[] data) {
    this(encodingConstant(pooled, constant), data);
  }

  public Encoding(boolean pooled, int entryWidth, int bitwidth, byte[] data) {
    this(encodingDense(pooled, entryWidth, bitwidth), data);
  }

  public Encoding(boolean pooled, int entryWidth, int blockSizeWidth, int bitwidth, byte[] data) {
    this(encodingSparse(pooled, entryWidth, blockSizeWidth, bitwidth), data);
  }

  /**
   * Construct an "encoding identifier" from a given opcode and operand.
   *
   * @param pooled Indicates whether entries represent heap indices or raw values.
   * @param entryWidth Determines bitwidth of entries in the encoded data.
   * @param bitwidth Determines bitwidth of raw values (i.e. regardless of whether they are
   *     encoded).
   * @return
   */
  private static int encodingSparse(
      boolean pooled, int entryWidth, int blockSizeWidth, int bitwidth) {
    int opcode;
    int byteWidth = Util.byteWidth(blockSizeWidth);
    //
    if (!pooled) {
      opcode = ENCODING_STATIC_SPARSE8 + (byteWidth - 1);
    } else if (entryWidth <= 8) {
      opcode = ENCODING_POOL8_SPARSE8 + (byteWidth - 1);
    } else if (entryWidth <= 16) {
      opcode = ENCODING_POOL16_SPARSE8 + (byteWidth - 1);
    } else if (entryWidth <= 32) {
      opcode = ENCODING_POOL32_SPARSE8 + (byteWidth - 1);
    } else {
      throw new IllegalArgumentException(
          "invalid dense encoding (" + pooled + ",u" + entryWidth + ",u" + bitwidth + ")");
    }
    //
    int encoding = (opcode & 0xff) << 24;
    return encoding | (bitwidth & 0xff_ffff);
  }

  private static int encodingDense(boolean pooled, int entryWidth, int bitwidth) {
    int opcode;
    //
    if (!pooled) {
      opcode = ENCODING_STATIC_DENSE;
    } else if (entryWidth <= 1) {
      opcode = ENCODING_POOL1_DENSE;
    } else if (entryWidth <= 2) {
      opcode = ENCODING_POOL2_DENSE;
    } else if (entryWidth <= 4) {
      opcode = ENCODING_POOL4_DENSE;
    } else if (entryWidth <= 8) {
      opcode = ENCODING_POOL8_DENSE;
    } else if (entryWidth <= 16) {
      opcode = ENCODING_POOL16_DENSE;
    } else if (entryWidth <= 32) {
      opcode = ENCODING_POOL32_DENSE;
    } else {
      throw new IllegalArgumentException(
          "invalid dense encoding (" + pooled + ",u" + entryWidth + ",u" + bitwidth + ")");
    }
    //
    int encoding = (opcode & 0xff) << 24;
    return encoding | (bitwidth & 0xff_ffff);
  }

  private static int encodingConstant(boolean pooled, int constant) {
    if (constant > 0xff_ffff) {
      throw new IllegalArgumentException("invalid encoded constant (" + constant + ")");
    }
    //
    int opcode = pooled ? ENCODING_POOL_CONSTANT : ENCODING_CONSTANT;
    int encoding = (opcode & 0xff) << 24;
    return encoding | (constant & 0xff_ffff);
  }

  /**
   * Construct a static encoding for a given set of column data. This is where values are encoded
   * directly in the resulting byte array (i.e. rather than being encoded as indexes into the heap).
   *
   * @param buffer Column data
   * @return Encoded column data
   */
  public static Encoding of(String name, int[] buffer) {
    long maxValue = Util.maxValue(buffer);
    long minValue = Util.minValue(buffer);
    int numberOfBlocks = Util.countNumberOfBlocks(buffer);
    int blockSize = Util.determineLargestBlock(buffer);
    int bitwidth = Util.bitWidthOf(maxValue);
    //
    if ((buffer.length == 0 || maxValue == minValue) && maxValue <= 0xFF_FFFF) {
      return new Encoding(false, (int) maxValue, encodeU0(buffer.length));
    } else if (preferSparseEncoding(numberOfBlocks, buffer.length, maxValue, blockSize)) {
      // sparse encoding
      return encodeSparseData(false, buffer, numberOfBlocks, maxValue, blockSize, bitwidth);
    } else {
      // dense encoding
      return encodeDenseData(false, buffer, maxValue, bitwidth);
    }
  }

  /**
   * Construct a pooled encoding for a given set of column data. This is where values are encoded in
   * the resulting array as indexes into the heap (i.e. rather than being encoded directly into the
   * byte array). The benefit of this encoding arises in two ways: (1) when there are lots of
   * repeated large (e.g. u128) values; (2) when we have a large column (e.g. u128) which contains
   * many small items. Furthermore, since the heap is shared across all column in the encoded trace,
   * this encoding benefits when the same value appears in multiple columns, etc.
   *
   * @param buffer Column data
   * @return Encoded column data
   */
  public static Encoding of(String name, int[] buffer, int bitwidth, BytesHeap heap) {
    long maxValue = Util.maxValue(buffer);
    long minValue = Util.minValue(buffer);
    int numberOfBlocks = Util.countNumberOfBlocks(buffer);
    int blockSize = Util.determineLargestBlock(buffer);
    //
    if ((buffer.length == 0 || maxValue == minValue) && maxValue <= 0xFF_FFFF) {
      if (buffer.length == 0 || maxValue == 0) {
        // NOTE: index 0 already represents actual 0
        return new Encoding(false, 0, encodeU0(buffer.length));
      } else {
        return new Encoding(true, (int) maxValue, encodeU0(buffer.length));
      }
    } else if (preferSparseEncoding(numberOfBlocks, buffer.length, maxValue, blockSize)) {
      // sparse encoding
      return encodeSparseData(true, buffer, numberOfBlocks, maxValue, blockSize, bitwidth);
    } else {
      // dense encoding
      return encodeDenseData(true, buffer, maxValue, bitwidth);
    }
  }

  /**
   * Check whether or not to use a sparse representation of the given data. The decision is made
   * simply by calculating the size of both the sparse and dense representations, and choosing the
   * smallest. In doing this calculation, we have to consider the width of values used in each
   * representation (e.g. because values can be wider when used in the sparse representation).
   *
   * @param numBlocks Identifies the number of entries required for the sparse representation.
   * @param length Identifies the number of actual rows in the column.
   * @param maxValue Identifies the maximum value in the buffer.
   * @param maxBlockSize Identifies the maximum block size in the buffer.
   * @return true if the sparse representation is smallest, false otherwise.
   */
  private static boolean preferSparseEncoding(
      int numBlocks, int length, long maxValue, int maxBlockSize) {
    int valueWidth = Util.bitWidthOf(maxValue);
    int blockSizeWidth = Util.bitWidthOf(maxBlockSize);
    // Sparse representation does not encode data in smaller chunks than a single byte, whereas the
    // dense
    // representation does.
    int sparseWidth = Math.max(8, valueWidth) + Math.max(8, blockSizeWidth);
    // Do the calculation
    int sparseLayout = numBlocks * sparseWidth;
    int denseLayout = length * valueWidth;
    // Decide which representation wins out.
    return sparseLayout < denseLayout;
  }

  private static Encoding encodeSparseData(
      boolean pooled,
      int[] buffer,
      int numberOfBlocks,
      long maxEntry,
      int maxBlockSize,
      int bitwidth) {
    int entryBitwidth = Math.max(8, Util.bitWidthOf(maxEntry));
    int blockSizeWidth = Math.max(8, Util.bitWidthOf(maxBlockSize));
    byte[] data;
    //
    switch (blockSizeWidth) {
      case 8:
        data =
            switch (entryBitwidth) {
              case 8 -> encodeU8Sparse8(numberOfBlocks, buffer);
              case 16 -> encodeU16Sparse8(numberOfBlocks, buffer);
              case 32 -> encodeU32Sparse8(numberOfBlocks, buffer);
              default ->
                  throw new IllegalArgumentException(
                      "invalid entry width (u" + entryBitwidth + ")");
            };
        break;
      case 16:
        data =
            switch (entryBitwidth) {
              case 8 -> encodeU8Sparse16(numberOfBlocks, buffer);
              case 16 -> encodeU16Sparse16(numberOfBlocks, buffer);
              case 32 -> encodeU32Sparse16(numberOfBlocks, buffer);
              default ->
                  throw new IllegalArgumentException(
                      "invalid entry width (u" + entryBitwidth + ")");
            };
        break;
      case 32:
        data =
            switch (entryBitwidth) {
              case 8 -> encodeU8Sparse32(numberOfBlocks, buffer);
              case 16 -> encodeU16Sparse32(numberOfBlocks, buffer);
              case 32 -> encodeU32Sparse32(numberOfBlocks, buffer);
              default ->
                  throw new IllegalArgumentException(
                      "invalid entry width (u" + entryBitwidth + ")");
            };
        break;
      default:
        throw new IllegalArgumentException("invalid block size width (u" + blockSizeWidth + ")");
    }
    //
    return new Encoding(pooled, entryBitwidth, blockSizeWidth, bitwidth, data);
  }

  private static Encoding encodeDenseData(
      boolean pooled, int[] buffer, long maxEntry, int bitwidth) {
    int entryBitwidth = Util.bitWidthOf(maxEntry);
    //
    byte[] data =
        switch (entryBitwidth) {
          case 1 -> encodeU1(buffer);
          case 2 -> encodeU2(buffer);
          case 4 -> encodeU4(buffer);
          case 8 -> encodeU8Dense(buffer);
          case 16 -> encodeU16Dense(buffer);
          case 32 -> encodeU32Dense(buffer);
          default ->
              throw new IllegalArgumentException("invalid entry width (u" + entryBitwidth + ")");
        };
    //
    return new Encoding(pooled, entryBitwidth, bitwidth, data);
  }

  /**
   * Encode an array of constant values. This amounts simply to writing the length (i.e. number of
   * rows) into the buffer, and nothing else.
   *
   * @param length Number of rows in the column.
   * @return byte encoding of the data
   */
  private static byte[] encodeU0(int length) {
    final ByteBuffer buf = ByteBuffer.allocate(4);
    buf.putInt(length);
    return buf.array();
  }

  /**
   * U1 encoding is given a special bit-level representation for efficiency. Specifically, 8
   * elements are packed into each byte of the resulting data. Furthermore, the number of unused
   * elements is stored in an additional last byte.
   *
   * @param buffer Contains only values in {0,1}.
   * @return byte encoding of the data
   */
  private static byte[] encodeU1(int[] buffer) {
    // Determine how many bytes required
    int n = Util.byteWidth(buffer.length);
    final byte[] bytes = new byte[n + 1];
    int bitIndex = 0;
    int byteIndex = 0;
    //
    for (int i = 0; i != buffer.length; i++) {
      int bit = (0x1 & buffer[i]) << bitIndex;
      int ith = (bytes[byteIndex] & 0xff) | bit;
      // Assign updated byte
      bytes[byteIndex] = (byte) ith;
      // Increment indices
      if (++bitIndex == 8) {
        bitIndex = 0;
        byteIndex++;
      }
    }
    // Mark unused bits
    bytes[n] = (byte) ((n * 8) - buffer.length);
    // Done
    return bytes;
  }

  /**
   * U2 encoding is given a special bit-level representation for efficiency. Specifically, 4
   * elements are packed into each byte of the resulting data. Furthermore, the number of unused
   * elements is stored in an additional last byte.
   *
   * @param buffer Contains only values in {0,1,2,3}.
   * @return byte encoding of the data
   */
  private static byte[] encodeU2(int[] buffer) {
    // Determine how many bytes required
    int n = Util.byteWidth(buffer.length * 2);
    final byte[] bytes = new byte[n + 1];
    int bitIndex = 0;
    int byteIndex = 0;
    //
    for (int i = 0; i != buffer.length; i++) {
      int bits = (0x3 & buffer[i]) << bitIndex;
      int ith = (bytes[byteIndex] & 0xff) | bits;
      // Assign updated byte
      bytes[byteIndex] = (byte) ith;
      // Increment indices
      bitIndex += 2;
      // Check overflow
      if (bitIndex == 8) {
        bitIndex = 0;
        byteIndex++;
      }
    }
    // Mark unused bits
    bytes[n] = (byte) ((n * 4) - buffer.length);
    // Done
    return bytes;
  }

  /**
   * U4 encoding is given a special bit-level representation for efficiency. Specifically, 2
   * elements are packed into each byte of the resulting data. Furthermore, the number of unused
   * elements is stored in an additional last byte.
   *
   * @param buffer Contains only values in {0,1,...14,15}.
   * @return byte encoding of the data
   */
  private static byte[] encodeU4(int[] buffer) {
    // Determine how many bytes required
    int n = Util.byteWidth(buffer.length * 4);
    final byte[] bytes = new byte[n + 1];
    int bitIndex = 0;
    int byteIndex = 0;
    //
    for (int i = 0; i != buffer.length; i++) {
      int bits = (0xf & buffer[i]) << bitIndex;
      int ith = (bytes[byteIndex] & 0xff) | bits;
      // Assign updated byte
      bytes[byteIndex] = (byte) ith;
      // Increment indices
      bitIndex += 4;
      // Check overflow
      if (bitIndex == 8) {
        bitIndex = 0;
        byteIndex++;
      }
    }
    // Mark unused bits
    bytes[n] = (byte) ((n * 2) - buffer.length);
    // Done
    return bytes;
  }

  /**
   * Encode a given set of byte values using a "dense encoding" where each value is stored
   * consecutively.
   *
   * @param buffer Contains only values in {0,1,...254,255}.
   * @return byte encoding of the data
   */
  private static byte[] encodeU8Dense(int[] buffer) {
    final byte[] bytes = new byte[buffer.length];
    //
    for (int i = 0; i != buffer.length; i++) {
      bytes[i] = (byte) buffer[i];
    }
    //
    return bytes;
  }

  /**
   * Encode 8bit values using a "sparse encoding" consisting of tuples (u8 value, u8 n), where each
   * represents n copies of the given value.
   *
   * @param buffer Contains only values in {0,1,...65534,65535}.
   * @return byte encoding of the data
   */
  private static byte[] encodeU8Sparse8(int numBlocks, int[] buffer) {
    final ByteBuffer bytes = ByteBuffer.allocate(numBlocks * 2);
    //
    if (numBlocks > 0) {
      int last = buffer[0];
      int lastIndex = 0;
      bytes.put((byte) (last & 0xff));
      //
      for (int i = 1; i < buffer.length; ++i) {
        if (buffer[i] != last) {
          bytes.put((byte) (i - lastIndex));
          last = buffer[i];
          lastIndex = i;
          bytes.put((byte) (last & 0xff));
        }
      }
      //
      bytes.put((byte) (buffer.length - lastIndex));
    }
    //
    return bytes.array();
  }

  /**
   * Encode 8bit values using a "sparse encoding" consisting of tuples (u8 value, u16 n), where each
   * represents n copies of the given value.
   *
   * @param buffer Contains only values in {0,1,...254,255}.
   * @return byte encoding of the data
   */
  private static byte[] encodeU8Sparse16(int numBlocks, int[] buffer) {
    final ByteBuffer bytes = ByteBuffer.allocate(numBlocks * 3);
    //
    if (numBlocks > 0) {
      int last = buffer[0];
      int lastIndex = 0;
      bytes.put((byte) (last & 0xff));
      //
      for (int i = 1; i < buffer.length; ++i) {
        if (buffer[i] != last) {
          bytes.putShort((short) (i - lastIndex));
          last = buffer[i];
          lastIndex = i;
          bytes.put((byte) (last & 0xff));
        }
      }
      //
      bytes.putShort((short) (buffer.length - lastIndex));
    }
    //
    return bytes.array();
  }

  /**
   * Encode 8but values using a "sparse encoding" consisting of tuples (u8 value, u32 n), where each
   * represents n copies of the given value.
   *
   * @param buffer Contains only values in {0,1,...65534,65535}.
   * @return byte encoding of the data
   */
  private static byte[] encodeU8Sparse32(int numBlocks, int[] buffer) {
    final ByteBuffer bytes = ByteBuffer.allocate(numBlocks * 5);
    //
    if (numBlocks > 0) {
      int last = buffer[0];
      int lastIndex = 0;
      bytes.put((byte) (last & 0xff));
      //
      for (int i = 1; i < buffer.length; ++i) {
        if (buffer[i] != last) {
          bytes.putInt(i - lastIndex);
          last = buffer[i];
          lastIndex = i;
          bytes.put((byte) (last & 0xff));
        }
      }
      //
      bytes.putInt(buffer.length - lastIndex);
    }
    //
    return bytes.array();
  }

  /**
   * Encode a given set of 16bit values using a "dense encoding" where each value is stored
   * consecutively.
   *
   * @param buffer Contains only values in {0,1,...65534,65535}.
   * @return byte encoding of the data
   */
  private static byte[] encodeU16Dense(int[] buffer) {
    final byte[] bytes = new byte[buffer.length * 2];
    //
    for (int i = 0; i != buffer.length; i++) {
      final long ith = buffer[i];
      bytes[i << 1] = (byte) (ith >> 8);
      bytes[(i << 1) + 1] = (byte) ith;
    }
    //
    return bytes;
  }

  /**
   * Encode 16bit values using a "sparse encoding" consisting of tuples (u16 value, u8 n), where
   * each represents n copies of the given value.
   *
   * @param buffer Contains only values in {0,1,...65534,65535}.
   * @return byte encoding of the data
   */
  private static byte[] encodeU16Sparse8(int numBlocks, int[] buffer) {
    final ByteBuffer bytes = ByteBuffer.allocate(numBlocks * 3);
    //
    if (numBlocks > 0) {
      int last = buffer[0];
      int lastIndex = 0;
      bytes.putShort((short) (last & 0xffff));
      //
      for (int i = 1; i < buffer.length; ++i) {
        if (buffer[i] != last) {
          bytes.put((byte) (i - lastIndex));
          last = buffer[i];
          lastIndex = i;
          bytes.putShort((short) (last & 0xffff));
        }
      }
      //
      bytes.put((byte) (buffer.length - lastIndex));
    }
    //
    return bytes.array();
  }

  /**
   * Encode 16bit values using a "sparse encoding" consisting of tuples (u16 value, u16 n), where
   * each represents n copies of the given value.
   *
   * @param buffer Contains only values in {0,1,...65534,65535}.
   * @return byte encoding of the data
   */
  private static byte[] encodeU16Sparse16(int numBlocks, int[] buffer) {
    final ByteBuffer bytes = ByteBuffer.allocate(numBlocks * 4);
    //
    if (numBlocks > 0) {
      int last = buffer[0];
      int lastIndex = 0;
      bytes.putShort((short) (last & 0xffff));
      //
      for (int i = 1; i < buffer.length; ++i) {
        if (buffer[i] != last) {
          bytes.putShort((short) (i - lastIndex));
          last = buffer[i];
          lastIndex = i;
          bytes.putShort((short) (last & 0xffff));
        }
      }
      //
      bytes.putShort((short) (buffer.length - lastIndex));
    }
    //
    return bytes.array();
  }

  /**
   * Encode 16bit values using a "sparse encoding" consisting of tuples (u16 value, u32 n), where
   * each represents n copies of the given value.
   *
   * @param buffer Contains only values in {0,1,...65534,65535}.
   * @return byte encoding of the data
   */
  private static byte[] encodeU16Sparse32(int numBlocks, int[] buffer) {
    final ByteBuffer bytes = ByteBuffer.allocate(numBlocks * 6);
    //
    if (numBlocks > 0) {
      int last = buffer[0];
      int lastIndex = 0;
      bytes.putShort((short) (last & 0xffff));
      //
      for (int i = 1; i < buffer.length; ++i) {
        if (buffer[i] != last) {
          bytes.putInt(i - lastIndex);
          last = buffer[i];
          lastIndex = i;
          bytes.putShort((short) (last & 0xffff));
        }
      }
      //
      bytes.putInt(buffer.length - lastIndex);
    }
    //
    return bytes.array();
  }

  /**
   * Encode 32bit values using a "dense encoding" where each value is stored consecutively.
   *
   * @param buffer Contains values to be encoded.
   * @return byte encoding of the data
   */
  private static byte[] encodeU32Dense(int[] buffer) {
    final byte[] bytes = new byte[buffer.length * 4];
    //
    for (int i = 0; i != buffer.length; i++) {
      final int ith = buffer[i];
      bytes[i << 2] = (byte) (ith >> 24);
      bytes[(i << 2) + 1] = (byte) (ith >> 16);
      bytes[(i << 2) + 2] = (byte) (ith >> 8);
      bytes[(i << 2) + 3] = (byte) ith;
    }
    //
    return bytes;
  }

  /**
   * Encode 32bit values using a "sparse encoding" consisting of tuples (u32 value, u8 n), where
   * each represents n copies of the given value.
   *
   * @param buffer Contains only values in {0,1,...2^31}.
   * @return byte encoding of the data
   */
  private static byte[] encodeU32Sparse8(int numBlocks, int[] buffer) {
    final ByteBuffer bytes = ByteBuffer.allocate(numBlocks * 5);
    //
    if (numBlocks > 0) {
      int last = buffer[0];
      int lastIndex = 0;
      bytes.putInt(last);
      //
      for (int i = 1; i < buffer.length; ++i) {
        if (buffer[i] != last) {
          bytes.put((byte) (i - lastIndex));
          last = buffer[i];
          lastIndex = i;
          bytes.putInt(last);
        }
      }
      //
      bytes.put((byte) (buffer.length - lastIndex));
    }
    //
    return bytes.array();
  }

  /**
   * Encode 32bit values using a "sparse encoding" consisting of tuples (u32 value, u16 n), where
   * each represents n copies of the given value.
   *
   * @param buffer Contains only values in {0,1,...2^31}.
   * @return byte encoding of the data
   */
  private static byte[] encodeU32Sparse16(int numBlocks, int[] buffer) {
    final ByteBuffer bytes = ByteBuffer.allocate(numBlocks * 6);
    //
    if (numBlocks > 0) {
      int last = buffer[0];
      int lastIndex = 0;
      bytes.putInt(last);
      //
      for (int i = 1; i < buffer.length; ++i) {
        if (buffer[i] != last) {
          bytes.putShort((short) (i - lastIndex));
          last = buffer[i];
          lastIndex = i;
          bytes.putInt(last);
        }
      }
      //
      bytes.putShort((short) (buffer.length - lastIndex));
    }
    //
    return bytes.array();
  }

  /**
   * Encode 32bit values using a "sparse encoding" consisting of tuples (u32 value, u32 n), where
   * each represents n copies of the given value.
   *
   * @param buffer Contains only values in {0,1,...2^31}.
   * @return byte encoding of the data
   */
  private static byte[] encodeU32Sparse32(int numBlocks, int[] buffer) {
    final ByteBuffer bytes = ByteBuffer.allocate(numBlocks * 8);
    //
    if (numBlocks > 0) {
      int last = buffer[0];
      int lastIndex = 0;
      bytes.putInt(last);
      //
      for (int i = 1; i < buffer.length; ++i) {
        if (buffer[i] != last) {
          bytes.putInt(i - lastIndex);
          last = buffer[i];
          lastIndex = i;
          bytes.putInt(last);
        }
      }
      //
      bytes.putInt(buffer.length - lastIndex);
    }
    //
    return bytes.array();
  }
}
