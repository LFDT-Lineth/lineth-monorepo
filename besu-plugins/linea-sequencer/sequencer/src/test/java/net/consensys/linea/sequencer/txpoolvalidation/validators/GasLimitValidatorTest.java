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

package net.consensys.linea.sequencer.txpoolvalidation.validators;

import java.util.Optional;

import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.config.LineaTransactionPoolValidatorCliOptions;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Wei;
import org.junit.jupiter.api.Assertions;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

@Slf4j
@RequiredArgsConstructor
public class GasLimitValidatorTest {
  public static final int MAX_TX_GAS_LIMIT = 9_000_000;
  private GasLimitValidator gasLimitValidator;

  @BeforeEach
  public void initialize() {
    gasLimitValidator =
        new GasLimitValidator(
            LineaTransactionPoolValidatorCliOptions.create().toDomainObject().toBuilder()
                .maxTxGasLimit(MAX_TX_GAS_LIMIT)
                .build());
  }

  @Test
  public void validatedWithValidGasLimit() {
    final org.hyperledger.besu.ethereum.core.Transaction.Builder builder =
        org.hyperledger.besu.ethereum.core.Transaction.builder();
    final org.hyperledger.besu.ethereum.core.Transaction transaction =
        builder.gasLimit(MAX_TX_GAS_LIMIT).gasPrice(Wei.ZERO).payload(Bytes.EMPTY).build();
    Assertions.assertEquals(
        gasLimitValidator.validateTransaction(transaction, false, false), Optional.empty());
  }

  @Test
  public void rejectedWithMaxGasLimitPlusOne() {
    final org.hyperledger.besu.ethereum.core.Transaction.Builder builder =
        org.hyperledger.besu.ethereum.core.Transaction.builder();
    final org.hyperledger.besu.ethereum.core.Transaction transaction =
        builder.gasLimit(MAX_TX_GAS_LIMIT + 1).gasPrice(Wei.ZERO).payload(Bytes.EMPTY).build();
    Assertions.assertEquals(
        gasLimitValidator.validateTransaction(transaction, false, false).orElseThrow(),
        "Gas limit of transaction is greater than the allowed max of " + MAX_TX_GAS_LIMIT);
  }
}
