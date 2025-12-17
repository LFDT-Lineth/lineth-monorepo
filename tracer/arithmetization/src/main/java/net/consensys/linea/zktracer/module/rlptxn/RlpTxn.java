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

package net.consensys.linea.zktracer.module.rlptxn;

import static net.consensys.linea.zktracer.module.ModuleName.RLP_TXN;

import java.util.List;
import lombok.Getter;
import lombok.RequiredArgsConstructor;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.container.module.OperationListModule;
import net.consensys.linea.zktracer.container.stacked.ModuleOperationStackedList;
import net.consensys.linea.zktracer.module.ModuleName;
import net.consensys.linea.zktracer.module.rlpUtils.RlpUtils;
import net.consensys.linea.zktracer.module.trm.Trm;
import net.consensys.linea.zktracer.types.TransactionProcessingMetadata;

@RequiredArgsConstructor
@Accessors(fluent = true)
public final class RlpTxn implements OperationListModule<RlpTxnOperation> {
  private final RlpUtils rlpUtils;
  private final Trm trm;

  @Getter
  private final ModuleOperationStackedList<RlpTxnOperation> operations =
      new ModuleOperationStackedList<>();

  @Override
  public ModuleName moduleKey() {
    return RLP_TXN;
  }

  @Override
  public void traceEndTx(TransactionProcessingMetadata tx) {
    operations.add(new RlpTxnOperation(rlpUtils, trm, tx));
  }

  @Override
  public List<Trace.ColumnHeader> columnHeaders(Trace trace) {
    return trace.rlptxn().headers(this.lineCount());
  }

  @Override
  public int spillage(Trace trace) {
    return trace.rlptxn().spillage();
  }

  @Override
  public void commit(Trace trace) {
    for (RlpTxnOperation op : operations.getAll()) {
      op.trace(trace.rlptxn(), operations.size());
    }
  }
}
