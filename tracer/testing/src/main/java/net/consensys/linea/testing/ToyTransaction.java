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

package net.consensys.linea.testing;

import static com.google.common.base.Preconditions.checkArgument;
import static net.consensys.linea.zktracer.Trace.LINEA_CHAIN_ID;
import static org.hyperledger.besu.crypto.SECPSignature.BYTES_REQUIRED;

import java.math.BigInteger;
import java.util.List;
import java.util.Optional;
import lombok.Builder;
import org.apache.tuweni.bytes.Bytes;
import org.bouncycastle.math.ec.custom.sec.SecP256K1Curve;
import org.hyperledger.besu.crypto.KeyPair;
import org.hyperledger.besu.crypto.SECPSignature;
import org.hyperledger.besu.datatypes.*;
import org.hyperledger.besu.ethereum.core.Transaction;

@Builder
public class ToyTransaction {
  private static final Wei DEFAULT_VALUE = Wei.ZERO;
  private static final Bytes DEFAULT_INPUT_DATA = Bytes.EMPTY;
  private static final long DEFAULT_GAS_LIMIT = 50_000L; // i.e. 21 000 + a bit
  private static final Wei DEFAULT_GAS_PRICE = Wei.of(10_000_000L);
  private static final TransactionType DEFAULT_TX_TYPE = TransactionType.FRONTIER;
  private static final Wei DEFAULT_MAX_FEE_PER_GAS = Wei.of(37_000_000_000L);
  private static final Wei DEFAULT_MAX_PRIORITY_FEE_PER_GAS = Wei.of(500_000_000L);

  private final ToyAccount to;
  private final Address toAddress;
  private final ToyAccount sender;
  private final Wei gasPrice;
  private final Long gasLimit;
  private final Wei value;
  private final TransactionType transactionType;
  private final Bytes payload;
  private final BigInteger chainId;
  private final KeyPair keyPair;
  private final List<AccessListEntry> accessList;
  private final Wei maxPriorityFeePerGas;
  private final Wei maxFeePerGas;
  private final Long nonce;
  private final Bytes signature;

  /** Customizations applied to the Lombok generated builder. */
  public static class ToyTransactionBuilder {

    /**
     * Builder method returning an instance of {@link Transaction}.
     *
     * @return an instance of {@link Transaction}
     */
    public Transaction build() {
      checkArgument(to == null || toAddress == null, "Useless to provide both to and toAddress");
      final Address recipientAddress =
          to == null
              ? toAddress
              : to.getAddress(); // if both are null, it means it's a deployment transaction
      final Transaction.Builder builder =
          Transaction.builder()
              .to(recipientAddress)
              .nonce(nonce != null ? nonce : sender.getNonce())
              .accessList(accessList)
              .type(Optional.ofNullable(transactionType).orElse(DEFAULT_TX_TYPE))
              .gasPrice(Optional.ofNullable(gasPrice).orElse(DEFAULT_GAS_PRICE))
              .gasLimit(Optional.ofNullable(gasLimit).orElse(DEFAULT_GAS_LIMIT))
              .value(Optional.ofNullable(value).orElse(DEFAULT_VALUE))
              .payload(Optional.ofNullable(payload).orElse(DEFAULT_INPUT_DATA))
              .chainId(Optional.ofNullable(chainId).orElse(BigInteger.valueOf(LINEA_CHAIN_ID)));

      if (transactionType == TransactionType.EIP1559) {
        builder.maxPriorityFeePerGas(
            Optional.ofNullable(maxPriorityFeePerGas).orElse(DEFAULT_MAX_PRIORITY_FEE_PER_GAS));
        builder.maxFeePerGas(Optional.ofNullable(maxFeePerGas).orElse(DEFAULT_MAX_FEE_PER_GAS));
      }

      if (signature != null) {
        checkArgument(keyPair == null, "Cannot provide both signature and keyPair");
        checkArgument(
            signature.size() == BYTES_REQUIRED, "Signature must be % bytes", BYTES_REQUIRED);
        checkArgument(sender.getAddress() != null, "Sender address must be provided");
        return builder
            .sender(sender.getAddress())
            .signature(SECPSignature.decode(signature, SecP256K1Curve.q))
            .build();
      } else {
        return builder.signAndBuild(keyPair);
      }
    }
  }
}
