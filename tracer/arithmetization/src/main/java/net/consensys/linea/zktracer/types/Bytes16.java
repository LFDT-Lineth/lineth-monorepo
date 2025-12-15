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

import java.security.SecureRandom;
import java.util.Random;

import org.apache.tuweni.bytes.Bytes;

/** A {@link Bytes} value that is guaranteed to contain exactly 16 bytes. */
public interface Bytes16 extends Bytes {
  /** The number of bytes in this value - i.e. 16 */
  int SIZE = 16;

  /** A {@link Bytes16} containing all zero bytes. */
  Bytes16 ZERO = wrap(new byte[SIZE]);

  /**
   * Wrap the provided byte array, which must be of length 16, as a {@link Bytes16}.
   *
   * <p>Note that value is not copied, only wrapped, and thus any future update to {@code value}
   * will be reflected in the returned value.
   *
   * @param bytes The bytes to wrap.
   * @return A {@link Bytes16} wrapping {@code value}.
   * @throws IllegalArgumentException if {@code value.length != 16}.
   */
  static Bytes16 wrap(byte[] bytes) {
    Checks.checkNotNull(bytes);
    Checks.checkArgument(bytes.length == SIZE, "Expected %s bytes but got %s", SIZE, bytes.length);
    return wrap(bytes, 0);
  }

  /**
   * Wrap a slice/sub-part of the provided array as a {@link Bytes16}.
   *
   * <p>Note that value is not copied, only wrapped, and thus any future update to {@code value}
   * within the wrapped parts will be reflected in the returned value.
   *
   * @param bytes The bytes to wrap.
   * @param offset The index (inclusive) in {@code value} of the first byte exposed by the returned
   *     value. In other words, you will have {@code wrap(value, i).get(0) == value[i]}.
   * @return A {@link Bytes16} that exposes the bytes of {@code value} from {@code offset}
   *     (inclusive) to {@code offset + 16} (exclusive).
   * @throws IndexOutOfBoundsException if {@code offset < 0 || (value.length > 0 && offset >=
   *     value.length)}.
   * @throws IllegalArgumentException if {@code length < 0 || offset + 16 > value.length}.
   */
  static Bytes16 wrap(byte[] bytes, int offset) {
    Checks.checkNotNull(bytes);
    return new ArrayWrappingBytes16(bytes, offset);
  }

  /**
   * Wrap the provided value, which must be of size 16, as a {@link Bytes16}.
   *
   * <p>Note that value is not copied, only wrapped, and thus any future update to {@code value}
   * will be reflected in the returned value.
   *
   * @param value The bytes to wrap.
   * @return A {@link Bytes16} that exposes the bytes of {@code value}.
   * @throws IllegalArgumentException if {@code value.size() != 16}.
   */
  static Bytes16 wrap(Bytes value) {
    Checks.checkNotNull(value);
    if (value instanceof Bytes16) {
      return (Bytes16) value;
    }
    Checks.checkArgument(value.size() == SIZE, "Expected %s bytes but got %s", SIZE, value.size());
    return new DelegatingBytes16(value);
  }

  /**
   * Wrap a slice/sub-part of the provided value as a {@link Bytes16}.
   *
   * <p>Note that value is not copied, only wrapped, and thus any future update to {@code value}
   * within the wrapped parts will be reflected in the returned value.
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
  static Bytes16 wrap(Bytes value, int offset) {
    Checks.checkNotNull(value);
    if (value instanceof Bytes16) {
      return (Bytes16) value;
    }
    Bytes slice = value.slice(offset, Bytes16.SIZE);
    if (slice instanceof Bytes16) {
      return (Bytes16) slice;
    }
    return new DelegatingBytes16(Bytes16.wrap(slice));
  }

  /**
   * Left pad a {@link Bytes} value with zero bytes to create a {@link Bytes16}.
   *
   * @param value The bytes value pad.
   * @return A {@link Bytes16} that exposes the left-padded bytes of {@code value}.
   * @throws IllegalArgumentException if {@code value.size() > 16}.
   */
  static Bytes16 leftPad(Bytes value) {
    Checks.checkNotNull(value);
    if (value instanceof Bytes16) {
      return (Bytes16) value;
    }
    Checks.checkArgument(
        value.size() <= SIZE, "Expected at most %s bytes but got %s", SIZE, value.size());
    MutableBytes16 result = MutableBytes16.create();
    value.copyTo(result, SIZE - value.size());
    return result;
  }

