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

import static net.consensys.linea.zktracer.Trace.LLARGE;

import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.txndata.rows.TxnDataRow;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.plugin.data.ProcessableBlockHeader;

public abstract class HubRow extends TxnDataRow {

  public final ProcessableBlockHeader header;
  public final Address coinbase;

  public HubRow(ProcessableBlockHeader header, Hub hub) {
    this.header = header;
    this.coinbase = hub.coinbaseAddress();
  }

  public void traceRow(Trace.Txndata trace) {
    trace
        .hub(true)
        .pHubBtcBlockNumber(header.getNumber())
        .pHubBtcBlockGasLimit(Bytes.ofUnsignedLong(header.getGasLimit()))
        .pHubBtcBasefee(
            Bytes.ofUnsignedLong(header.getBaseFee().get().getAsBigInteger().longValueExact()))
        .pHubBtcTimestamp(Bytes.ofUnsignedLong(header.getTimestamp()))
        .pHubBtcCoinbaseAddressHi(coinbase.getBytes().slice(0, 4).toLong())
        .pHubBtcCoinbaseAddressLo(coinbase.getBytes().slice(4, LLARGE));
  }
}
