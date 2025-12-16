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

package net.consensys.linea.zktracer.module.stp;

import static com.google.common.base.Preconditions.*;
import static net.consensys.linea.zktracer.module.ModuleName.STP;

import java.util.List;
import lombok.Getter;
import lombok.RequiredArgsConstructor;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.container.module.OperationSetModule;
import net.consensys.linea.zktracer.container.stacked.ModuleOperationStackedSet;
import net.consensys.linea.zktracer.module.ModuleName;
import net.consensys.linea.zktracer.module.hub.fragment.imc.StpCall;

@RequiredArgsConstructor
@Accessors(fluent = true)
public class Stp implements OperationSetModule<StpOperation> {

  @Getter
  private final ModuleOperationStackedSet<StpOperation> operations =
      new ModuleOperationStackedSet<>();

  public void call(StpCall stpCall) {
    final StpOperation stpOperation = new StpOperation(stpCall);
    operations.add(stpOperation);

    checkArgument(
        stpCall.opCodeData().isCall() || stpCall.opCodeData().isCreate(),
        "STP handles only Calls and CREATEs");
  }

  @Override
  public ModuleName moduleKey() {
    return STP;
  }

  @Override
  public List<Trace.ColumnHeader> columnHeaders(Trace trace) {
    return trace.stp().headers(this.lineCount());
  }

  @Override
  public int spillage(Trace trace) {
    return trace.stp().spillage();
  }

  @Override
  public void commit(Trace trace) {
    for (StpOperation operation : operations.sortOperations(new StpOperationComparator())) {
      operation.trace(trace.stp());
    }
  }
}
