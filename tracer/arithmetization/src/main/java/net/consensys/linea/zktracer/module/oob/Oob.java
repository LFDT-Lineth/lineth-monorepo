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

package net.consensys.linea.zktracer.module.oob;

import static net.consensys.linea.zktracer.module.ModuleName.OOB;

import java.util.List;
import lombok.Getter;
import lombok.RequiredArgsConstructor;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.container.module.OperationSetModule;
import net.consensys.linea.zktracer.container.stacked.ModuleOperationAdder;
import net.consensys.linea.zktracer.container.stacked.ModuleOperationStackedSet;
import net.consensys.linea.zktracer.module.ModuleName;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.OobCall;

/** Implementation of a {@link Module} for out of bounds. */
@RequiredArgsConstructor
@Accessors(fluent = true)
public class Oob implements OperationSetModule<OobCall> {

  @Getter
  private final ModuleOperationStackedSet<OobCall> operations = new ModuleOperationStackedSet<>();

  @Override
  public ModuleName moduleKey() {
    return OOB;
  }

  public OobCall call(OobCall oobCall, Hub hub) {
    oobCall.setInputs(hub, hub.messageFrame());
    final ModuleOperationAdder addedOperation = operations.addAndGet(oobCall);
    final OobCall op = (OobCall) addedOperation.op();
    if (addedOperation.isNew()) {
      op.setOutputs();
    }
    return op;
  }

  @Override
  public List<Trace.ColumnHeader> columnHeaders(Trace trace) {
    return trace.oob().headers(this.lineCount());
  }

  @Override
  public int spillage(Trace trace) {
    return trace.oob().spillage();
  }

  @Override
  public void commit(Trace trace) {
    for (OobCall op : operations.getAll()) {
      op.traceOob(trace.oob());
    }
  }
}
