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

package lineth.zktracer.module.mod;

import static lineth.zktracer.opcode.OpCode.*;

import java.math.BigInteger;
import java.util.List;
import lineth.zktracer.Trace;
import lineth.zktracer.container.module.OperationSetWithAdditionalRowsModule;
import lineth.zktracer.container.stacked.CountOnlyOperation;
import lineth.zktracer.container.stacked.ModuleOperationStackedSet;
import lineth.zktracer.module.ModuleName;
import lineth.zktracer.opcode.OpCode;
import lombok.Getter;
import lombok.experimental.Accessors;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.evm.frame.MessageFrame;

@Getter
@Accessors(fluent = true)
public class Mod implements OperationSetWithAdditionalRowsModule<ModOperation> {
  private final ModuleOperationStackedSet<ModOperation> operations =
      new ModuleOperationStackedSet<>();
  private final CountOnlyOperation additionalRows = new CountOnlyOperation();

  @Override
  public ModuleName moduleKey() {
    return ModuleName.MOD;
  }

  public void callMod(MessageFrame frame, OpCode opcode) {
    final Bytes32 arg1 = Bytes32.leftPad(frame.getStackItem(0));
    final Bytes32 arg2 = Bytes32.leftPad(frame.getStackItem(1));
    operations.add(new ModOperation(opcode, arg1, arg2));
  }

  @Override
  public List<Trace.ColumnHeader> columnHeaders(Trace trace) {
    return trace.mod().headers(this.lineCount());
  }

  @Override
  public int spillage(Trace trace) {
    return trace.mod().spillage();
  }

  @Override
  public void commit(Trace trace) {
    for (ModOperation op : operations.sortOperations(new ModOperationComparator())) {
      op.trace(trace.mod());
    }
  }

  /**
   * Performs a context-free call to the DIV opcode in the current trace.
   *
   * @param arg1 the divider
   * @param arg2 the dividend
   */
  public BigInteger callDIV(Bytes32 arg1, Bytes32 arg2) {
    this.operations.add(new ModOperation(OpCode.DIV, arg1, arg2));
    return arg1.toUnsignedBigInteger().divide(arg2.toUnsignedBigInteger());
  }

  /**
   * Performs a context-free call to the MOD opcode in the current trace.
   *
   * @param arg1 the number
   * @param arg2 the module
   */
  public BigInteger callMOD(Bytes32 arg1, Bytes32 arg2) {
    this.operations.add(new ModOperation(OpCode.MOD, arg1, arg2));
    return arg1.toUnsignedBigInteger().mod(arg2.toUnsignedBigInteger());
  }
}
