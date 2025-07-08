/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */

package net.consensys.linea.sequencer.txpoolvalidation.validators;

import java.util.Optional;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
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
    gasLimitValidator = new GasLimitValidator(MAX_TX_GAS_LIMIT);
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
