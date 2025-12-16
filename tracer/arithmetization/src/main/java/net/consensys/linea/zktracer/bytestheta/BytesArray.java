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

import java.util.Arrays;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.apache.tuweni.bytes.MutableBytes;

/** Represents an array of mutable byte arrays. */
public class BytesArray {

  /** The internal array of MutableBytes. */
  final MutableBytes[] bytesArray;

  /**
   * Constructs a BytesArray from a given two-dimensional byte array.
   *
   * @param bytes a two-dimensional byte array
   */
  public BytesArray(final byte[][] bytes) {
    int arraySize = bytes.length;
    bytesArray = new MutableBytes[arraySize];

    for (int i = 0; i < arraySize; i++) {
      bytesArray[i] = MutableBytes.wrap(bytes[i]);
    }
  }

  /**
   * Constructs a BytesArray with a specified size and initializes it with zeros.
   *
   * @param size the size of the BytesArray
   */
  public BytesArray(int size) {
    bytesArray = new MutableBytes[size];
    Arrays.fill(bytesArray, MutableBytes.of(0, 0, 0, 0, 0, 0, 0, 0));
  }

  /**
   * Returns the size of the BytesArray.
   *
   * @return the size of the BytesArray
   */
  public int size() {
    return bytesArray.length;
  }

  /**
   * Constructs a BytesArray from a given Bytes32 object.
   *
   * @param bytes32 a Bytes32 object
   */
  public BytesArray(Bytes32 bytes32) {
    bytesArray = new MutableBytes[4];

    for (int i = 0; i < 4; i++) {
      bytesArray[i] = MutableBytes.wrap(bytes32.slice(i * 8, 8).toArray());
    }
  }

  /**
   * Retrieves a MutableBytes object at the specified index.
   *
   * @param index the index of the MutableBytes object
   * @return the MutableBytes object at the specified index
   */
  public MutableBytes get(int index) {
    return bytesArray[index];
  }

  /**
   * Sets a new Bytes value at the specified index.
   *
   * @param index the index of the MutableBytes object to be replaced
   * @param bytes the new Bytes value
   */
  public void set(int index, Bytes bytes) {
    checkArgument(bytes.size() == 8, "bytes size must be 8");
    bytesArray[index] = MutableBytes.wrap(bytes.toArray());
  }

  /**
   * Returns the internal array of MutableBytes objects.
   *
   * @return the internal array of MutableBytes objects
   */
  public Bytes[] getBytesArray() {
    return bytesArray;
  }

  /**
   * Retrieves a range of Bytes objects from the specified start index to the end index.
   *
   * @param start the start index of the range
   * @param end the end index of the range
   * @return an array of Bytes objects in the specified range
   */
  public Bytes[] getBytesRange(final int start, final int end) {
    int rangeSize = end - start + 1;
    Bytes[] bytes = new Bytes[rangeSize];
    for (int i = 0; i < rangeSize; i++) {
      bytes[i] = get(start + i);
    }

    return bytes;
  }
}
