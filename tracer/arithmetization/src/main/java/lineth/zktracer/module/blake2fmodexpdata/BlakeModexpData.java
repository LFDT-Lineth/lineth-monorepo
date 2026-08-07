/*
 * Copyright ConsenSys Inc.
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

package lineth.zktracer.module.blake2fmodexpdata;

import static com.google.common.base.Preconditions.checkState;
import static lineth.zktracer.module.ModuleName.BLAKE_MODEXP_DATA;

import java.util.List;
import lineth.zktracer.Trace;
import lineth.zktracer.container.module.IncrementAndDetectModule;
import lineth.zktracer.container.module.IncrementingModule;
import lineth.zktracer.container.module.OperationListModule;
import lineth.zktracer.container.stacked.ModuleOperationStackedList;
import lineth.zktracer.module.ModuleName;
import lineth.zktracer.module.limits.precompiles.BlakeRounds;
import lineth.zktracer.module.wcp.Wcp;
import lombok.Getter;
import lombok.RequiredArgsConstructor;
import lombok.experimental.Accessors;
import org.hyperledger.besu.evm.worldstate.WorldView;

@RequiredArgsConstructor
@Getter
@Accessors(fluent = true)
public class BlakeModexpData implements OperationListModule<BlakeModexpDataOperation> {
  private final Wcp wcp;
  private final IncrementAndDetectModule modexpEffectiveCall;
  private final IncrementingModule modexpLargeCall;
  private final IncrementingModule blakeEffectiveCall;
  private final BlakeRounds blakeRounds;

  private final ModuleOperationStackedList<BlakeModexpDataOperation> operations =
      new ModuleOperationStackedList<>();

  private long previousID = 0;

  @Override
  public ModuleName moduleKey() {
    return BLAKE_MODEXP_DATA;
  }

  public void callModexp(BlakeModexpDataOperation modexpOperation) {

    checkState(modexpOperation.isModexpOperation(), "Operation must be a MODEXP operation");
    operations.add(modexpOperation);

    modexpEffectiveCall.updateTally(1);
    modexpLargeCall.updateTally(modexpOperation.modexpMetaData.get().largeModexp());
    callWcpForIdCheck(modexpOperation.id());
  }

  public void callBlake(BlakeModexpDataOperation blakeOperation) {
    checkState(blakeOperation.isBlakeOperation(), "Operation must be a BLAKE2f operation");
    operations.add(blakeOperation);

    blakeEffectiveCall.updateTally(1);
    blakeRounds.addPrecompileLimit(blakeOperation.blake2fComponents.get().r());
    callWcpForIdCheck(blakeOperation.id());
  }

  private void callWcpForIdCheck(final long operationID) {
    wcp.callLT(previousID, operationID);
    previousID = operationID;
  }

  @Override
  public List<Trace.ColumnHeader> columnHeaders(Trace trace) {
    return trace.blake2fmodexpdata().headers(this.lineCount());
  }

  @Override
  public int spillage(Trace trace) {
    return trace.blake2fmodexpdata().spillage();
  }

  @Override
  public void traceEndConflation(final WorldView state) {
    operations().finishConflation();
  }

  @Override
  public void commit(Trace trace) {
    int stamp = 0;
    for (BlakeModexpDataOperation o : operations.getAll()) {
      o.trace(trace.blake2fmodexpdata(), ++stamp);
    }
  }
}
