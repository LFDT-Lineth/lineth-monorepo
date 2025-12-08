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

package net.consensys.linea.zktracer.module.txndata.shanghai;

import static net.consensys.linea.zktracer.TraceShanghai.Txndata.NB_ROWS_TYPE_0;
import static net.consensys.linea.zktracer.TraceShanghai.Txndata.NB_ROWS_TYPE_1;
import static net.consensys.linea.zktracer.TraceShanghai.Txndata.NB_ROWS_TYPE_2;

import net.consensys.linea.zktracer.module.euc.Euc;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.txndata.london.LondonTxnData;
import net.consensys.linea.zktracer.module.wcp.Wcp;
import net.consensys.linea.zktracer.types.TransactionProcessingMetadata;

public class ShanghaiTxnData extends LondonTxnData {

  private static final int NB_WCP_EUC_ROWS_FRONTIER_ACCESS_LIST_SHANGHAI = 9;

  public ShanghaiTxnData(Hub hub, Wcp wcp, Euc euc) {
    super(hub, wcp, euc);
  }

  @Override
  public void traceEndTx(TransactionProcessingMetadata tx) {
    operations()
        .add(
            new ShanghaiTxndataOperation(
                wcp(),
                euc(),
                tx,
                NB_ROWS_TYPE_0,
                NB_ROWS_TYPE_1,
                NB_ROWS_TYPE_2,
                NB_WCP_EUC_ROWS_FRONTIER_ACCESS_LIST_SHANGHAI));
  }
}
