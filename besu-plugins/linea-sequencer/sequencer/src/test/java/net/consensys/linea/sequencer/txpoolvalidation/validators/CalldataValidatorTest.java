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
import net.consensys.linea.config.LineaTransactionPoolValidatorCliOptions;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Wei;
import org.junit.jupiter.api.Assertions;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

@Slf4j
@RequiredArgsConstructor
public class CalldataValidatorTest {
  public static final int MAX_TX_CALLDATA_SIZE = 10_000;
  private CalldataValidator calldataValidator;

  @BeforeEach
  public void initialize() {
    calldataValidator =
        new CalldataValidator(
            LineaTransactionPoolValidatorCliOptions.create().toDomainObject().toBuilder()
                .maxTxCalldataSize(MAX_TX_CALLDATA_SIZE)
                .build());
  }

  @Test
  public void validatedWithValidCalldata() {
    final org.hyperledger.besu.ethereum.core.Transaction.Builder builder =
        org.hyperledger.besu.ethereum.core.Transaction.builder();
    final org.hyperledger.besu.ethereum.core.Transaction transaction =
        builder.gasPrice(Wei.ZERO).payload(Bytes.random(MAX_TX_CALLDATA_SIZE)).build();
    Assertions.assertEquals(
        calldataValidator.validateTransaction(transaction, false, false), Optional.empty());
  }

  @Test
  public void rejectedWithTooBigCalldata() {
    final org.hyperledger.besu.ethereum.core.Transaction.Builder builder =
        org.hyperledger.besu.ethereum.core.Transaction.builder();
    final org.hyperledger.besu.ethereum.core.Transaction transaction =
        builder.gasPrice(Wei.ZERO).payload(Bytes.random(MAX_TX_CALLDATA_SIZE + 1)).build();
    Assertions.assertEquals(
        calldataValidator.validateTransaction(transaction, false, false).orElseThrow(),
        "Calldata of transaction is greater than the allowed max of " + MAX_TX_CALLDATA_SIZE);
  }
}
