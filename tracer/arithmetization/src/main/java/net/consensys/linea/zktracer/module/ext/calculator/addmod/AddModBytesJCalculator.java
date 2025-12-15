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

package net.consensys.linea.zktracer.module.ext.calculator.addmod;

import static net.consensys.linea.zktracer.module.Util.uInt64ToBytes;

import net.consensys.linea.zktracer.bytestheta.BaseTheta;
import net.consensys.linea.zktracer.bytestheta.BytesArray;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.units.bigints.UInt256;

/**
 * Provides functionality to compute Js by adding two Bytes objects and converting the result into a
 * byte array matrix.
 */
public class AddModBytesJCalculator {
  /**
   * Computes the Js by adding two Bytes objects, converting the result into a byte array matrix.
   *
   * @param arg1 the first Bytes object to be added
   * @param arg2 the second Bytes object to be added
   * @return a BytesArray object containing the computed Js as a byte array matrix
   */
  public static BytesArray computeJs(Bytes arg1, Bytes arg2) {
    byte[][] jBytes = new byte[8][8];
    UInt256 sum = UInt256.fromBytes(arg1).add(UInt256.fromBytes(arg2));
    BaseTheta sumBaseTheta = BaseTheta.fromBytes32(sum);

    for (int k = 0; k < 4; k++) {
      jBytes[k] = sumBaseTheta.get(k).toArray();
    }

    if (UInt256.fromBytes(arg1).compareTo(sum) > 0) {
      jBytes[4] = uInt64ToBytes(1);
    }

    return new BytesArray(jBytes);
  }
}
