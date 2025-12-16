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

package net.consensys.linea.zktracer.module.ext.calculator.mulmod;

import static net.consensys.linea.zktracer.module.Util.uInt64ToBytes;

import java.math.BigInteger;
import net.consensys.linea.zktracer.bytestheta.BytesArray;
import net.consensys.linea.zktracer.module.ext.BigIntegerConverter;
import org.apache.tuweni.bytes.Bytes32;

public class MulModBytesJCalculator {
  /**
   * Computes the product of two Bytes32 arguments and returns the result as a BytesArray.
   *
   * @param arg1 The first Bytes32 argument.
   * @param arg2 The second Bytes32 argument.
   * @return The product of the two arguments as a BytesArray.
   */
  public static BytesArray computeJs(Bytes32 arg1, Bytes32 arg2) {
    byte[][] jBytes = new byte[8][8];

    BigInteger arg1UInt = arg1.toUnsignedBigInteger();
    BigInteger arg2UInt = arg2.toUnsignedBigInteger();
    // Compute the product of the two unsigned BigIntegers.
    BigInteger prod = arg1UInt.multiply(arg2UInt);

    long[] prodArray = BigIntegerConverter.toLongArray(prod);
    // Iterate through the long array and convert each element to a byte array.
    for (int k = 0; k < prodArray.length; k++) {
      jBytes[k] = uInt64ToBytes(prodArray[k]);
    }

    return new BytesArray(jBytes);
  }
}
