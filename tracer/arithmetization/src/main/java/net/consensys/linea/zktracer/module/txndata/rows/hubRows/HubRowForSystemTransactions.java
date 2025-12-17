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
package net.consensys.linea.zktracer.module.txndata.rows.hubRows;

import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.types.EWord;
import org.hyperledger.besu.plugin.data.ProcessableBlockHeader;

public class HubRowForSystemTransactions extends HubRow {

  public EWord systemTransactionData1;
  public short systemTransactionData2;
  public EWord systemTransactionData3;
  public EWord systemTransactionData4;
  public boolean systemTransactionData5;
  public final Type type;

  public HubRowForSystemTransactions(ProcessableBlockHeader header, Hub hub, Type type) {
    super(header, hub);
    this.type = type;
  }

  @Override
  public void traceRow(Trace.Txndata trace) {
    super.traceRow(trace);
    trace
        .pHubSystTxnData1(type == Type.NOOP ? EWord.ZERO : systemTransactionData1)
        .pHubSystTxnData2(type == Type.NOOP ? 0 : systemTransactionData2)
        .pHubSystTxnData3(type == Type.NOOP ? EWord.ZERO : systemTransactionData3)
        .pHubSystTxnData4(type == Type.NOOP ? EWord.ZERO : systemTransactionData4)
        .pHubSystTxnData5(!(type == Type.NOOP) && systemTransactionData5)
        .pHubEip2935(type == Type.EIP2935)
        .pHubEip4788(type == Type.EIP4788)
        .pHubNoop(type == Type.NOOP);
  }
}
