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

package net.consensys.linea.blockcapture.snapshots;

import static org.hyperledger.besu.ethereum.core.Transaction.REPLAY_PROTECTED_V_BASE;
import static org.hyperledger.besu.ethereum.core.Transaction.REPLAY_PROTECTED_V_MIN;
import static org.hyperledger.besu.ethereum.core.Transaction.REPLAY_UNPROTECTED_V_BASE;

import java.math.BigInteger;
import java.util.List;
import java.util.Optional;
import lombok.Getter;
import lombok.Setter;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.DelegatingBytes;
import org.hyperledger.besu.crypto.SignatureAlgorithmFactory;
import org.hyperledger.besu.datatypes.AccessListEntry;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Quantity;
import org.hyperledger.besu.datatypes.TransactionType;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.hyperledger.besu.evm.internal.Words;

public class TransactionSnapshot {
  @Getter private final String r;
  @Getter private final String s;
  @Getter private final String v;
  @Getter private final TransactionType type;
  @Getter private final String sender;
  @Getter private final Optional<String> to;
  @Getter private final long nonce;
  @Getter private final String value;
  @Getter private final String payload;
  @Getter private final Optional<String> gasPrice;
  @Getter private final Optional<String> maxPriorityFeePerGas;
  @Getter private final Optional<String> maxFeePerGas;
  @Getter private final Optional<String> maxFeePerBlobGas;
  @Getter private final long gasLimit;
  @Getter private final BigInteger chainId;
  @Getter private final Optional<List<AccessListEntrySnapshot>> accessList;
  @Getter @Setter private TransactionResultSnapshot outcome;

  /**
   * Construct an initial snapshot from a given transaction. Observe that this is not yet complete
   * since it doesn't include the transaction outcomes.
   *
   * @param tx The transaction being recorded as a snapshot
   */
  public TransactionSnapshot(Transaction tx) {
    this.r = tx.getR().toString(16);
    this.s = tx.getS().toString(16);
    this.v =
        tx.getType() == TransactionType.FRONTIER
            ? tx.getV().toString(16)
            : tx.getYParity().toString(16);
    this.type = tx.getType();
    this.sender = tx.getSender().toHexString();
    this.to = tx.getTo().map(DelegatingBytes::toHexString);
    this.nonce = tx.getNonce();
    this.value = tx.getValue().toHexString();
    this.payload = tx.getPayload().toHexString();
    this.gasPrice = tx.getGasPrice().map(Quantity::toHexString);
    this.maxPriorityFeePerGas = tx.getMaxPriorityFeePerGas().map(Quantity::toHexString);
    this.maxFeePerGas = tx.getMaxFeePerGas().map(Quantity::toHexString);
    this.maxFeePerBlobGas = tx.getMaxFeePerBlobGas().map(Quantity::toHexString);
    this.gasLimit = tx.getGasLimit();
    this.chainId = tx.getChainId().orElse(null);
    this.accessList =
        tx.getAccessList().map(l -> l.stream().map(AccessListEntrySnapshot::from).toList());
  }

  /**
   * Set the outcome of the transaction so that the result is recorded and can be checked during the
   * replay itself.
   *
   * @param result The transaction process result to record.
   */
  public void setTransactionResult(TransactionResultSnapshot result) {
    this.outcome = result;
  }

  public static TransactionSnapshot of(Transaction tx) {
    return new TransactionSnapshot(tx);
  }

  public Transaction toTransaction() {
    BigInteger r = Bytes.fromHexStringLenient(this.r).toUnsignedBigInteger();
    BigInteger s = Bytes.fromHexStringLenient(this.s).toUnsignedBigInteger();
    BigInteger v = Bytes.fromHexStringLenient(this.v).toUnsignedBigInteger();

    final var tx =
        Transaction.builder()
            .type(this.type)
            .sender(Address.fromHexString(this.sender))
            .nonce(this.nonce)
            .value(Wei.fromHexString(this.value))
            .payload(Bytes.fromHexString(this.payload))
            .gasLimit(this.gasLimit);
    // Set the chainID (if it makes sense to do so).
    if (this.type != TransactionType.FRONTIER || v.compareTo(REPLAY_PROTECTED_V_MIN) > 0) {
      tx.chainId(this.chainId);
    }
    // Update v for legacy transactions
    if (this.type == TransactionType.FRONTIER) {
      if (v.compareTo(REPLAY_PROTECTED_V_MIN) > 0) {
        v = v.subtract(REPLAY_PROTECTED_V_BASE).subtract(chainId.multiply(BigInteger.TWO));
      } else {
        v = v.subtract(REPLAY_UNPROTECTED_V_BASE);
      }
    }
    // Set transaction signature
    tx.signature(SignatureAlgorithmFactory.getInstance().createSignature(r, s, v.byteValueExact()));
    //
    this.to.ifPresent(to -> tx.to(Words.toAddress(Bytes.fromHexString(to))));
    this.gasPrice.ifPresent(gasPrice -> tx.gasPrice(Wei.fromHexString(gasPrice)));
    this.maxPriorityFeePerGas.ifPresent(
        maxPriorityFeePerGas -> tx.maxPriorityFeePerGas(Wei.fromHexString(maxPriorityFeePerGas)));
    this.maxFeePerGas.ifPresent(maxFeePerGas -> tx.maxFeePerGas(Wei.fromHexString(maxFeePerGas)));
    this.maxFeePerBlobGas.ifPresent(
        maxFeePerBlobGas -> tx.maxFeePerBlobGas(Wei.fromHexString(maxFeePerBlobGas)));
    this.accessList.ifPresent(
        l ->
            tx.accessList(
                l.stream()
                    .map(
                        e ->
                            AccessListEntry.createAccessListEntry(
                                Address.fromHexString(e.address()), e.storageKeys()))
                    .toList()));

    return tx.build();
  }
}
