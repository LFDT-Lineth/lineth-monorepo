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

import static net.consensys.linea.zktracer.Trace.HISTORY_BUFFER_LENGTH;
import static net.consensys.linea.zktracer.module.hub.TransactionProcessingType.SYSI;
import static net.consensys.linea.zktracer.module.hub.section.systemTransaction.EIP4788BeaconBlockRootSection.HISTORY_BUFFER_LENGTH_BI;
import static net.consensys.linea.zktracer.module.txndata.rows.computationRows.EucRow.callToEuc;
import static net.consensys.linea.zktracer.module.txndata.rows.computationRows.WcpRow.smallCallToIszero;
import static net.consensys.linea.zktracer.module.txndata.rows.hubRows.Type.EIP4788;
import static net.consensys.linea.zktracer.types.Conversions.longToUnsignedBigInteger;

import java.math.BigInteger;
import net.consensys.linea.zktracer.module.txndata.TxnData;
import net.consensys.linea.zktracer.module.txndata.TxnDataOperation;
import net.consensys.linea.zktracer.module.txndata.rows.computationRows.EucRow;
import net.consensys.linea.zktracer.module.txndata.rows.computationRows.WcpRow;
import net.consensys.linea.zktracer.module.txndata.rows.hubRows.HubRowForSystemTransactions;
import net.consensys.linea.zktracer.types.EWord;
import org.apache.tuweni.bytes.Bytes32;

public class SysiEip4788Transaction extends TxnDataOperation {

  public static final short NB_ROWS_TXN_DATA_SYSI_EIP_4788 = 3;

  public SysiEip4788Transaction(final TxnData txnData) {
    super(txnData, SYSI);
    process();
  }

  private void process() {
    hubRow();
    computeTimestampModulo8191ComputationRow();
    detectTheGenesisBlockComputationRow();
  }

  protected void hubRow() {
    HubRowForSystemTransactions hubRow = new HubRowForSystemTransactions(blockHeader, hub, EIP4788);

    final BigInteger timestamp = longToUnsignedBigInteger(blockHeader.getTimestamp());
    Bytes32 parentBeaconBlockRoot =
        blockHeader.getParentBeaconBlockRoot().isPresent()
            ? blockHeader.getParentBeaconBlockRoot().get()
            : Bytes32.ZERO;

    hubRow.systemTransactionData1 = EWord.of(timestamp);
    hubRow.systemTransactionData2 = timestamp.mod(HISTORY_BUFFER_LENGTH_BI).shortValueExact();
    hubRow.systemTransactionData3 = EWord.of(EWord.of(parentBeaconBlockRoot).hi());
    hubRow.systemTransactionData4 = EWord.of(EWord.of(parentBeaconBlockRoot).lo());
    hubRow.systemTransactionData5 = blockHeader.getNumber() == 0;

    rows.add(hubRow);
  }

  private void computeTimestampModulo8191ComputationRow() {
    EucRow row = callToEuc(euc, blockHeader.getTimestamp(), HISTORY_BUFFER_LENGTH);
    rows.add(row);
  }

  private void detectTheGenesisBlockComputationRow() {
    WcpRow row = smallCallToIszero(wcp, blockHeader.getNumber());
    rows.add(row);
  }

  @Override
  protected int ctMax() {
    return NB_ROWS_TXN_DATA_SYSI_EIP_4788 - 1;
  }
}
