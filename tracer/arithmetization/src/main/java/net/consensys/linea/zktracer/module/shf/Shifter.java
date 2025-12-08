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

package net.consensys.linea.zktracer.module.shf;

import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.apache.tuweni.units.bigints.UInt256;

public class Shifter {
  private static final UInt256 ALL_BITS = UInt256.MAX_VALUE;

  public static Bytes32 shift(final OpCode opCode, final Bytes32 value, final int shiftAmount) {
    return switch (opCode) {
      case SHR -> value.shiftRight(shiftAmount);
      case SHL -> value.shiftLeft(shiftAmount);
      case SAR -> sarOperation(value, shiftAmount);
      default -> Bytes32.ZERO;
    };
  }

  private static Bytes32 sarOperation(final Bytes32 value, final int shiftAmountInt) {
    Bytes shiftAmountBytes = Bytes.ofUnsignedInt(shiftAmountInt);
    final boolean isNegativeNumber = value.get(0) < 0;

    if (shiftAmountBytes.size() > 4 && shiftAmountBytes.trimLeadingZeros().size() > 4) {
      return isNegativeNumber ? ALL_BITS : UInt256.ZERO;
    } else {
      if (shiftAmountInt >= 256 || shiftAmountInt < 0) {
        return isNegativeNumber ? ALL_BITS : UInt256.ZERO;
      } else {
        // first perform standard shift right.
        Bytes32 result = value.shiftRight(shiftAmountInt);

        // if a negative number, carry through the sign.
        if (isNegativeNumber) {
          final Bytes32 significantBits = ALL_BITS.shiftLeft(256 - shiftAmountInt);
          result = result.or(significantBits);
        }

        return result;
      }
    }
  }
}
