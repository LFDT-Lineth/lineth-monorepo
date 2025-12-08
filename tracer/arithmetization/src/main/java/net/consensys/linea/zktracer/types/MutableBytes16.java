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

import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.MutableBytes;

/** A mutable {@link Bytes16}, that is a mutable {@link Bytes} value of exactly 16 bytes. */
public interface MutableBytes16 extends MutableBytes, Bytes16 {

  /**
   * Create a new mutable 16 bytes value.
   *
   * @return A newly allocated {@link MutableBytes} value.
   */
  static MutableBytes16 create() {
    return new MutableArrayWrappingBytes16(new byte[SIZE]);
  }

  /**
   * Wrap a 16 bytes array as a mutable 16 bytes value.
   *
   * @param value The value to wrap.
   * @return A {@link MutableBytes16} wrapping {@code value}.
   * @throws IllegalArgumentException if {@code value.length != 16}.
   */
  static MutableBytes16 wrap(byte[] value) {
    Checks.checkNotNull(value);
    return new MutableArrayWrappingBytes16(value);
  }

  /**
   * Wrap the provided array as a {@link MutableBytes16}.
   *
   * <p>Note that value is not copied, only wrapped, and thus any future update to {@code value}
   * within the wrapped parts will be reflected in the returned value.
   *
   * @param value The bytes to wrap.
   * @param offset The index (inclusive) in {@code value} of the first byte exposed by the returned
   *     value. In other words, you will have {@code wrap(value, i).get(0) == value[i]}.
   * @return A {@link MutableBytes16} that exposes the bytes of {@code value} from {@code offset}
   *     (inclusive) to {@code offset + 16} (exclusive).
   * @throws IndexOutOfBoundsException if {@code offset < 0 || (value.length > 0 && offset >=
   *     value.length)}.
   * @throws IllegalArgumentException if {@code length < 0 || offset + 16 > value.length}.
   */
  static MutableBytes16 wrap(byte[] value, int offset) {
    Checks.checkNotNull(value);
    return new MutableArrayWrappingBytes16(value, offset);
  }

  /**
   * Wrap the provided value, which must be of size 16, as a {@link MutableBytes16}.
   *
   * <p>Note that value is not copied, only wrapped, and thus any future update to {@code value}
   * will be reflected in the returned value.
   *
   * @param value The bytes to wrap.
   * @return A {@link MutableBytes16} that exposes the bytes of {@code value}.
   * @throws IllegalArgumentException if {@code value.size() != 16}.
   */
  static MutableBytes16 wrap(MutableBytes value) {
    Checks.checkNotNull(value);
    if (value instanceof MutableBytes16) {
      return (MutableBytes16) value;
    }
    return DelegatingMutableBytes16.delegateTo(value);
  }

  /**
   * Wrap a slice/sub-part of the provided value as a {@link MutableBytes16}.
   *
   * <p>Note that the value is not copied, and thus any future update to {@code value} within the
   * wrapped parts will be reflected in the returned value.
   *
   * @param value The bytes to wrap.
   * @param offset The index (inclusive) in {@code value} of the first byte exposed by the returned
   *     value. In other words, you will have {@code wrap(value, i).get(0) == value.get(i)}.
   * @return A {@link Bytes16} that exposes the bytes of {@code value} from {@code offset}
   *     (inclusive) to {@code offset + 16} (exclusive).
   * @throws IndexOutOfBoundsException if {@code offset < 0 || (value.size() > 0 && offset >=
   *     value.size())}.
   * @throws IllegalArgumentException if {@code length < 0 || offset + 16 > value.size()}.
   */
  static MutableBytes16 wrap(MutableBytes value, int offset) {
    Checks.checkNotNull(value);
    if (value instanceof MutableBytes16) {
      return (MutableBytes16) value;
    }

    MutableBytes slice = value.mutableSlice(offset, SIZE);
    if (slice instanceof MutableBytes16) {
      return (MutableBytes16) slice;
    }

    return DelegatingMutableBytes16.delegateTo(slice);
  }
}
