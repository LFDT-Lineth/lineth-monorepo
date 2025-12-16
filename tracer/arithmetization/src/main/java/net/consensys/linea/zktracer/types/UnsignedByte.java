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

import com.fasterxml.jackson.annotation.JsonValue;
import java.math.BigInteger;

/** Represents an unsigned byte type. */
public class UnsignedByte {
  public static final UnsignedByte ZERO = UnsignedByte.of(0);
  public static final UnsignedByte[] EMPTY_BYTES16 = new UnsignedByte[16];
  private final short unsignedByte;

  private UnsignedByte(final short unsignedByte) {
    this.unsignedByte = unsignedByte;
  }

  /**
   * Factory method for the creation of an {@link UnsignedByte} from a primitive byte type.
   *
   * @param b parameter of type byte.
   * @return an instance of {@link UnsignedByte}.
   */
  public static UnsignedByte of(final byte b) {
    final short unsignedB = (short) (b & 0xff);

    checkLength(unsignedB);

    return new UnsignedByte(unsignedB);
  }

  /**
   * Factory method for the creation of an {@link UnsignedByte} from a primitive int type.
   *
   * @param b parameter of type int.
   * @return an instance of {@link UnsignedByte}.
   */
  public static UnsignedByte of(final int b) {
    checkLength(b);

    return new UnsignedByte((short) b);
  }

  /**
   * Factory method for the creation of an {@link UnsignedByte} from a primitive long type.
   *
   * @param b parameter of type long.
   * @return an instance of {@link UnsignedByte}.
   */
  public static UnsignedByte of(final long b) {
    checkLength(b);

    return new UnsignedByte((short) b);
  }

  /**
   * Left bit shift operation on an instance of {@link UnsignedByte}.
   *
   * @param shiftAmount amount of bits represented as a {@link UnsignedByte} type to shift left.
   * @return a bit shifted instance of {@link UnsignedByte}.
   */
  public UnsignedByte shiftLeft(final UnsignedByte shiftAmount) {
    return shiftLeft(shiftAmount.toInteger());
  }

  /**
   * Left bit shift operation on an instance of {@link UnsignedByte}.
   *
   * @param shiftAmount amount of bits represented as an int type to shift left.
   * @return a bit shifted instance of {@link UnsignedByte}.
   */
  public UnsignedByte shiftLeft(final int shiftAmount) {
    return new UnsignedByte((short) ((unsignedByte << shiftAmount) & 0xff));
  }

  /**
   * Right bit shift operation on an instance of {@link UnsignedByte}.
   *
   * @param shiftAmount amount of bits represented as a {@link UnsignedByte} type to shift right.
   * @return a bit shifted instance of {@link UnsignedByte}.
   */
  public UnsignedByte shiftRight(final UnsignedByte shiftAmount) {
    return shiftRight(shiftAmount.toInteger());
  }

  /**
   * Right bit shift operation on an instance of {@link UnsignedByte}.
   *
   * @param shiftAmount amount of bits represented as an int type to shift right.
   * @return a bit shifted instance of {@link UnsignedByte}.
   */
  public UnsignedByte shiftRight(final int shiftAmount) {
    return new UnsignedByte((short) ((unsignedByte >> shiftAmount) & 0xff));
  }

  /**
   * Mod operation on an instance of {@link UnsignedByte}.
   *
   * @param m right hand side of division by mod operation.
   * @return the result of mod division as an instance of {@link UnsignedByte}.
   */
  public UnsignedByte mod(final int m) {
    return new UnsignedByte((short) (unsignedByte % m));
  }

  @JsonValue
  public int toInteger() {
    return unsignedByte;
  }

  public byte toByte() {
    return (byte) unsignedByte;
  }

  /**
   * Converts to {@link BigInteger} type.
   *
   * @return the unsigned byte to converted to {@link BigInteger}
   */
  public BigInteger toBigInteger() {
    return BigInteger.valueOf(this.toInteger());
  }

  private static void checkLength(final long b) {
    if (b < 0 || b > 255) {
      throw new IllegalArgumentException("Unsigned byte value must be between 0 - 255. Is " + b);
    }
  }

  /**
   * Retrieves the value of the most significant bit (MSB) at a specified position of an instance of
   * {@link UnsignedByte}.
   *
   * @param i The position of the MSB to retrieve, counting from the MSB as position 0.
   * @return true if the most significant bit at the specified position is set (1), false otherwise.
   * @throws IndexOutOfBoundsException if i < 0 or i >= 8.
   */
  public boolean get(int i) {
    if (i < 0 || i >= 8) {
      throw new IndexOutOfBoundsException("Index must be between 0 - 7. Is " + i);
    }
    String ub = Integer.toBinaryString(this.toInteger());
    if (ub.length() < 8) {
      ub = "0".repeat(8 - ub.length()) + ub;
    }
    return ub.charAt(i) == '1';
  }

  /**
   * Retrieves a slice of bits starting from a specified index of an instance of {@link
   * UnsignedByte}.
   *
   * @param i The index from which to start retrieving the slice.
   * @param length The length of the slice to retrieve, representing the number of bits.
   * @return An integer representing the slice of bits starting from the specified index.
   * @throws IllegalArgumentException if length < 0.
   * @throws IndexOutOfBoundsException if i < 0 or i >= 8 or i + length > 8.
   */
  public int slice(int i, int length) {
    if (length < 0) {
      throw new IllegalArgumentException("Length must be non-negative. Is " + length);
    }
    if (i < 0 || i >= 8 || i + length > 8) {
      throw new IndexOutOfBoundsException("Index out of bounds. i=" + i + ", length=" + length);
    }
    String ub = Integer.toBinaryString(this.toInteger());
    if (ub.length() < 8) {
      ub = "0".repeat(8 - ub.length()) + ub;
    }
    return Integer.parseInt(ub.substring(i, i + length), 2);
  }
}
