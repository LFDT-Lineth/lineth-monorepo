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

package net.consensys.linea.zktracer.module.hub.fragment.transaction;

import static net.consensys.linea.zktracer.types.AddressUtils.hiPart;
import static net.consensys.linea.zktracer.types.AddressUtils.loPart;
import static net.consensys.linea.zktracer.types.Conversions.bigIntegerToBytes;

import lombok.RequiredArgsConstructor;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.hub.fragment.TraceFragment;
import net.consensys.linea.zktracer.types.TransactionProcessingMetadata;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Transaction;

@RequiredArgsConstructor
public final class UserTransactionFragment implements TraceFragment {
  private final TransactionProcessingMetadata transactionProcessingMetadata;

  @Override
  public Trace.Hub trace(Trace.Hub trace) {
    final Transaction tx = transactionProcessingMetadata.getBesuTransaction();
    final Address to = transactionProcessingMetadata.getEffectiveRecipient();
    final Address from = transactionProcessingMetadata.getSender();

    return trace
        .peekAtTransaction(true)
        .pTransactionFromAddressHi(hiPart(from))
        .pTransactionFromAddressLo(loPart(from))
        .pTransactionNonce(Bytes.ofUnsignedLong(tx.getNonce()))
        .pTransactionInitialBalance(
            bigIntegerToBytes(transactionProcessingMetadata.getInitialBalance()))
        .pTransactionValue(bigIntegerToBytes(tx.getValue().getAsBigInteger()))
        .pTransactionToAddressHi(hiPart(to))
        .pTransactionToAddressLo(loPart(to))
        .pTransactionRequiresEvmExecution(transactionProcessingMetadata.requiresEvmExecution())
        .pTransactionCopyTxcd(transactionProcessingMetadata.copyTransactionCallData())
        .pTransactionIsDeployment(tx.getTo().isEmpty())
        .pTransactionTransactionTypeSupportsEip1559GasSemantics(
            tx.getType().supports1559FeeMarket())
        .pTransactionTransactionTypeSupportsDelegationLists(tx.getType().supportsDelegateCode())
        .pTransactionGasLimit(tx.getGasLimit())
        .pTransactionGasInitiallyAvailable(transactionProcessingMetadata.getInitiallyAvailableGas())
        .pTransactionGasPrice(
            Bytes.minimalBytes(transactionProcessingMetadata.getEffectiveGasPrice()))
        .pTransactionPriorityFeePerGas(
            Bytes.minimalBytes(transactionProcessingMetadata.feeRateForCoinbase()))
        .pTransactionBasefee(Bytes.minimalBytes(transactionProcessingMetadata.getBaseFee()))
        .pTransactionCallDataSize(tx.getData().map(Bytes::size).orElse(0))
        .pTransactionInitCodeSize(tx.getInit().map(Bytes::size).orElse(0))
        .pTransactionStatusCode(transactionProcessingMetadata.statusCode())
        .pTransactionGasLeftover(transactionProcessingMetadata.getLeftoverGas())
        .pTransactionRefundCounterInfinity(transactionProcessingMetadata.getRefundCounterMax())
        .pTransactionRefundEffective(transactionProcessingMetadata.getGasRefunded())
        .pTransactionCoinbaseAddressHi(hiPart(transactionProcessingMetadata.getCoinbaseAddress()))
        .pTransactionCoinbaseAddressLo(loPart(transactionProcessingMetadata.getCoinbaseAddress()))
        .pTransactionLengthOfDelegationList(
            transactionProcessingMetadata.getBesuTransaction().getCodeDelegationList().isPresent()
                ? transactionProcessingMetadata
                    .getBesuTransaction()
                    .getCodeDelegationList()
                    .get()
                    .size()
                : 0)
        .pTransactionNumberOfSuccessfulSenderDelegations(
            transactionProcessingMetadata.getNumberOfSuccessfulSenderDelegations());
  }
}
