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

package lineth.zktracer.module.ext;

import static lineth.zktracer.module.ModuleName.EXT;
import static lineth.zktracer.opcode.OpCode.*;

import java.util.List;
import lineth.zktracer.Trace;
import lineth.zktracer.container.module.OperationSetModule;
import lineth.zktracer.container.stacked.ModuleOperationAdder;
import lineth.zktracer.container.stacked.ModuleOperationStackedSet;
import lineth.zktracer.module.ModuleName;
import lineth.zktracer.opcode.OpCode;
import lombok.Getter;
import lombok.RequiredArgsConstructor;
import lombok.experimental.Accessors;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.evm.frame.MessageFrame;

@RequiredArgsConstructor
@Getter
@Accessors(fluent = true)
public class Ext implements OperationSetModule<ExtOperation> {

  private final ModuleOperationStackedSet<ExtOperation> operations =
      new ModuleOperationStackedSet<>();

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
    return ((ExtOperation) addedOp.op()).result();
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
    for (ExtOperation operation : operations.sortOperations(new ExtOperationComparator())) {
      operation.trace(trace.ext());
    }
  }
}
