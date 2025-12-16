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

package net.consensys.linea.zktracer.module.rlputilsOld;

import static com.google.common.base.Preconditions.*;

import java.math.BigInteger;
import java.util.ArrayList;
import org.apache.tuweni.bytes.Bytes;

public class Pattern {
  /**
   * Returns the size of RLP(something) where something is of size inputSize (!=1) (it can be ZERO
   * though).
   */
  public static int outerRlpSize(int inputSize) {
    int rlpSize = inputSize;
    if (inputSize == 1) {
      throw new IllegalArgumentException("Input size must be different from 1");
    } else {
      rlpSize += 1;
      if (inputSize >= 56) {
        rlpSize += Bytes.minimalBytes(inputSize).size();
      }
    }
    return rlpSize;
  }

  public static int innerRlpSize(int rlpSize) {
    // If rlpSize >1, return size(something) where rlpSize = size(RLP(something)).
    checkArgument(rlpSize >= 2, "rlpSize should be at least 2 to get its inner size");
    int output = rlpSize;

    if (rlpSize < 57) {
      output -= 1;
    } else if (rlpSize == 57) {
      throw new RuntimeException("can't be of size 57");
    } else if (rlpSize < 258) {
      output -= 2;
    } else if (rlpSize == 258) {
      throw new RuntimeException("can't be of size 258");
    } else {
      for (int i = 1; i < 8; i++) {
        if ((rlpSize - 2 - i >= Math.pow(2, 8 * i))
            && (rlpSize - i - 1 <= Math.pow(2, 8 * (i + 1)))) {
          output -= (2 + i);
        } else if (rlpSize == Math.pow(2, i) + 1 + i) {
          throw new RuntimeException("can't be this size");
        }
      }
    }
    return output;
  }

  /**
   * Create the Power and AccSize list of the ByteCountAndPower RLP pattern.
   *
   * @param inputByteLen represents the number of meaningful bytes of inputByte, i.e. without the
   *     zero left padding
   * @param nbStep
   * @return
   */
  public static ByteCountAndPowerOutput byteCounting(int inputByteLen, int nbStep) {

    ArrayList<BigInteger> powerInit = new ArrayList<>(nbStep);
    ArrayList<Integer> acc = new ArrayList<>(nbStep);
    ByteCountAndPowerOutput output = new ByteCountAndPowerOutput(powerInit, acc);

    BigInteger power;
    int accByteSize = 0;
    int offset = 16 - nbStep;

    if (inputByteLen == nbStep) {
      power = BigInteger.valueOf(256).pow(offset);
      accByteSize = 1;
    } else {
      offset += 1;
      power = BigInteger.valueOf(256).pow(offset);
    }

    output.powerList().add(0, power);
    output.accByteSizeList().add(0, accByteSize);

    for (int i = 1; i < nbStep; i++) {
      if (inputByteLen + i < nbStep) {
        power = power.multiply(BigInteger.valueOf(256));
      } else {
        accByteSize += 1;
      }
      output.powerList().add(i, power);
      output.accByteSizeList().add(i, accByteSize);
    }
    return output;
  }
}
