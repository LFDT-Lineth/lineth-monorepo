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

package net.consensys.linea.zktracer.module.trm;

import static net.consensys.linea.zktracer.module.ModuleName.TRM;

import java.util.List;
import lombok.Getter;
import lombok.RequiredArgsConstructor;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Fork;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.container.module.OperationSetModule;
import net.consensys.linea.zktracer.container.stacked.ModuleOperationStackedSet;
import net.consensys.linea.zktracer.module.ModuleName;
import net.consensys.linea.zktracer.types.EWord;
import net.consensys.linea.zktracer.types.TransactionProcessingMetadata;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.datatypes.CodeDelegation;

@Getter
@Accessors(fluent = true)
@RequiredArgsConstructor
public class Trm implements OperationSetModule<TrmOperation> {
  private final Fork fork;
  private final ModuleOperationStackedSet<TrmOperation> operations =
      new ModuleOperationStackedSet<>();

  @Override
  public ModuleName moduleKey() {
    return TRM;
  }

  @Override
  public void traceEndTx(TransactionProcessingMetadata tx) {
    // Note: this is useless as the range proof for the delegation address is done in RLP_AUTH, but
    // in order to have a single address compound constrain, the RLP_TXN does call TRM
    if (tx.requiresAuthorizationPhase()) {
      for (CodeDelegation delegation : tx.getBesuTransaction().getCodeDelegationList().get()) {
        callTrimming(delegation.address());
      }
    }
  }

  public void callTrimming(final Bytes32 rawAddress) {
    operations.add(new TrmOperation(fork, EWord.of(rawAddress)));
  }

  public void callTrimming(final Bytes rawAddress) {
    callTrimming(Bytes32.leftPad(rawAddress));
  }

  @Override
  public List<Trace.ColumnHeader> columnHeaders(Trace trace) {
    return trace.trm().headers(this.lineCount());
  }

  @Override
  public int spillage(Trace trace) {
    return trace.trm().spillage();
  }

  @Override
  public void commit(Trace trace) {
    for (TrmOperation operation : operations.sortOperations(new TrmOperationComparator())) {
      operation.trace(trace.trm());
    }
  }
}
