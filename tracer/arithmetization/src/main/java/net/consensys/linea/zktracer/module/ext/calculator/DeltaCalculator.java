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

package net.consensys.linea.zktracer.module.ext.calculator;

import net.consensys.linea.zktracer.bytestheta.BaseTheta;
import net.consensys.linea.zktracer.bytestheta.BytesArray;
import org.apache.tuweni.bytes.Bytes32;
import org.apache.tuweni.units.bigints.UInt256;

/** A utility class for computing delta values for extended modular arithmetic operations. */
public class DeltaCalculator {
  /**
   * Computes the delta values for the given cBytes and rBytes values.
   *
   * @param cBytes the cBytes value.
   * @param rBytes the rBytes value.
   * @return the delta values.
   */
  public static BaseTheta computeDeltas(BytesArray cBytes, BytesArray rBytes) {
    BaseTheta deltaBytes = BaseTheta.fromBytes32(Bytes32.ZERO);

    for (int i = 0; i < 4; i++) {
      UInt256 c = UInt256.fromBytes(cBytes.get(i));
      UInt256 r = UInt256.fromBytes(rBytes.get(i));
      UInt256 delta;

      boolean cGreaterThanR = c.compareTo(r) > 0;
      if (cGreaterThanR) {
        delta = c.subtract(r).subtract(UInt256.ONE);
      } else {
        delta = r.subtract(c);
      }

      // Convert the delta value to a byte array and store it in the ith element of deltaBytes
      BaseTheta truc = (BaseTheta.fromBytes32(delta));
      deltaBytes.set(i, truc.get(0));
    }

    return deltaBytes;
  }
}