  /**
   * Right pad a {@link Bytes} value with zero bytes to create a {@link Bytes16}.
   *
   * @param value The bytes value pad.
   * @return A {@link Bytes16} that exposes the right-padded bytes of {@code value}.
   * @throws IllegalArgumentException if {@code value.size() > 16}.
   */
  static Bytes16 rightPad(Bytes value) {
    Checks.checkNotNull(value);
    if (value instanceof Bytes16) {
      return (Bytes16) value;
    }
    Checks.checkArgument(
        value.size() <= SIZE, "Expected at most %s bytes but got %s", SIZE, value.size());
    MutableBytes16 result = MutableBytes16.create();
    value.copyTo(result, 0);
    return result;
  }

  /**
   * Parse a hexadecimal string into a {@link Bytes16}.
   *
   * <p>This method is lenient in that {@code str} may of an odd length, in which case it will
   * behave exactly as if it had an additional 0 in front.
   *
   * @param str The hexadecimal string to parse, which may or may not start with "0x". That
   *     representation may contain less than 16 bytes, in which case the result is left padded with
   *     zeros (see {@link #fromHexStringStrict} if this is not what you want).
   * @return The value corresponding to {@code str}.
   * @throws IllegalArgumentException if {@code str} does not correspond to a valid hexadecimal
   *     representation or contains more than 16 bytes.
   */
  static Bytes16 fromHexStringLenient(CharSequence str) {
    Checks.checkNotNull(str);
    return wrap(BytesValues.fromRawHexString(str, SIZE, true));
  }

  /**
   * Parse a hexadecimal string into a {@link Bytes16}.
   *
   * <p>This method is strict in that {@code str} must of an even length.
   *
   * @param str The hexadecimal string to parse, which may or may not start with "0x". That
   *     representation may contain less than 16 bytes, in which case the result is left padded with
   *     zeros (see {@link #fromHexStringStrict} if this is not what you want).
   * @return The value corresponding to {@code str}.
   * @throws IllegalArgumentException if {@code str} does not correspond to a valid hexadecimal
   *     representation, is of an odd length, or contains more than 16 bytes.
   */
  static Bytes16 fromHexString(CharSequence str) {
    Checks.checkNotNull(str);
    return wrap(BytesValues.fromRawHexString(str, SIZE, false));
  }

  /**
   * Generate random bytes.
   *
   * @return A value containing random bytes.
   */
  static Bytes16 random() {
    return random(new SecureRandom());
  }

  /**
   * Generate random bytes.
   *
   * @param generator The generator for random bytes.
   * @return A value containing random bytes.
   */
  static Bytes16 random(Random generator) {
    byte[] array = new byte[16];
    generator.nextBytes(array);
    return wrap(array);
  }

  /**
   * Parse a hexadecimal string into a {@link Bytes16}.
   *
   * <p>This method is extra strict in that {@code str} must have an even length and the provided
   * representation must have exactly 16 bytes.
   *
   * @param str The hexadecimal string to parse, which may or may not start with "0x".
   * @return The value corresponding to {@code str}.
   * @throws IllegalArgumentException if {@code str} does not correspond to a valid hexadecimal
   *     representation, is of an odd length or does not contain exactly 16 bytes.
   */
  static Bytes16 fromHexStringStrict(CharSequence str) {
    Checks.checkNotNull(str);
    return wrap(BytesValues.fromRawHexString(str, -1, false));
  }

  @Override
  default int size() {
    return SIZE;
  }

  /**
   * Return a bit-wise AND of these bytes and the supplied bytes.
   *
   * @param other The bytes to perform the operation with.
   * @return The result of a bit-wise AND.
   */
  default Bytes16 and(Bytes16 other) {
    return and(other, MutableBytes16.create());
  }

  /**
   * Return a bit-wise OR of these bytes and the supplied bytes.
   *
   * @param other The bytes to perform the operation with.
   * @return The result of a bit-wise OR.
   */
  default Bytes16 or(Bytes16 other) {
    return or(other, MutableBytes16.create());
  }

  /**
   * Return a bit-wise XOR of these bytes and the supplied bytes.
   *
   * @param other The bytes to perform the operation with.
   * @return The result of a bit-wise XOR.
   */
  default Bytes16 xor(Bytes16 other) {
    return xor(other, MutableBytes16.create());
  }

  @Override
  default Bytes16 not() {
    return not(MutableBytes16.create());
  }

  @Override
  default Bytes16 shiftRight(int distance) {
    return shiftRight(distance, MutableBytes16.create());
  }

  @Override
  default Bytes16 shiftLeft(int distance) {
    return shiftLeft(distance, MutableBytes16.create());
  }

  @Override
  Bytes16 copy();

  @Override
  MutableBytes16 mutableCopy();
}
