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
package net.consensys.linea.zktracer.module.txndata.transactions;

import static net.consensys.linea.zktracer.Trace.HISTORY_SERVE_WINDOW;
import static net.consensys.linea.zktracer.module.hub.TransactionProcessingType.SYSI;
import static net.consensys.linea.zktracer.module.txndata.rows.computationRows.WcpRow.smallCallToIszero;

import net.consensys.linea.zktracer.module.txndata.TxnData;
import net.consensys.linea.zktracer.module.txndata.TxnDataOperation;
import net.consensys.linea.zktracer.module.txndata.rows.computationRows.EucRow;
import net.consensys.linea.zktracer.module.txndata.rows.computationRows.WcpRow;
import net.consensys.linea.zktracer.module.txndata.rows.hubRows.HubRowForSystemTransactions;
import net.consensys.linea.zktracer.module.txndata.rows.hubRows.Type;
import net.consensys.linea.zktracer.types.EWord;
import org.apache.tuweni.bytes.Bytes;

public class SysiEip2935Transaction extends TxnDataOperation {

  public static final short NB_ROWS_TXN_DATA_SYSI_EIP_2935 = 3;

  @Override
  protected int ctMax() {
    return NB_ROWS_TXN_DATA_SYSI_EIP_2935 - 1;
  }

  public SysiEip2935Transaction(final TxnData txnData) {
    super(txnData, SYSI);
    process();
  }

  private void process() {
    hubRow();
    detectTheGenesisBlockComputationRow();
    computePreviousBlockNumberModulo8191ComputationRow();
  }

  protected void hubRow() {

    HubRowForSystemTransactions hubRow =
        new HubRowForSystemTransactions(blockHeader, hub, Type.EIP2935);

    hubRow.systemTransactionData1 = EWord.of(previousBlockNumber());
    hubRow.systemTransactionData2 = (short) (previousBlockNumber() % HISTORY_SERVE_WINDOW);
    hubRow.systemTransactionData3 = EWord.of(EWord.of(previousBlockHash()).hi());
    hubRow.systemTransactionData4 = EWord.of(EWord.of(previousBlockHash()).lo());
    hubRow.systemTransactionData5 = currentBlockIsGenesisBlock();

    rows.add(hubRow);
  }

  private void detectTheGenesisBlockComputationRow() {
    WcpRow row = smallCallToIszero(wcp, blockHeader.getNumber());
    rows.add(row);
  }

  private void computePreviousBlockNumberModulo8191ComputationRow() {
    EucRow row = EucRow.callToEuc(euc, previousBlockNumber(), HISTORY_SERVE_WINDOW);
    rows.add(row);
  }

  private long previousBlockNumber() {
    return currentBlockIsGenesisBlock() ? 0 : blockHeader.getNumber() - 1;
  }

  private boolean currentBlockIsGenesisBlock() {
    return blockHeader.getNumber() == 0;
  }

  private Bytes previousBlockHash() {
    return currentBlockIsGenesisBlock() ? Bytes.EMPTY : blockHeader.getParentHash();
  }
}
