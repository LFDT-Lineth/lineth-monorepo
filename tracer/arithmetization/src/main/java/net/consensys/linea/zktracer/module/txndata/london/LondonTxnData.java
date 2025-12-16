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

package net.consensys.linea.zktracer.module.txndata.london;

import static com.google.common.base.Preconditions.checkState;
import static net.consensys.linea.zktracer.TraceLondon.Txndata.NB_ROWS_TYPE_0;
import static net.consensys.linea.zktracer.TraceLondon.Txndata.NB_ROWS_TYPE_1;
import static net.consensys.linea.zktracer.TraceLondon.Txndata.NB_ROWS_TYPE_2;

import java.util.ArrayList;
import java.util.List;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.euc.Euc;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.fragment.transaction.system.*;
import net.consensys.linea.zktracer.module.txndata.BlockSnapshot;
import net.consensys.linea.zktracer.module.txndata.TxnData;
import net.consensys.linea.zktracer.module.txndata.TxnDataOperation;
import net.consensys.linea.zktracer.module.wcp.Wcp;
import net.consensys.linea.zktracer.types.TransactionProcessingMetadata;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.evm.worldstate.WorldView;
import org.hyperledger.besu.plugin.data.BlockBody;
import org.hyperledger.besu.plugin.data.BlockHeader;
import org.hyperledger.besu.plugin.data.ProcessableBlockHeader;

public class LondonTxnData extends TxnData {

  private static final int NB_WCP_EUC_ROWS_FRONTIER_ACCESS_LIST_LONDON = 7;

  private final List<BlockSnapshot> blocks = new ArrayList<>();

  public LondonTxnData(Hub hub, Wcp wcp, Euc euc) {
    super(hub, wcp, euc);
  }

  @Override
  public final void traceStartBlock(
      WorldView world, final ProcessableBlockHeader blockHeader, final Address miningBeneficiary) {
    blocks.add(new BlockSnapshot(blockHeader));
  }

  @Override
  public void traceEndBlock(final BlockHeader blockHeader, final BlockBody blockBody) {
    checkState(
        currentTx() instanceof LondonTxnDataOperation,
        "non London transaction in LondonTxnData module");
    currentBlock()
        .setNbOfTxsInBlock(
            ((LondonTxnDataOperation) currentTx()).tx.getRelativeTransactionNumber());
    ((LondonTxnDataOperation) currentTx())
        .setCallWcpLastTxOfBlock(currentBlock().getBlockGasLimit());
  }

  @Override
  public void traceEndTx(TransactionProcessingMetadata tx) {
    operations()
        .add(
            new LondonTxnDataOperation(
                wcp(),
                euc(),
                tx,
                NB_ROWS_TYPE_0,
                NB_ROWS_TYPE_1,
                NB_ROWS_TYPE_2,
                NB_WCP_EUC_ROWS_FRONTIER_ACCESS_LIST_LONDON));
  }

  public BlockSnapshot currentBlock() {
    return blocks.getLast();
  }

  private TxnDataOperation currentTx() {
    return operations().getLast();
  }

  @Override
  public int numberOfUserTransactionsInCurrentBlock() {
    return currentBlock().getNbOfTxsInBlock();
  }

  @Override
  public void traceStartConflation(final long blockCount) {
    wcp().additionalRows.add(4); /* 4 = byte length of LINEA_BLOCK_GAS_LIMIT */
  }

  @Override
  public void callTxnDataForSystemTransaction(SystemTransactionType type) {
    throw new IllegalStateException("System transactions appear in Cancun.");
  }

  @Override
  public int lineCount() {
    // The last tx of each block has one more rows
    return operations().lineCount() + blocks.size();
  }

  @Override
  public void commit(Trace trace) {
    final int absTxNumMax = operations().size();

    for (TxnDataOperation tx : operations().getAll()) {
      tx.traceTransaction(
          trace.txndata(),
          blocks.get(((LondonTxnDataOperation) tx).getTx().getRelativeBlockNumber() - 1),
          absTxNumMax);
    }
  }
}
