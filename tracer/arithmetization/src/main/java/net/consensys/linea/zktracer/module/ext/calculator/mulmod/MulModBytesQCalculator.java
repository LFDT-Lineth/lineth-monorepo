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

import static net.consensys.linea.zktracer.module.Util.isUInt256;
import static net.consensys.linea.zktracer.module.Util.uInt64ToBytes;

import java.math.BigInteger;
import net.consensys.linea.zktracer.bytestheta.BaseTheta;
import net.consensys.linea.zktracer.bytestheta.BytesArray;
import net.consensys.linea.zktracer.module.UtilCalculator;
import net.consensys.linea.zktracer.module.ext.BigIntegerConverter;
import org.apache.tuweni.bytes.Bytes32;

public class MulModBytesQCalculator {
  /**
   * Computes the quotient of the product of arg1 and arg2 divided by arg3, all of Bytes32 type, and
   * returns the result as a BytesArray. (arg1 * arg2 ) / arg3
   *
   * @param arg1 The first Bytes32 argument.
   * @param arg2 The second Bytes32 argument.
   * @param arg3 The third Bytes32 argument.
   * @return The quotient of the product of arg1 and arg2 divided by arg3 as a BytesArray.
   */
  public static BytesArray computeQs(Bytes32 arg1, Bytes32 arg2, Bytes32 arg3) {
    byte[][] qBytes = new byte[8][8];

    BigInteger prod = UtilCalculator.calculateProduct(arg1, arg2);

    if (isUInt256(prod)) {
      BigInteger quotBigInteger = UtilCalculator.calculateQuotient(prod, arg3);
      BaseTheta quotBaseTheta = UtilCalculator.convertToBaseTheta(quotBigInteger);

      for (int i = 0; i < 4; i++) {
        // Copy the BaseTheta byte arrays into the result byte array.
        qBytes[i] = quotBaseTheta.get(i).toArray();
      }
    } else {
      // Divide the product by arg3 and convert the quotient to a byte array.
      BigInteger[] divAndRemainder = prod.divideAndRemainder(arg3.toUnsignedBigInteger());
      long[] quot = BigIntegerConverter.toLongArray(divAndRemainder[0]);
      for (int k = 0; k < quot.length; k++) {
        qBytes[k] = uInt64ToBytes(quot[k]);
      }
    }
    // Return the result as a BytesArray.
    return new BytesArray(qBytes);
  }
}
