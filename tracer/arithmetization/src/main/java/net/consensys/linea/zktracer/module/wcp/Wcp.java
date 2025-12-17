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

package net.consensys.linea.zktracer.module.wcp;

import static net.consensys.linea.zktracer.module.ModuleName.WCP;
import static net.consensys.linea.zktracer.module.wcp.WcpOperation.EQbv;
import static net.consensys.linea.zktracer.module.wcp.WcpOperation.GEQbv;
import static net.consensys.linea.zktracer.module.wcp.WcpOperation.GTbv;
import static net.consensys.linea.zktracer.module.wcp.WcpOperation.ISZERObv;
import static net.consensys.linea.zktracer.module.wcp.WcpOperation.LEQbv;
import static net.consensys.linea.zktracer.module.wcp.WcpOperation.LTbv;
import static net.consensys.linea.zktracer.module.wcp.WcpOperation.SGTbv;
import static net.consensys.linea.zktracer.module.wcp.WcpOperation.SLTbv;
import static net.consensys.linea.zktracer.opcode.OpCode.*;

import java.util.List;
import lombok.RequiredArgsConstructor;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.container.module.Module;
import net.consensys.linea.zktracer.container.stacked.CountOnlyOperation;
import net.consensys.linea.zktracer.container.stacked.ModuleOperationStackedSet;
import net.consensys.linea.zktracer.module.ModuleName;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.evm.frame.MessageFrame;
import org.hyperledger.besu.evm.worldstate.WorldView;
import org.hyperledger.besu.plugin.data.ProcessableBlockHeader;

@RequiredArgsConstructor
public class Wcp implements Module {

  private final ModuleOperationStackedSet<WcpOperation> ltOperations =
      new ModuleOperationStackedSet<>();
  private final ModuleOperationStackedSet<WcpOperation> leqOperations =
      new ModuleOperationStackedSet<>();
  private final ModuleOperationStackedSet<WcpOperation> gtOperations =
      new ModuleOperationStackedSet<>();
  private final ModuleOperationStackedSet<WcpOperation> geqOperations =
      new ModuleOperationStackedSet<>();
  private final ModuleOperationStackedSet<WcpOperation> sltOperations =
      new ModuleOperationStackedSet<>();
  private final ModuleOperationStackedSet<WcpOperation> sgtOperations =
      new ModuleOperationStackedSet<>();
  private final ModuleOperationStackedSet<WcpOperation> eqOperations =
      new ModuleOperationStackedSet<>();
  private final ModuleOperationStackedSet<WcpOperation> isZeroOperations =
      new ModuleOperationStackedSet<>();

  /**
   * For perf, we split the WcpOperations into different StackedSet in order to - have smaller
   * set,thus faster to check for equality - remove the opcode value when checking the equality
   */
  private final List<ModuleOperationStackedSet<WcpOperation>> operations =
      List.of(
          ltOperations,
          leqOperations,
          gtOperations,
          geqOperations,
          sltOperations,
          sgtOperations,
          eqOperations,
          isZeroOperations);

  /** count the number of rows that could be added after the sequencer counts the number of line */
  public final CountOnlyOperation additionalRows = new CountOnlyOperation();

  @Override
  public ModuleName moduleKey() {
    return WCP;
  }

  @Override
  public void commitTransactionBundle() {
    for (ModuleOperationStackedSet<WcpOperation> operationsSet : operations) {
      operationsSet.commitTransactionBundle();
    }
    additionalRows.commitTransactionBundle();
  }

  @Override
  public void popTransactionBundle() {
    for (ModuleOperationStackedSet<WcpOperation> operationsSet : operations) {
      operationsSet.popTransactionBundle();
    }
    additionalRows.popTransactionBundle();
  }

  @Override
  public void traceStartBlock(
      WorldView world,
      final ProcessableBlockHeader processableBlockHeader,
      final Address miningBeneficiary) {
    additionalRows.commitTransactionBundle();
  }

