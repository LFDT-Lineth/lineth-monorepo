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
import static net.consensys.linea.zktracer.types.Conversions.bigIntegerToBytes;

import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.types.TransactionProcessingMetadata;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.plugin.data.ProcessableBlockHeader;

public class HubRowForUserTransactions extends HubRow {
  public final TransactionProcessingMetadata txn;

  public HubRowForUserTransactions(
      final ProcessableBlockHeader header, final TransactionProcessingMetadata txn) {
    super(header, txn.getHub());
    this.txn = txn;
  }

  @Override
  public void traceRow(Trace.Txndata trace) {
    super.traceRow(trace);

    trace
        // BTC columns are traced indiscriminately for all HubRow's in the parent class
        .pHubToAddressHi(txn.getEffectiveRecipient().slice(0, 4).toLong())
        .pHubToAddressLo(txn.getEffectiveRecipient().slice(4, LLARGE))
        .pHubFromAddressHi(txn.getSender().slice(0, 4).toLong())
        .pHubFromAddressLo(txn.getSender().slice(4, LLARGE))
        .pHubIsDeployment(txn.isDeployment())
        .pHubNonce(Bytes.ofUnsignedLong(txn.getBesuTransaction().getNonce()))
        .pHubValue(bigIntegerToBytes(txn.getBesuTransaction().getValue().getAsBigInteger()))
        .pHubGasLimit(txn.getBesuTransaction().getGasLimit())
        .pHubGasPrice(Bytes.ofUnsignedLong(txn.getEffectiveGasPrice()))
        .pHubGasInitiallyAvailable(txn.getInitiallyAvailableGas())
        .pHubCallDataSize(txn.isMessageCall() ? txn.getBesuTransaction().getPayload().size() : 0)
        .pHubInitCodeSize(txn.isDeployment() ? txn.getBesuTransaction().getPayload().size() : 0)
        .pHubHasEip1559GasSemantics(txn.getBesuTransaction().getType().supports1559FeeMarket())
        .pHubRequiresEvmExecution(txn.requiresEvmExecution())
        .pHubCopyTxcd(txn.copyTransactionCallData())
        .pHubCfi(txn.getCodeFragmentIndex())
        .pHubInitBalance(bigIntegerToBytes(txn.getInitialBalance()))
        .pHubStatusCode(txn.statusCode())
        .pHubGasLeftover(txn.getLeftoverGas())
        .pHubRefundCounterFinal(txn.getRefundCounterMax())
        .pHubRefundEffective(txn.computeRefunded())
    // EIP-4844, EIP-2935, NOOP flags aswell as SYST_TXN_DATA_k not set for USER transactions
    ;
  }
}
