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

package lineth.zktracer.module.rlpAuth;

import static lineth.zktracer.module.ModuleName.RLP_AUTH;

import java.util.List;
import lineth.zktracer.Trace;
import lineth.zktracer.container.module.OperationListModule;
import lineth.zktracer.container.stacked.ModuleOperationStackedList;
import lineth.zktracer.module.ModuleName;
import lineth.zktracer.module.ecdata.EcData;
import lineth.zktracer.module.hub.fragment.AuthorizationFragment;
import lineth.zktracer.module.shakiradata.ShakiraData;
import lombok.Getter;
import lombok.RequiredArgsConstructor;
import lombok.experimental.Accessors;

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
