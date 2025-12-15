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

package net.consensys.linea.zktracer.module.add;

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
public class AddOperation extends ModuleOperation {
  public static final short NB_ROWS_ADD = 1;

  @EqualsAndHashCode.Include @Getter protected final OpCode opCode;
  @EqualsAndHashCode.Include @Getter protected final Bytes32 arg1;
  @EqualsAndHashCode.Include @Getter protected final Bytes32 arg2;

  public AddOperation(OpCode opCode, Bytes32 arg1, Bytes32 arg2) {
    this.opCode = opCode;
    this.arg1 = arg1;
    this.arg2 = arg2;
  }

  public void trace(Trace.Add trace) {
    // Compute result of this operation.
    final UInt256 res =
        switch (opCode) {
          case ADD -> UInt256.fromBytes(arg1).add(UInt256.fromBytes(arg2));
          case SUB -> UInt256.fromBytes(arg1).subtract(UInt256.fromBytes(arg2));
          default -> throw new IllegalArgumentException("invalid operation: " + opCode);
        };
    // Trace it
    trace.arg1(arg1).arg2(arg2).inst(opCode.byteValue()).res(res).validateRow();
  }

  @Override
  protected int computeLineCount() {
    return NB_ROWS_ADD;
  }

  public static class Comparator implements java.util.Comparator<AddOperation> {
    public int compare(AddOperation op1, AddOperation op2) {
      // First sort by OpCode
      final int opCodeComp = op1.opCode().compareTo(op2.opCode());
      if (opCodeComp != 0) {
        return opCodeComp;
      }
      // Second sort by Arg1
      final int arg1Comp = op1.arg1().compareTo(op2.arg1());
      if (arg1Comp != 0) {
        return arg1Comp;
      }
      // Third, sort by Arg2
      return op1.arg2().compareTo(op2.arg2());
    }
  }
}
