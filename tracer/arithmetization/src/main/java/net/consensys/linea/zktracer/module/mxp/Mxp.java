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

package net.consensys.linea.zktracer.module.mxp;

import static net.consensys.linea.zktracer.module.ModuleName.MXP;

import java.util.List;
import lombok.Getter;
import lombok.RequiredArgsConstructor;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.container.module.Module;
import net.consensys.linea.zktracer.container.module.OperationListModule;
import net.consensys.linea.zktracer.container.stacked.ModuleOperationStackedList;
import net.consensys.linea.zktracer.module.ModuleName;
import net.consensys.linea.zktracer.module.hub.fragment.imc.MxpCall;

/** Abstract class for implementation of a {@link Module} for memory expansion. */
@Getter
@Accessors(fluent = true)
@RequiredArgsConstructor
public final class Mxp implements OperationListModule<MxpOperation> {

  private final ModuleOperationStackedList<MxpOperation> operations =
      new ModuleOperationStackedList<>();

  @Override
  public List<Trace.ColumnHeader> columnHeaders(Trace trace) {
    return trace.mxp().headers(this.lineCount());
  }

  @Override
  public int spillage(Trace trace) {
    return trace.mxp().spillage();
  }

  @Override
  public void commit(Trace trace) {
    int stamp = 0;
    for (MxpOperation op : operations().getAll()) {
      op.trace(++stamp, trace.mxp());
    }
  }

  @Override
  public ModuleName moduleKey() {
    return MXP;
  }

  public void call(MxpCall mxpCall) {
    operations().add(new MxpOperation(mxpCall));
  }
}
