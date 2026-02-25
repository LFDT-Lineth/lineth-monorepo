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

package net.consensys.linea.zktracer.module.rlpAuth;

import static net.consensys.linea.zktracer.module.ModuleName.RLP_AUTH;

import java.util.List;
import lombok.Getter;
import lombok.RequiredArgsConstructor;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.container.module.OperationListModule;
import net.consensys.linea.zktracer.container.stacked.ModuleOperationStackedList;
import net.consensys.linea.zktracer.module.ModuleName;
import net.consensys.linea.zktracer.module.ecdata.EcData;
import net.consensys.linea.zktracer.module.hub.fragment.AuthorizationFragment;
import net.consensys.linea.zktracer.module.shakiradata.ShakiraData;

@RequiredArgsConstructor
@Accessors(fluent = true)
@Getter
public final class RlpAuth implements OperationListModule<RlpAuthOperation> {

  final ShakiraData shakiraData;
  final EcData ecData;

  private final ModuleOperationStackedList<RlpAuthOperation> operations =
      new ModuleOperationStackedList<>();

  @Override
  public ModuleName moduleKey() {
    return RLP_AUTH;
  }

  public void callRlpAuth(AuthorizationFragment authorizationFragment) {
    RlpAuthOperation op = new RlpAuthOperation(authorizationFragment, ecData, shakiraData);
    operations.add(op);
  }

  @Override
  public List<Trace.ColumnHeader> columnHeaders(Trace trace) {
    return trace.rlpauth().headers(this.lineCount());
  }

  @Override
  public int spillage(Trace trace) {
    return trace.rlpauth().spillage();
  }

  @Override
  public void commit(Trace trace) {
    for (RlpAuthOperation op : operations.getAll()) {
      op.trace(trace.rlpauth());
    }
  }
}
