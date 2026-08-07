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

package lineth.zktracer.module.exp;

import static lineth.zktracer.module.ModuleName.EXP;

import java.util.List;
import lombok.Getter;
import lombok.RequiredArgsConstructor;
import lombok.experimental.Accessors;
import lombok.extern.slf4j.Slf4j;
import lineth.zktracer.Trace;
import lineth.zktracer.container.module.OperationSetModule;
import lineth.zktracer.container.stacked.ModuleOperationStackedSet;
import lineth.zktracer.module.ModuleName;
import lineth.zktracer.module.hub.fragment.imc.exp.ExpCall;

@Slf4j
@RequiredArgsConstructor
@Accessors(fluent = true)
public class Exp implements OperationSetModule<ExpOperation> {

  @Getter
  private final ModuleOperationStackedSet<ExpOperation> operations =
      new ModuleOperationStackedSet<>();

  @Override
  public ModuleName moduleKey() {
    return EXP;
  }

  public void call(ExpCall expCall) {
    operations.add(new ExpOperation(expCall));
  }

  @Override
  public List<Trace.ColumnHeader> columnHeaders(Trace trace) {
    return trace.exp().headers(this.lineCount());
  }

  @Override
  public int spillage(Trace trace) {
    return trace.exp().spillage();
  }

  @Override
  public void commit(Trace trace) {
    for (ExpOperation expOp : operations.sortOperations(new ExpOperationComparator())) {
      expOp.trace(trace.exp());
    }
  }
}
