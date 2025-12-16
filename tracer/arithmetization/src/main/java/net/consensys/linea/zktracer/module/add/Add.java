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

import static net.consensys.linea.zktracer.opcode.OpCode.ADD;
import static net.consensys.linea.zktracer.opcode.OpCode.SUB;

import java.math.BigInteger;
import java.util.List;
import lombok.Getter;
import lombok.RequiredArgsConstructor;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.container.module.Module;
import net.consensys.linea.zktracer.container.module.OperationSetWithAdditionalRowsModule;
import net.consensys.linea.zktracer.container.stacked.CountOnlyOperation;
import net.consensys.linea.zktracer.container.stacked.ModuleOperationStackedSet;
import net.consensys.linea.zktracer.module.ModuleName;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.evm.frame.MessageFrame;

/** Implementation of a {@link Module} for addition/subtraction. */
@RequiredArgsConstructor
@Getter
@Accessors(fluent = true)
public class Add implements OperationSetWithAdditionalRowsModule<AddOperation> {

  protected final ModuleOperationStackedSet<AddOperation> operations =
      new ModuleOperationStackedSet<>();

  private final CountOnlyOperation additionalRows = new CountOnlyOperation();

  @Override
  public ModuleName moduleKey() {
    return ModuleName.ADD;
  }

  public void callAdd(MessageFrame frame, OpCode opcode) {
    call(opcode, Bytes32.leftPad(frame.getStackItem(0)), Bytes32.leftPad(frame.getStackItem(1)));
  }

  @Override
  public List<Trace.ColumnHeader> columnHeaders(Trace trace) {
    return trace.add().headers(this.lineCount());
  }

  @Override
  public int spillage(Trace trace) {
    return trace.add().spillage();
  }

  @Override
  public void commit(Trace trace) {
    OperationSetWithAdditionalRowsModule.super.commit(trace);
    for (AddOperation op : sortOperations(new AddOperation.Comparator())) {
      op.trace(trace.add());
    }
  }

  /**
   * Register a call into the Add module for a given opcode and argument pairing. This returns the
   * expected result.
   *
   * @param opcode Opcode for operation, which must be either ADD or SUB.
   * @param arg1 First argument.
   * @param arg2 Second argument.
   * @return Result of the addition / subtraction.
   */
  public BigInteger call(OpCode opcode, Bytes32 arg1, Bytes32 arg2) {
    if (opcode != ADD && opcode != SUB) {
      throw new IllegalArgumentException("invalid opcode for Add: " + opcode);
    }
    // Register the operation
    this.addOperation(opcode, arg1, arg2);
    // Compute the expected result
    if (opcode == ADD) {
      return arg1.toUnsignedBigInteger().add(arg2.toUnsignedBigInteger());
    } else {
      return arg1.toUnsignedBigInteger().subtract(arg2.toUnsignedBigInteger());
    }
  }

  protected void addOperation(OpCode opcode, Bytes32 arg1, Bytes32 arg2) {
    operations.add(new AddOperation(opcode, arg1, arg2));
  }
}
