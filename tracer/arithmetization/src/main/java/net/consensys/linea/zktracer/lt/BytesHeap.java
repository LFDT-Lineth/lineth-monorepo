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
import java.util.Arrays;
import java.util.HashMap;

/** BytesHeap provides a representation of a "heap" for use with the LTv2 file format. */
public class BytesHeap {
  private final HashMap<Entry, Integer> cache = new HashMap<>();
  private byte[] bytes = new byte[256];
  private byte[] lengths = new byte[256];
  private int length;

  public BytesHeap() {
    for (int i = 0; i != 65536; i++) {
      byte[] bytes = Util.long2TruncatedBytes(i);
      alloc(new Entry(bytes));
    }
  }

  /**
   * Insert a new element into this heap, returning its index.
   *
   * @param key
   * @return
   */
  public int insert(byte[] key) {
    final Entry e = new Entry(key);
    // Look for an exact match
    final Integer val = this.cache.get(e);
    // Check for exact match.
    if (val == null) {
      return alloc(e);
    }
    //
    return val;
  }

  /**
   * Get the bytes chunk at a given index.
   *
   * @param index
   * @return
   */
  public byte[] get(int index) {
    int n = lengths[index];
    byte[] slice = new byte[n];
    System.arraycopy(bytes, index, slice, 0, n);
    return slice;
  }

  /**
   * Encode this heap into a given set of bytes.
   *
   * @return
   */
  public byte[] toBytes() {
    int len = 4 + (2 * length);
    ByteBuffer buffer = ByteBuffer.allocate(len);
    // write heap length
    buffer.putInt(length);
    // write lengths
    buffer.put(lengths, 0, length);
    // write bytes
    buffer.put(bytes, 0, length);
    //
    return buffer.array();
  }

  private int alloc(Entry e) {
    int val = length;
    int size = Math.max(e.bytes.length, 1);
    // Ensure enough capacity
    if (length + size > bytes.length) {
      bytes = Arrays.copyOf(bytes, 2 * (size + length + 1));
      lengths = Arrays.copyOf(lengths, 2 * (size + length + 1));
    }
    // Configure entry
    this.lengths[val] = (byte) e.bytes.length;
    this.length += size;
    // Copy over bytes
    System.arraycopy(e.bytes, 0, this.bytes, val, e.bytes.length);
    // Update the cache
    this.cache.put(e, val);
    // Done
    return val;
  }

  private static final class Entry {
    private final byte[] bytes;

    public Entry(byte[] bytes) {
      this.bytes = bytes;
    }

    @Override
    public boolean equals(Object o) {
      if (o instanceof Entry e) {
        return Arrays.equals(bytes, e.bytes);
      }
      //
      return false;
    }

    @Override
    public int hashCode() {
      return Arrays.hashCode(bytes);
    }
  }
}
