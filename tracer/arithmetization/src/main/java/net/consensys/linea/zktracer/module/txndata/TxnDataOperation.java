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

import static net.consensys.linea.zktracer.module.hub.TransactionProcessingType.*;

import java.util.ArrayList;
import java.util.List;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.container.ModuleOperation;
import net.consensys.linea.zktracer.module.euc.Euc;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.TransactionProcessingType;
import net.consensys.linea.zktracer.module.txndata.rows.TxnDataRow;
import net.consensys.linea.zktracer.module.txndata.transactions.UserTransaction;
import net.consensys.linea.zktracer.module.wcp.Wcp;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.plugin.data.ProcessableBlockHeader;

public abstract class TxnDataOperation extends ModuleOperation {
  public final ProcessableBlockHeader blockHeader;
  public final Hub hub;
  public final Euc euc;
  public final Wcp wcp;
  public final short relativeBlockNumber;
  public final short sysiTransactionNumber;
  public final short userTransactionNumber;
  public final short sysfTransactionNumber;
  public final List<TxnDataRow> rows = new ArrayList<>();
  public final TransactionProcessingType category;
  public final BlockSnapshot blockSnapshot;

  protected abstract int ctMax();

  @Override
  public int computeLineCount() {
    return rows.size();
  }

  public TxnDataOperation(TxnData txnData, TransactionProcessingType category) {
    blockHeader = txnData.currentBlockHeader();
    hub = txnData.hub();
    wcp = hub.wcp();
    euc = hub.euc();
    relativeBlockNumber = (short) hub.blockStack().currentRelativeBlockNumber();
    sysiTransactionNumber = hub.state.sysiTransactionNumber();
    userTransactionNumber = hub.state.getUserTransactionNumber();
    sysfTransactionNumber = hub.state.sysfTransactionNumber();
    this.category = category;
    blockSnapshot = txnData.blocks().getLast();
  }

  public void traceTransaction(Trace.Txndata trace, long totalUserTransactionsInConflation) {
    short ct = 0;
    for (TxnDataRow row : rows) {
      traceCommonSaveForFlags(trace, ct, totalUserTransactionsInConflation);
      row.traceRow(trace);
      trace.fillAndValidateRow();
      ct++;
    }
  }

  private void traceCommonSaveForFlags(
      Trace.Txndata trace, int ct, long totalUserTransactionsInConflation) {
    final long relativeUserTxNumMax = blockSnapshot.getNbOfTxsInBlock();
    trace
        // BLK_NUMBER is (defcomputed ...)
        // TOTL_TXN_NUMBER is (defcomputed ...)
        .sysiTxnNumber(sysiTransactionNumber)
        .userTxnNumber(userTransactionNumber)
        .sysfTxnNumber(sysfTransactionNumber)
        .sysi(category == SYSI)
        .user(category == USER)
        .sysf(category == SYSF)
        // CMPTN, HUB, RLP flags get traced by the rows themselves
        .ct(ct)
        .ctMax(ctMax())
        .proverRelativeUserTxnNumberMax(relativeUserTxNumMax)
        .proverUserTxnNumberMax(totalUserTransactionsInConflation);
    // GAS_CUMULATIVE gets traced for USER transactions only

    if (this instanceof UserTransaction userTransaction) {
      trace.gasCumulative(Bytes.ofUnsignedLong(userTransaction.txn.getAccumulatedGasUsedInBlock()));
    }
  }
}
