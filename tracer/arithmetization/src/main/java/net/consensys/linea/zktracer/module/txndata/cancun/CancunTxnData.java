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
package net.consensys.linea.zktracer.module.txndata.cancun;

import java.util.ArrayList;
import java.util.List;

import lombok.Getter;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.euc.Euc;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.fragment.transaction.system.SystemTransactionType;
import net.consensys.linea.zktracer.module.txndata.BlockSnapshot;
import net.consensys.linea.zktracer.module.txndata.TxnData;
import net.consensys.linea.zktracer.module.txndata.TxnDataOperation;
import net.consensys.linea.zktracer.module.txndata.cancun.transactions.CancunUserTransaction;
import net.consensys.linea.zktracer.module.txndata.cancun.transactions.SysfNoopTransaction;
import net.consensys.linea.zktracer.module.txndata.cancun.transactions.SysiEip2935Transaction;
import net.consensys.linea.zktracer.module.txndata.cancun.transactions.SysiEip4788Transaction;
import net.consensys.linea.zktracer.module.wcp.Wcp;
import net.consensys.linea.zktracer.types.TransactionProcessingMetadata;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.evm.worldstate.WorldView;
import org.hyperledger.besu.plugin.data.BlockBody;
import org.hyperledger.besu.plugin.data.BlockHeader;
import org.hyperledger.besu.plugin.data.ProcessableBlockHeader;

public class CancunTxnData extends TxnData {

  @Getter private final List<BlockSnapshot> blocks = new ArrayList<>();
  @Getter private ProcessableBlockHeader currentBlockHeader;

  public CancunTxnData(Hub hub, Wcp wcp, Euc euc) {
    super(hub, wcp, euc);
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

  @Override
  public void traceEndTx(TransactionProcessingMetadata tx) {
    operations().add(new CancunUserTransaction(this, tx));
  }

  @Override
  public int numberOfUserTransactionsInCurrentBlock() {
    return blocks.getLast().getNbOfTxsInBlock();
  }

  public void callTxnDataForSystemTransaction(final SystemTransactionType type) {
    switch (type) {
      case SYSI_EIP_4788_BEACON_BLOCK_ROOT -> operations().add(new SysiEip4788Transaction(this));
      case SYSI_EIP_2935_HISTORICAL_HASH -> operations().add(new SysiEip2935Transaction(this));
      case SYSF_NOOP -> operations().add(new SysfNoopTransaction(this));
      case SYSI_NOOP -> throw new IllegalArgumentException(
          "Unsupported system transaction type: " + type);
    }
  }

  public long totalNumberOfUserTransactions() {
    return operations().stream().filter(op -> op instanceof CancunUserTransaction).count();
  }

  @Override
  public void commit(Trace trace) {
    for (TxnDataOperation tx : operations().getAll()) {
      CancunTxnDataOperation cancunTx = (CancunTxnDataOperation) tx;
      cancunTx.traceTransaction(trace.txndata(), totalNumberOfUserTransactions());
    }
  }
}
