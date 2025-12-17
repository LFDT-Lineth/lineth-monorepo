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

import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;

/**
 * Represents a BaseTheta data structure, which is an extension of BytesArray, with support for high
 * and low bytes' manipulation. It is organized as an array of 4 Bytes instances, each containing 8
 * bytes of a Bytes32 input.
 */
public class BaseTheta extends BytesArray implements HighLowBytes {

  /**
   * Constructs a new BaseTheta instance by slicing a given Bytes32 into 4 Bytes instances.
   *
   * @param arg A Bytes32 input.
   */
  public BaseTheta(final Bytes32 arg) {
    super(arg);

    for (int k = 0; k < 4; k++) {
      Bytes bytes = arg.slice(8 * k, 8);
      set((3 - k), bytes);
    }
  }

  /**
   * Creates a new BaseTheta instance from a given Bytes32 input.
   *
   * @param arg A Bytes32 input.
   * @return A new BaseTheta instance.
   */
  public static BaseTheta fromBytes32(Bytes32 arg) {
    return new BaseTheta(arg);
  }

  /**
   * Returns a Bytes32 instance representing the concatenated bytes in the BaseTheta.
   *
   * @return A Bytes32 instance.
   */
  public Bytes32 getBytes32() {
    return Bytes32.wrap(
        Bytes.concatenate(bytesArray[0], bytesArray[1], bytesArray[2], bytesArray[3]));
  }

  /**
   * Returns a new `Bytes16` object that is the concatenation of the third and second 64-bit
   * sections of the `bytes32` instance variable.
   *
   * @return A new `Bytes16` object that is the concatenation of the third and second 64-bit
   *     sections of the `bytes32` instance variable.
   */
  @Override
  public Bytes getHigh() {
    final Bytes output = Bytes.concatenate(bytesArray[3], bytesArray[2]);
    checkArgument(output.size() == LLARGE, "output should be of size 16");
    return output;
  }

  /**
   * Returns a new `Bytes16` object that is the concatenation of the first and second 64-bit
   * sections of the `bytes32` instance variable.
   *
   * @return A new `Bytes16` object that is the concatenation of the first and second 64-bit
   *     sections of the `bytes32` instance variable.
   */
  @Override
  public Bytes getLow() {
    final Bytes output = Bytes.concatenate(bytesArray[1], bytesArray[0]);
    checkArgument(output.size() == LLARGE, "output should be of size 16");
    return output;
  }

  /**
   * Returns the byte at the specified position within the specified Bytes instance in the array.
   *
   * @param i The index of the Bytes instance.
   * @param j The index of the byte within the Bytes instance.
   * @return The byte at the specified position.
   */
  public byte get(final int i, final int j) {
    return bytesArray[i].get(j);
  }

  /**
   * Returns a range of bytes from a specified Bytes instance within the array.
   *
   * @param i The index of the Bytes instance.
   * @param start The start index of the range (inclusive).
   * @param length The length of the range.
   * @return A new Bytes instance containing the specified range of bytes.
   */
  public Bytes getRange(final int i, final int start, final int length) {
    return bytesArray[i].slice(start, length);
  }

  /**
   * Sets the byte at the specified position within the specified Bytes instance in the array.
   *
   * @param i The index of the Bytes instance.
   * @param j The index of the byte within the Bytes instance.
   * @param b The byte to be set.
   */
  public void set(int i, int j, byte b) {
    bytesArray[i].set(j, b);
  }

  // set the whole chunk of bytes at the given index.
  // assumes index is one of 0,1,2,3
  // assumes length of bytes is 8
  public void setChunk(int index, Bytes bytes) {
    set(index, bytes);
  }

  @Override
  public String toString() {
    return "hi=%s, lo=%s\n  0 1 2 3\n  %s %s %s %s "
        .formatted(getHigh(), getLow(), get(0), get(1), get(2), get(3));
  }
}
