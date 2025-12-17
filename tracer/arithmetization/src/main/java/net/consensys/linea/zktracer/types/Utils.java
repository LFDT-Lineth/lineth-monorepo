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

package net.consensys.linea.zktracer.types;

import static com.google.common.base.Preconditions.*;
import static net.consensys.linea.zktracer.Trace.LLARGE;

import java.math.BigInteger;
import java.util.ArrayList;
import org.apache.tuweni.bytes.Bytes;

public class Utils {

  /**
   * Add zeroes to the left of the {@link Bytes} to create {@link Bytes} of the given size. The
   * wantedSize must be at least the size of the Bytes.
   *
   * @param input
   * @param wantedSize
   * @return
   */
  public static Bytes leftPadTo(Bytes input, int wantedSize) {
    checkArgument(wantedSize >= input.size(), "wantedSize can't be shorter than the input size");
    return Bytes.concatenate(Bytes.repeat((byte) 0, wantedSize - input.size()), input);
  }

  public static Bytes rightPadTo(Bytes input, int wantedSize) {
    checkArgument(wantedSize >= input.size(), "wantedSize can't be shorter than the input size");
    return Bytes.concatenate(input, Bytes.repeat((byte) 0, wantedSize - input.size()));
  }

  public static Bytes leftPadToBytes16(Bytes input) {
    return leftPadTo(input, LLARGE);
  }

  public static Bytes BYTES16_ZERO = Bytes.repeat((byte) 0, LLARGE);

  public static Bytes rightPadToBytes16(Bytes input) {
    return rightPadTo(input, LLARGE);
  }

  public static int fromDataSizeToLimbNbRows(int dataSize) {
    checkArgument(dataSize >= 0, "Data size must be non-negative");
    if (dataSize == 0) {
      return 1;
    }
    final double result = Math.ceil((double) dataSize / LLARGE);
    checkArgument(result <= Integer.MAX_VALUE, "LimbNbRows must fit in an integer");
    return (int) result;
  }

  public static int fromDataSizeToLimbCtMax(int dataSize) {
    final int result = (fromDataSizeToLimbNbRows(dataSize) - 1);
    checkArgument(result >= 0, "LimbCtMax must be non-negative");
    return result;
  }

  /**
   * Create the Bit and BitDec list of the RLP pattern of an int.
   *
   * @param input
   * @param nbStep
   * @return
   */
  public static BitDecOutput bitDecomposition(int input, int nbStep) {
    final int nbStepMin = 8;
    checkArgument(nbStep >= nbStepMin, "Number of steps must be at least " + nbStepMin);
    checkArgument(input >= 0, "Input must be non-negative");

    ArrayList<Boolean> bit = new ArrayList<>(nbStep);
    ArrayList<Integer> acc = new ArrayList<>(nbStep);
    for (int i = 0; i < nbStep - nbStepMin; i++) {
      bit.add(i, false);
      acc.add(i, 0);
    }
    BitDecOutput output = new BitDecOutput(bit, acc);

    int bitAcc = 0;
    boolean bitDec = false;
    double div = 0;

    for (int i = nbStepMin - 1; i >= 0; i--) {
      div = Math.pow(2, i);
      bitAcc *= 2;

      if (input >= div) {
        bitDec = true;
        bitAcc += 1;
        input -= (int) div;
      } else {
        bitDec = false;
      }

      output.bitDecList().add(nbStep - i - 1, bitDec);
      output.bitAccList().add(nbStep - i - 1, bitAcc);
    }
    return output;
  }

  /**
   * Adds an offset to a hexadecimal string representation of a number. This method takes a
   * hexadecimal string and an integer offset, adds the offset to the number represented by the
   * hexadecimal string, and returns the result as a hexadecimal string.
   *
   * @param offset The integer offset to add to the number represented by the hexadecimal string.
   * @param hexString The hexadecimal string representation of the number to which the offset will
   *     be added.
   * @return A hexadecimal string representing the sum of the original number and the offset.
   */
  public static String addOffsetToHexString(int offset, String hexString) {
    return new BigInteger(hexString, 16).add(BigInteger.valueOf(offset)).toString(16);
  }
}
