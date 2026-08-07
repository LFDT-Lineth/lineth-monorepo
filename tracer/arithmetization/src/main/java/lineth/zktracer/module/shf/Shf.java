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

package lineth.zktracer.module.shf;

import static lineth.zktracer.module.ModuleName.SHF;
import static lineth.zktracer.opcode.OpCode.*;

import java.util.List;
import lineth.zktracer.Trace;
import lineth.zktracer.container.module.OperationSetModule;
import lineth.zktracer.container.stacked.ModuleOperationStackedSet;
import lineth.zktracer.module.ModuleName;
import lineth.zktracer.opcode.OpCode;
import lombok.Getter;
import lombok.experimental.Accessors;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.evm.frame.MessageFrame;

@Getter
@Accessors(fluent = true)
public class Shf implements OperationSetModule<ShfOperation> {

  private final ModuleOperationStackedSet<ShfOperation> operations =
      new ModuleOperationStackedSet<>();

  @Override
  public ModuleName moduleKey() {
    return SHF;
  }

  public void callShf(MessageFrame frame, OpCode opcode) {
    final Bytes32 arg1 = Bytes32.leftPad(frame.getStackItem(0));
    final Bytes32 arg2 = Bytes32.leftPad(frame.getStackItem(1));
    operations.add(new ShfOperation(opcode, arg1, arg2));
  }

  @Override
  public List<Trace.ColumnHeader> columnHeaders(Trace trace) {
    return trace.shf().headers(this.lineCount());
  }

  @Override
  public int spillage(Trace trace) {
    return trace.shf().spillage();
  }

  @Override
  public void commit(Trace trace) {
    for (ShfOperation op : operations.sortOperations(new ShfOperationComparator())) {
      op.trace(trace.shf());
    }
  }
}
