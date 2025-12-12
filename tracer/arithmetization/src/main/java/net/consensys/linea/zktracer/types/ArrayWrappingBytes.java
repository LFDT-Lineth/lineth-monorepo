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

import java.nio.ByteBuffer;
import java.security.MessageDigest;
import java.util.Arrays;

import io.vertx.core.buffer.Buffer;
import org.apache.tuweni.bytes.AbstractBytes;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.MutableBytes;
import org.jetbrains.annotations.NotNull;

class ArrayWrappingBytes extends AbstractBytes {

  protected final byte[] bytes;
  protected final int offset;
  protected final int length;

  ArrayWrappingBytes(byte[] bytes) {
    this(bytes, 0, bytes.length);
  }

  ArrayWrappingBytes(byte[] bytes, int offset, int length) {
    Checks.checkArgument(length >= 0, "Invalid negative length");
    if (bytes.length > 0) {
      Checks.checkElementIndex(offset, bytes.length);
    }
    Checks.checkArgument(
        offset + length <= bytes.length,
        "Provided length %s is too big: the value has only %s bytes from offset %s",
        length,
        bytes.length - offset,
        offset);

    this.bytes = bytes;
    this.offset = offset;
    this.length = length;
  }

  @Override
  public int size() {
    return length;
  }

  @Override
  public byte get(int i) {
    // Check bounds because while the array access would throw, the error message
    // would be confusing
    // for the caller.
    Checks.checkElementIndex(i, size());
    return bytes[offset + i];
  }

  @Override
  public Bytes slice(int i, int length) {
    if (i == 0 && length == this.length) {
      return this;
    }
    if (length == 0) {
      return Bytes.EMPTY;
    }

    Checks.checkElementIndex(i, this.length);
    Checks.checkArgument(
        i + length <= this.length,
        "Provided length %s is too big: the value has size %s and has only %s bytes from %s",
        length,
        this.length,
        this.length - i,
        i);

    return length == Bytes16.SIZE
        ? new ArrayWrappingBytes16(bytes, offset + i)
        : new ArrayWrappingBytes(bytes, offset + i, length);
  }

  // MUST be overridden by mutable implementations
  @Override
  public Bytes copy() {
    if (offset == 0 && length == bytes.length) {
      return this;
    }
    return new ArrayWrappingBytes(toArray());
  }

  @Override
  public MutableBytes mutableCopy() {
    return new MutableArrayWrappingBytes(toArray());
  }

  @Override
  public int commonPrefixLength(@NotNull Bytes other) {
    if (!(other instanceof ArrayWrappingBytes o)) {
      return super.commonPrefixLength(other);
    }

    int i = 0;
    while (i < length && i < o.length && bytes[offset + i] == o.bytes[o.offset + i]) {
      i++;
    }

    return i;
  }

  @Override
  public void update(MessageDigest digest) {
    digest.update(bytes, offset, length);
  }

  @Override
  public void copyTo(@NotNull MutableBytes destination, int destinationOffset) {
    if (!(destination instanceof MutableArrayWrappingBytes d)) {
      super.copyTo(destination, destinationOffset);
      return;
    }

    int size = size();
    if (size == 0) {
      return;
    }

    Checks.checkElementIndex(destinationOffset, destination.size());
    Checks.checkArgument(
        destination.size() - destinationOffset >= size,
        "Cannot copy %s bytes, destination has only %s bytes from index %s",
        size,
        destination.size() - destinationOffset,
        destinationOffset);

    System.arraycopy(bytes, offset, d.bytes, d.offset + destinationOffset, size);
  }

  @Override
  public void appendTo(ByteBuffer byteBuffer) {
    byteBuffer.put(bytes, offset, length);
  }

  @Override
  public void appendTo(Buffer buffer) {
    buffer.appendBytes(bytes, offset, length);
  }

  @Override
  public byte[] toArray() {
    return Arrays.copyOfRange(bytes, offset, offset + length);
  }

  @Override
  public byte[] toArrayUnsafe() {
    if (offset == 0 && length == bytes.length) {
      return bytes;
    }

    return toArray();
  }
}