  public void callWcp(MessageFrame frame, OpCode opcode) {
    final Bytes32 arg1 = Bytes32.leftPad(frame.getStackItem(0));
    final Bytes32 arg2 =
        (opcode != OpCode.ISZERO) ? Bytes32.leftPad(frame.getStackItem(1)) : Bytes32.ZERO;

    switch (opcode) {
      case LT -> ltOperations.add(new WcpOperation(LTbv, arg1, arg2));
      case GT -> gtOperations.add(new WcpOperation(GTbv, arg1, arg2));
      case SLT -> sltOperations.add(new WcpOperation(SLTbv, arg1, arg2));
      case SGT -> sgtOperations.add(new WcpOperation(SGTbv, arg1, arg2));
      case EQ -> eqOperations.add(new WcpOperation(EQbv, arg1, arg2));
      case ISZERO -> isZeroOperations.add(new WcpOperation(ISZERObv, arg1, Bytes32.ZERO));
      default -> throw new UnsupportedOperationException("Not given a WCP EVM Opcode");
    }
  }

  @Override
  public List<Trace.ColumnHeader> columnHeaders(Trace trace) {
    return trace.wcp().headers(this.lineCount());
  }

  @Override
  public void commit(Trace trace) {
    final WcpOperationComparator comparator = new WcpOperationComparator();
    for (ModuleOperationStackedSet<WcpOperation> operationsSet : operations) {
      for (WcpOperation operation : operationsSet.sortOperations(comparator)) {
        operation.trace(trace.wcp());
      }
    }
  }

  @Override
  public int lineCount() {
    final int count = operations.stream().mapToInt(ModuleOperationStackedSet::lineCount).sum();
    return ltOperations.conflationFinished() ? count : count + additionalRows.lineCount();
  }

  @Override
  public int spillage(Trace trace) {
    return trace.wcp().spillage();
  }

  public boolean callLT(final Bytes32 arg1, final Bytes32 arg2) {
    ltOperations.add(new WcpOperation(LTbv, arg1, arg2));
    return arg1.compareTo(arg2) < 0;
  }

  public boolean callLT(final Bytes arg1, final Bytes arg2) {
    return callLT(Bytes32.leftPad(arg1), Bytes32.leftPad(arg2));
  }

  public boolean callLT(final long arg1, final long arg2) {
    return callLT(Bytes.ofUnsignedLong(arg1), Bytes.ofUnsignedLong(arg2));
  }

  public boolean callGT(final Bytes32 arg1, final Bytes32 arg2) {
    gtOperations.add(new WcpOperation(GTbv, arg1, arg2));
    return arg1.compareTo(arg2) > 0;
  }

  public boolean callGT(final Bytes arg1, final Bytes arg2) {
    return callGT(Bytes32.leftPad(arg1), Bytes32.leftPad(arg2));
  }

  public boolean callGT(final int arg1, final int arg2) {
    return callGT(Bytes.ofUnsignedLong(arg1), Bytes.ofUnsignedLong(arg2));
  }

  public boolean callEQ(final Bytes32 arg1, final Bytes32 arg2) {
    eqOperations.add(new WcpOperation(EQbv, arg1, arg2));
    return arg1.compareTo(arg2) == 0;
  }

  public boolean callEQ(final Bytes arg1, final Bytes arg2) {
    return callEQ(Bytes32.leftPad(arg1), Bytes32.leftPad(arg2));
  }

  public boolean callISZERO(final Bytes32 arg1) {
    isZeroOperations.add(new WcpOperation(ISZERObv, arg1, Bytes32.ZERO));
    return arg1.isZero();
  }

  public boolean callISZERO(final Bytes arg1) {
    return callISZERO(Bytes32.leftPad(arg1));
  }

  public boolean callLEQ(final Bytes32 arg1, final Bytes32 arg2) {
    leqOperations.add(new WcpOperation(LEQbv, arg1, arg2));
    return arg1.compareTo(arg2) <= 0;
  }

  public boolean callLEQ(final long arg1, final long arg2) {
    return callLEQ(Bytes.ofUnsignedLong(arg1), Bytes.ofUnsignedLong(arg2));
  }

  public boolean callLEQ(final Bytes arg1, final Bytes arg2) {
    return callLEQ(Bytes32.leftPad(arg1), Bytes32.leftPad(arg2));
  }

  public boolean callGEQ(final Bytes32 arg1, final Bytes32 arg2) {
    geqOperations.add(new WcpOperation(GEQbv, arg1, arg2));
    return arg1.compareTo(arg2) >= 0;
  }

  public boolean callGEQ(final Bytes arg1, final Bytes arg2) {
    return callGEQ(Bytes32.leftPad(arg1), Bytes32.leftPad(arg2));
  }

  @Override
  public void traceEndConflation(final WorldView state) {
    for (ModuleOperationStackedSet<WcpOperation> operationsSet : operations) {
      operationsSet.finishConflation();
    }
  }
}
