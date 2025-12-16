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

package net.consensys.linea.zktracer.module.ext;

import static net.consensys.linea.zktracer.module.ModuleName.EXT;
import static net.consensys.linea.zktracer.opcode.OpCode.*;

import java.util.List;
import lombok.Getter;
import lombok.RequiredArgsConstructor;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.container.module.OperationSetWithAdditionalRowsModule;
import net.consensys.linea.zktracer.container.stacked.CountOnlyOperation;
import net.consensys.linea.zktracer.container.stacked.ModuleOperationAdder;
import net.consensys.linea.zktracer.container.stacked.ModuleOperationStackedSet;
import net.consensys.linea.zktracer.module.ModuleName;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.evm.frame.MessageFrame;

@RequiredArgsConstructor
@Getter
@Accessors(fluent = true)
public class Ext implements OperationSetWithAdditionalRowsModule<ExtOperation> {

  private final ModuleOperationStackedSet<ExtOperation> operations =
      new ModuleOperationStackedSet<>();
  private final CountOnlyOperation additionalRows = new CountOnlyOperation();

  @Override
  public ModuleName moduleKey() {
    return EXT;
  }

  public void callExt(MessageFrame frame, OpCode opcode) {
    call(opcode, frame.getStackItem(0), frame.getStackItem(1), frame.getStackItem(2));
  }

  public Bytes call(OpCode opCode, Bytes _arg1, Bytes _arg2, Bytes _arg3) {
    final Bytes32 arg1 = Bytes32.leftPad(_arg1);
    final Bytes32 arg2 = Bytes32.leftPad(_arg2);
    final Bytes32 arg3 = Bytes32.leftPad(_arg3);
    final ExtOperation op = new ExtOperation(opCode, arg1, arg2, arg3);
    final ModuleOperationAdder addedOp = operations.addAndGet(op);
    if (addedOp.isNew()) {
      ((ExtOperation) addedOp.op()).computeResult();
    }
    return ((ExtOperation) addedOp.op()).resultUInt256();
  }

  @Override
  public List<Trace.ColumnHeader> columnHeaders(Trace trace) {
    return trace.ext().headers(this.lineCount());
  }

  @Override
  public int spillage(Trace trace) {
    return trace.ext().spillage();
  }

  @Override
  public void commit(Trace trace) {
    OperationSetWithAdditionalRowsModule.super.commit(trace);
    int stamp = 0;
    for (ExtOperation operation : operations.sortOperations(new ExtOperationComparator())) {
      operation.trace(trace.ext(), ++stamp);
    }
  }
}
