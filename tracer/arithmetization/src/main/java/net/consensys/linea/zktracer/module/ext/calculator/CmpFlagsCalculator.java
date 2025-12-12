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

import net.consensys.linea.zktracer.bytestheta.BytesArray;
import org.apache.tuweni.units.bigints.UInt256;

/** A utility class for computing comparison flags for extended modular arithmetic operations. */
public class CmpFlagsCalculator {

  /**
   * Computes the comparison flags for the given cBytes and rBytes values.
   *
   * @param cBytes the cBytes value.
   * @param rBytes the rBytes value.
   * @return the comparison flags.
   */
  static boolean[] computeComparisonFlags(BytesArray cBytes, BytesArray rBytes) {
    boolean[] cmp = new boolean[8];
    for (int i = 0; i < 4; i++) {
      UInt256 c = UInt256.fromBytes(cBytes.get(i));
      UInt256 r = UInt256.fromBytes(rBytes.get(i));
      boolean cGreaterThanR = c.compareTo(r) > 0;
      if (cGreaterThanR) {
        cmp[i] = true;
      } else {
        cmp[4 + i] = c.equals(r);
      }
    }
    return cmp;
  }
}
