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

package net.consensys.linea.zktracer.module.mod;

import lombok.EqualsAndHashCode;
import lombok.Getter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.container.ModuleOperation;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes32;
import org.apache.tuweni.units.bigints.UInt256;

@Accessors(fluent = true)
@EqualsAndHashCode(onlyExplicitlyIncluded = true, callSuper = false)
public class ModOperation extends ModuleOperation {

  public static final short NB_ROWS_MOD = 1;

  @EqualsAndHashCode.Include @Getter private final OpCode opCode;
  @EqualsAndHashCode.Include @Getter private final Bytes32 arg1;
  @EqualsAndHashCode.Include @Getter private final Bytes32 arg2;

  public ModOperation(OpCode opCode, Bytes32 arg1, Bytes32 arg2) {
    this.arg1 = arg1;
    this.arg2 = arg2;
    this.opCode = opCode;
  }

  public void trace(Trace.Mod trace) {
    final UInt256 res;
    // Sanity check for division-by-zero
    if (this.arg2.isZero()) {
      res = UInt256.ZERO;
    } else {
      res =
          switch (opCode) {
            case DIV -> UInt256.fromBytes(arg1).divide(UInt256.fromBytes(arg2));
            case SDIV -> UInt256.fromBytes(arg1).sdiv0(UInt256.fromBytes(arg2));
            case MOD -> UInt256.fromBytes(arg1).mod(UInt256.fromBytes(arg2));
            case SMOD -> UInt256.fromBytes(arg1).smod0(UInt256.fromBytes(arg2));
            default ->
                throw new IllegalArgumentException("Modular arithmetic was given wrong opcode");
          };
    }
    // trace inputs / outputs
    trace.inst(opCode.byteValue()).arg1(arg1).arg2(arg2).res(res).validateRow();
  }

  @Override
  protected int computeLineCount() {
    return 1;
  }
}
