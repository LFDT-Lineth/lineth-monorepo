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

package net.consensys.linea.zktracer.module.txndata;

import static net.consensys.linea.zktracer.module.ModuleName.TXN_DATA;

import java.util.List;
import lombok.Getter;
import lombok.RequiredArgsConstructor;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.container.module.OperationListModule;
import net.consensys.linea.zktracer.container.stacked.ModuleOperationStackedList;
import net.consensys.linea.zktracer.module.ModuleName;
import net.consensys.linea.zktracer.module.euc.Euc;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.fragment.transaction.system.SystemTransactionType;
import net.consensys.linea.zktracer.module.wcp.Wcp;
import net.consensys.linea.zktracer.types.TransactionProcessingMetadata;

@RequiredArgsConstructor
@Accessors(fluent = true)
public abstract class TxnData implements OperationListModule<TxnDataOperation> {
  @Getter
  private final ModuleOperationStackedList<TxnDataOperation> operations =
      new ModuleOperationStackedList<>();

  @Getter private final Hub hub;
  @Getter private final Wcp wcp;
  @Getter private final Euc euc;

  @Override
  public ModuleName moduleKey() {
    return TXN_DATA;
  }

  @Override
  public abstract void traceEndTx(TransactionProcessingMetadata tx);

  @Override
  public int spillage(Trace trace) {
    return trace.txndata().spillage();
  }

  @Override
  public List<Trace.ColumnHeader> columnHeaders(Trace trace) {
    return trace.txndata().headers(this.lineCount());
  }

  public abstract int numberOfUserTransactionsInCurrentBlock();

  public abstract void callTxnDataForSystemTransaction(final SystemTransactionType type);
}
