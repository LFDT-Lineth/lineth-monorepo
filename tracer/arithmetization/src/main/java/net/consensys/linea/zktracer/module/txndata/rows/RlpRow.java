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
package net.consensys.linea.zktracer.module.txndata.rows;

import static net.consensys.linea.zktracer.Trace.LLARGE;
import static net.consensys.linea.zktracer.types.Conversions.bigIntegerToBytes;

import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.types.TransactionProcessingMetadata;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Transaction;

public class RlpRow extends TxnDataRow {

  private final TransactionProcessingMetadata txn;

  public RlpRow(TransactionProcessingMetadata txn) {
    this.txn = txn;
  }

  @Override
  public void traceRow(Trace.Txndata trace) {
    Transaction besuTxn = txn.getBesuTransaction();
    trace
        .rlp(true)
        .pRlpTxType(txn.type())
        .pRlpType0(txn.type() == 0)
        .pRlpType1(txn.type() == 1)
        .pRlpType2(txn.type() == 2)
        .pRlpType3(txn.type() == 3)
        .pRlpType4(txn.type() == 4)
        .pRlpToAddressHi(
            txn.isMessageCall() ? txn.getEffectiveRecipient().slice(0, 4).toLong() : 0L)
        .pRlpToAddressLo(
            txn.isMessageCall() ? txn.getEffectiveRecipient().slice(4, LLARGE) : Bytes.EMPTY)
        .pRlpNonce(Bytes.ofUnsignedLong(besuTxn.getNonce()))
        .pRlpIsDeployment(txn.isDeployment())
        .pRlpValue(bigIntegerToBytes(besuTxn.getValue().getAsBigInteger()))
        .pRlpNumberOfZeroBytes(txn.numberOfZeroBytesInPayload())
        .pRlpNumberOfNonzeroBytes(txn.numberOfNonzeroBytesInPayload())
        .pRlpDataSize(besuTxn.getPayload().size())
        .pRlpGasLimit(besuTxn.getGasLimit())
        .pRlpGasPrice(
            besuTxn.getType().supports1559FeeMarket()
                ? Bytes.EMPTY
                : bigIntegerToBytes(besuTxn.getGasPrice().get().getAsBigInteger()))
        .pRlpMaxFeePerGas(
            besuTxn.getType().supports1559FeeMarket()
                ? bigIntegerToBytes(besuTxn.getMaxFeePerGas().get().getAsBigInteger())
                : Bytes.EMPTY)
        .pRlpMaxPriorityFeePerGas(
            besuTxn.getType().supports1559FeeMarket()
                ? bigIntegerToBytes(besuTxn.getMaxPriorityFeePerGas().get().getAsBigInteger())
                : Bytes.EMPTY)
        .pRlpNumberOfAccessListAddresses(txn.numberOfWarmedAddresses())
        .pRlpNumberOfAccessListStorageKeys(txn.numberOfWarmedStorageKeys())
        .pRlpChainId(txn.chainId())
        .pRlpCfi(txn.getCodeFragmentIndex())
        .pRlpRequiresEvmExecution(txn.requiresEvmExecution());
  }
}
