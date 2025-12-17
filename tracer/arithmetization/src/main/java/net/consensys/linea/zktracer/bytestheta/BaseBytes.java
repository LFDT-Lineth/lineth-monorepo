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

package net.consensys.linea.zktracer.bytestheta;

import static com.google.common.base.Preconditions.checkArgument;
import static net.consensys.linea.zktracer.Trace.LLARGE;
import static net.consensys.linea.zktracer.types.Conversions.longToBytes32;

import lombok.EqualsAndHashCode;
import net.consensys.linea.zktracer.types.UnsignedByte;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.apache.tuweni.bytes.MutableBytes32;

/**
 * The `BaseBytes` class provides a base implementation for manipulating 256-bit (32-byte) blocks of
 * data.
 */
@EqualsAndHashCode(onlyExplicitlyIncluded = true, callSuper = false)
public class BaseBytes implements HighLowBytes {
  /** The size in bytes of the high and low sections of the 256-bit block. */
  private static final int LOW_HIGH_SIZE = 16;

  /**
   * The mutable `Bytes32` object that stores the 256-bit block of data. Equals and hashCode must
   * only be computed on the numeric value wrapped by this class, so that sets of operations
   * parameterized by these values hash correctly.
   */
  @EqualsAndHashCode.Include protected MutableBytes32 bytes32;

  /**
   * This static factory method returns a new instance of the `BaseBytes` class, initialized with
   * the given `arg` parameter.
   *
   * @param arg The `Bytes32` parameter used to initialize the new `BaseBytes` instance.
   * @return A new instance of the `BaseBytes` class, initialized with the given `arg` parameter.
   */
  public static BaseBytes fromBytes32(Bytes32 arg) {
    return new BaseBytes(arg);
  }

  public static BaseBytes fromInt(int arg) {
    return new BaseBytes(longToBytes32(arg));
  }

  /**
   * The constructor for the `BaseBytes` class. It takes a parameter of type `Bytes32` called `arg`,
   * which is used to initialize the object.
   *
   * @param arg The `Bytes32` parameter used to initialize the object.
   */
  protected BaseBytes(final Bytes32 arg) {
    bytes32 = arg.mutableCopy();
  }

  /**
   * Returns a new `Bytes16` object that is the high section (first 16 bytes) of the bytes32`
   * instance variable.
   */
  @Override
  public Bytes getHigh() {
    final Bytes output = bytes32.slice(0, LOW_HIGH_SIZE);
    checkArgument(output.size() == LLARGE, "output should be of size 16");
    return output;
  }

  /**
   * Returns a new `Bytes16` object that is the low section (last 16 bytes) of the `bytes32`
   * instance variable.
   */
  @Override
  public Bytes getLow() {
    final Bytes output = bytes32.slice(LOW_HIGH_SIZE);
    checkArgument(output.size() == LLARGE, "output should be of size 16");
    return output;
  }

  /**
   * Returns the byte at the specified `index` in the `bytes32` instance variable.
   *
   * @param index The index of the byte to be returned.
   * @return The byte at the specified index in the `bytes32` instance variable.
   */
  public byte getByte(int index) {
    return bytes32.get(index);
  }

  /**
   * Returns the `Bytes32` object that stores the 256-bit block of data.
   *
   * @return The `Bytes32` object that stores the 256-bit block of data.
   */
  public Bytes32 getBytes32() {
    return bytes32;
  }

  /**
   * Returns `true` if the `bytes32` instance variable contains all zeros, and `false` otherwise.
   *
   * @return `true` if the `bytes32` instance variable contains all zeros, and `false` otherwise.
   */
  public boolean isZero() {
    return bytes32.isZero();
  }

  /**
   * Add operation on an instance of {@link UnsignedByte}.
   *
   * @param other right hand side of and operation.
   * @return the result of add operation as an instance of {@link UnsignedByte}.
   */
  public BaseBytes and(final BaseBytes other) {
    return new BaseBytes(bytes32.and(other.bytes32));
  }

  /**
   * Or operation on an instance of {@link UnsignedByte}.
   *
   * @param other right hand side of or operation.
   * @return the result of or operation as an instance of {@link UnsignedByte}.
   */
  public BaseBytes or(final BaseBytes other) {
    return new BaseBytes(bytes32.or(other.bytes32));
  }

  /**
   * Xor operation on an instance of {@link UnsignedByte}.
   *
   * @param other right hand side of xor operation.
   * @return the result of xor operation as an instance of {@link UnsignedByte}.
   */
  public BaseBytes xor(final BaseBytes other) {
    return new BaseBytes(bytes32.xor(other.bytes32));
  }

  /**
   * Not operation on an instance of {@link UnsignedByte}.
   *
   * @return the result of not operation as an instance of {@link UnsignedByte}.
   */
  public BaseBytes not() {
    return new BaseBytes(bytes32.not());
  }
}
