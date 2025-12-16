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
import net.consensys.linea.zktracer.Trace;

public interface Column extends Trace.Column {
  /**
   * Unqualified name of the column.
   *
   * @return
   */
  String name();

  /**
   * bitwidth of enclosing column.
   *
   * @return
   */
  int bitwidth();

  /**
   * Get an encoding of this column
   *
   * @return
   */
  Encoding toEncoding();

  static Column of(Trace.ColumnHeader header, BytesHeap heap) {
    if (header.bitwidth() <= 32) {
      return new Small(header);
    } else {
      return new Large(header, heap);
    }
  }

  class Base {
    private final String name;
    private final int bitwidth;

    public Base(Trace.ColumnHeader header) {
      final String[] split = header.name().split("\\.");
      this.bitwidth = header.bitwidth();
      // Unqualify the qualified name
      this.name =
          switch (split.length) {
            case 1 -> split[0];
            case 2 -> split[1];
            default -> {
              throw new IllegalArgumentException("invalid column name: " + header.name());
            }
          };
    }

    public String name() {
      return this.name;
    }

    public int bitwidth() {
      return this.bitwidth;
    }
  }

  /**
   * Provides an encoding for small column data, where each element is stored explicitly (i.e. not
   * as a pool index). This uses an optimised encoding based upon the largest element encountered.
   * This is suitable for the v2 file format.
   */
  class Small extends Base implements Column {
    private final long longMax;
    private final int[] buffer;
    private int index;

    public Small(Trace.ColumnHeader header) {
      super(header);
      // Sanity check bitwidth
      if (header.bitwidth() >= 64) {
        throw new IllegalArgumentException(
            "invalid width for small column (u" + header.bitwidth() + ")");
      }
      // Following cannot overflow because of above check.
      this.longMax = 1L << header.bitwidth();
      this.buffer = new int[header.length()];
    }

    @Override
    public void write(boolean value) {
      this.buffer[index++] = value ? 1 : 0;
    }

    @Override
    public void write(byte value) {
      this.write(value & 0xff);
    }

    @Override
    public void write(long value) {
      // Sanity check
      if (value < 0 || longMax <= value) {
        throw new IllegalArgumentException(name() + " has invalid value (" + value + ")");
      }
      //
      this.buffer[index++] = (int) value;
    }

    /**
     * Write element bytes
     *
     * @param bytes stored in big-endian form and already trimmed.
     */
    @Override
    public void write(byte[] bytes) {
      throw new UnsupportedOperationException();
    }

    /**
     * Convert this column into a given byte encoding.
     *
     * @return
     */
    public Encoding toEncoding() {
      return Encoding.of(name(), buffer);
    }
  }

  /**
   * Provides an encoding for potentially large column data, where each element is stored as an
   * index into the heap. This is suitable for the v2 file format.
   */
  class Large extends Base implements Column {
    private final BytesHeap heap;
    private final int[] buffer;
    private int index;

    public Large(Trace.ColumnHeader header, BytesHeap heap) {
      super(header);
      this.heap = heap;
      this.buffer = new int[header.length()];
    }

    @Override
    public void write(boolean value) {
      this.write(value ? 1L : 0L);
    }

    @Override
    public void write(byte value) {
      this.write(value & 0xff);
    }

    @Override
    public void write(long value) {
      this.write(Util.long2TruncatedBytes(value));
    }

    /**
     * Write element bytes
     *
     * @param bytes stored in big-endian form and already trimmed.
     */
    @Override
    public void write(byte[] bytes) {
      this.buffer[index++] = heap.insert(bytes);
    }

    /**
     * Convert this column into a given byte encoding.
     *
     * @return
     */
    public Encoding toEncoding() {
      return Encoding.of(name(), buffer, bitwidth(), heap);
    }
  }

  /**
   * Provides a simple encoding of column data, where each element is stored directly in place. This
   * is suitable for the v1 file format.
   */
  class Raw implements Trace.Column {
    private final String name;
    private final int bitWidth;
    private final int byteWidth;
    private final long longMax;
    private final ByteBuffer buffer;

    public Raw(String name, int bitwidth, int length) {
      this.name = name;
      this.bitWidth = bitwidth;
      this.byteWidth = Util.byteWidth(bitwidth);
      this.longMax = 1L << bitwidth;
      this.buffer = ByteBuffer.allocate(length * byteWidth);
    }

    @Override
    public void write(boolean value) {
      this.buffer.put((byte) (value ? 1 : 0));
    }

    @Override
    public void write(byte value) {
      this.write(value & 0xff);
    }

    @Override
    public void write(long value) {
      // Sanity check
      if (value < 0 || longMax <= value) {
        throw new IllegalArgumentException(name + " has invalid value (" + value + ")");
      }
      //
      switch (byteWidth) {
        case 8:
          this.buffer.put((byte) (value >> 56));
        case 7:
          this.buffer.put((byte) (value >> 48));
        case 6:
          this.buffer.put((byte) (value >> 40));
        case 5:
          this.buffer.put((byte) (value >> 32));
        case 4:
          this.buffer.put((byte) (value >> 24));
        case 3:
          this.buffer.put((byte) (value >> 16));
        case 2:
          this.buffer.put((byte) (value >> 8));
        case 1:
          this.buffer.put((byte) value);
          break;
        default:
          throw new IllegalArgumentException(name + " has invalid width (" + byteWidth + "bytes)");
      }
    }

    /**
     * Write element bytes
     *
     * @param bytes stored in big-endian form and already trimmed.
     */
    @Override
    public void write(byte[] bytes) {
      final int n = Util.bitLengthOf(bytes);
      // Sanity check
      if (n > bitWidth || bytes.length > byteWidth) {
        throw new IllegalArgumentException(name + " has invalid width (" + n + " bits)");
      }
      // Write padding (if necessary)
      for (int i = bytes.length; i < byteWidth; i++) {
        buffer.put((byte) 0);
      }
      // Write data
      for (int i = 0; i != bytes.length; i++) {
        buffer.put(bytes[i]);
      }
    }

    /**
     * Access the underling array of bytes for this column.
     *
     * @return
     */
    public byte[] toBytes() {
      return buffer.array();
    }
  }
}
