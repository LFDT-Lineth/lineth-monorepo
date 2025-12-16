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

package net.consensys.linea.zktracer.module.bin;

import static net.consensys.linea.zktracer.module.ModuleName.BIN;
import static net.consensys.linea.zktracer.opcode.OpCode.*;

import java.util.List;
import lombok.Getter;
import lombok.RequiredArgsConstructor;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.bytestheta.BaseBytes;
import net.consensys.linea.zktracer.container.module.Module;
import net.consensys.linea.zktracer.container.module.OperationSetModule;
import net.consensys.linea.zktracer.container.stacked.ModuleOperationStackedSet;
import net.consensys.linea.zktracer.module.ModuleName;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.evm.frame.MessageFrame;

/** Implementation of a {@link Module} for binary operations. */
@Getter
@Accessors(fluent = true)
@RequiredArgsConstructor
public class Bin implements OperationSetModule<BinOperation> {

  private final ModuleOperationStackedSet<BinOperation> operations =
      new ModuleOperationStackedSet<>();

  @Override
  public ModuleName moduleKey() {
    return BIN;
  }

  public void callBin(MessageFrame frame, OpCode opcode) {
    final Bytes32 arg1 = Bytes32.leftPad(frame.getStackItem(0));
    final Bytes32 arg2 =
        isOneStackArgument(opcode) ? Bytes32.ZERO : Bytes32.leftPad(frame.getStackItem(1));

    operations.add(
        new BinOperation(opcode, BaseBytes.fromBytes32(arg1), BaseBytes.fromBytes32(arg2)));
  }

  @Override
  public List<Trace.ColumnHeader> columnHeaders(Trace trace) {
    return trace.bin().headers(this.lineCount());
  }

  @Override
  public int spillage(Trace trace) {
    return trace.bin().spillage();
  }

  @Override
  public void commit(Trace trace) {
    for (BinOperation op : operations.sortOperations(new BinOperationComparator())) {
      op.traceBinOperation(trace.bin());
    }
  }

  private static boolean isOneStackArgument(OpCode opcode) {
    return opcode == NOT || opcode == CLZ;
  }
}
