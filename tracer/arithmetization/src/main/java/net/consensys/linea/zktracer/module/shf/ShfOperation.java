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

import lombok.EqualsAndHashCode;
import lombok.Getter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.container.ModuleOperation;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes32;

@Accessors(fluent = true)
@EqualsAndHashCode(onlyExplicitlyIncluded = true, callSuper = false)
final class ShfOperation extends ModuleOperation {
  @EqualsAndHashCode.Include @Getter private final OpCode opCode;
  @EqualsAndHashCode.Include @Getter private final Bytes32 arg1;
  @EqualsAndHashCode.Include @Getter private final Bytes32 arg2;

  public ShfOperation(OpCode opCode, Bytes32 arg1, Bytes32 arg2) {
    this.opCode = opCode;
    this.arg1 = arg1;
    this.arg2 = arg2;
  }

  public void trace(Trace.Shf trace) {
    // compute result
    Bytes32 res = Shifter.shift(this.opCode, this.arg2, shiftBy(this.arg1));
    // trace function instance
    trace.inst(opCode.unsignedByteValue()).arg1(this.arg1).arg2(this.arg2).res(res).validateRow();
  }

  @Override
  protected int computeLineCount() {
    return 1;
  }

  private static int shiftBy(final Bytes32 arg) {
    return allButLastByteZero(arg) ? arg.get(31) & 0xff : 256;
  }

  private static boolean allButLastByteZero(final Bytes32 bytes) {
    for (int i = 0; i < 31; i++) {
      // careful: bytes are signed
      if (bytes.get(i) != 0) {
        return false;
      }
    }

    return true;
  }
}
