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
package net.consensys.linea.zktracer.module.txndata.cancun.transactions;

import static net.consensys.linea.zktracer.module.hub.TransactionProcessingType.SYSF;
import static net.consensys.linea.zktracer.module.txndata.cancun.rows.hubRows.Type.NOOP;

import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.txndata.cancun.CancunTxnData;
import net.consensys.linea.zktracer.module.txndata.cancun.CancunTxnDataOperation;
import net.consensys.linea.zktracer.module.txndata.cancun.rows.computationRows.NoopRow;
import net.consensys.linea.zktracer.module.txndata.cancun.rows.hubRows.HubRowForSystemTransactions;

public class SysfNoopTransaction extends CancunTxnDataOperation {

  public static final short NB_ROWS_TXN_DATA_SYSF_NOOP = 2;

  public final CancunTxnData txnData;

  @Override
  protected int ctMax() {
    return NB_ROWS_TXN_DATA_SYSF_NOOP - 1;
  }

  public SysfNoopTransaction(CancunTxnData txnData) {
    super(txnData, SYSF);
    this.txnData = txnData;
    process();
  }

  private void process() {
    rows.add(new HubRowForSystemTransactions(txnData.getCurrentBlockHeader(), txnData.hub(), NOOP));
    rows.add(new NoopRow());
  }

  @Override
  public void traceTransaction(Trace.Txndata trace) {}
}
