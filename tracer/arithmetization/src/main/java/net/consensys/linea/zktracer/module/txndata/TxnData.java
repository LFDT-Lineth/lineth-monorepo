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

import java.util.ArrayList;
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
import net.consensys.linea.zktracer.module.txndata.transactions.SysfNoopTransaction;
import net.consensys.linea.zktracer.module.txndata.transactions.SysiEip2935Transaction;
import net.consensys.linea.zktracer.module.txndata.transactions.SysiEip4788Transaction;
import net.consensys.linea.zktracer.module.txndata.transactions.UserTransaction;
import net.consensys.linea.zktracer.module.wcp.Wcp;
import net.consensys.linea.zktracer.types.TransactionProcessingMetadata;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.evm.worldstate.WorldView;
import org.hyperledger.besu.plugin.data.BlockBody;
import org.hyperledger.besu.plugin.data.BlockHeader;
import org.hyperledger.besu.plugin.data.ProcessableBlockHeader;

@RequiredArgsConstructor
@Accessors(fluent = true)
public final class TxnData implements OperationListModule<TxnDataOperation> {
  @Getter
  private final ModuleOperationStackedList<TxnDataOperation> operations =
      new ModuleOperationStackedList<>();

  @Getter private final Hub hub;
  @Getter private final Wcp wcp;
  @Getter private final Euc euc;

  @Getter private final List<BlockSnapshot> blocks = new ArrayList<>();
  @Getter private ProcessableBlockHeader currentBlockHeader;

  @Override
  public ModuleName moduleKey() {
    return TXN_DATA;
  }

  @Override
  public void traceStartBlock(
      WorldView world,
      final ProcessableBlockHeader processableBlockHeader,
      final Address miningBeneficiary) {
    blocks.add(new BlockSnapshot(processableBlockHeader));
    currentBlockHeader = processableBlockHeader;
  }

  @Override
  public void traceEndBlock(final BlockHeader blockHeader, final BlockBody blockBody) {
    blocks.getLast().setNbOfTxsInBlock(blockBody.getTransactions().size());
  }

  public void callTxnDataForSystemTransaction(final SystemTransactionType type) {
    switch (type) {
      case SYSI_EIP_4788_BEACON_BLOCK_ROOT -> operations().add(new SysiEip4788Transaction(this));
      case SYSI_EIP_2935_HISTORICAL_HASH -> operations().add(new SysiEip2935Transaction(this));
      case SYSF_NOOP -> operations().add(new SysfNoopTransaction(this));
      case SYSI_NOOP ->
          throw new IllegalArgumentException("Unsupported system transaction type: " + type);
    }
  }

  @Override
  public int spillage(Trace trace) {
    return trace.txndata().spillage();
  }

  @Override
  public List<Trace.ColumnHeader> columnHeaders(Trace trace) {
    return trace.txndata().headers(this.lineCount());
  }

  @Override
  public void commit(Trace trace) {
    final int totUserFinal = totalNumberOfUserTransactions();
    for (TxnDataOperation tx : operations().getAll()) {
      tx.traceTransaction(trace.txndata(), totUserFinal);
    }
  }

  @Override
  public void traceEndTx(TransactionProcessingMetadata tx) {
    operations().add(new UserTransaction(this, tx));
  }

  public int numberOfUserTransactionsInCurrentBlock() {
    return blocks.getLast().getNbOfTxsInBlock();
  }

  public int totalNumberOfUserTransactions() {
    return Math.toIntExact(
        operations().stream().filter(op -> op instanceof UserTransaction).count());
  }
}
