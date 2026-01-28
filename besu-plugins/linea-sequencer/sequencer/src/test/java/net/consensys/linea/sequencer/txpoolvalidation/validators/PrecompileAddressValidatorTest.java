/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.sequencer.txpoolvalidation.validators;

import static org.assertj.core.api.Assertions.assertThat;

import java.util.Optional;
import java.util.stream.Stream;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.MethodSource;

class PrecompileAddressValidatorTest {

  private static final Address REGULAR_ADDRESS =
      Address.fromHexString("0x0000000000000000000000000000000000001000");

  private PrecompileAddressValidator validator;

  @BeforeEach
  void setUp() {
    validator = new PrecompileAddressValidator();
  }

  @Test
  void validatedIfRecipientIsRegularAddress() {
    final Transaction transaction =
        Transaction.builder()
            .sender(REGULAR_ADDRESS)
            .to(REGULAR_ADDRESS)
            .gasPrice(Wei.ZERO)
            .payload(Bytes.EMPTY)
            .build();

    assertThat(validator.validateTransaction(transaction, false, false)).isEmpty();
  }

  @Test
  void validatedForContractCreation() {
    final Transaction transaction =
        Transaction.builder()
            .sender(REGULAR_ADDRESS)
            .gasPrice(Wei.ZERO)
            .payload(Bytes.EMPTY)
            .build();

    assertThat(validator.validateTransaction(transaction, false, false)).isEmpty();
  }

  @ParameterizedTest(name = "precompile 0x{0} is rejected")
  @MethodSource("precompileAddresses")
  void deniedIfRecipientIsPrecompile(String addressHex) {
    final Address precompile = Address.fromHexString("0x" + addressHex);
    final Transaction transaction =
        Transaction.builder()
            .sender(REGULAR_ADDRESS)
            .to(precompile)
            .gasPrice(Wei.ZERO)
            .payload(Bytes.EMPTY)
            .build();

    final Optional<String> result = validator.validateTransaction(transaction, false, false);

    assertThat(result).isPresent();
    assertThat(result.get())
        .isEqualTo("destination address is a precompile address and cannot receive transactions");
  }

  static Stream<Arguments> precompileAddresses() {
    return Stream.of(
        Arguments.of("0000000000000000000000000000000000000001"), // ecRecover
        Arguments.of("0000000000000000000000000000000000000002"), // SHA256
        Arguments.of("0000000000000000000000000000000000000003"), // RIPEMD160
        Arguments.of("0000000000000000000000000000000000000004"), // identity
        Arguments.of("0000000000000000000000000000000000000005"), // modexp
        Arguments.of("0000000000000000000000000000000000000006"), // ecAdd
        Arguments.of("0000000000000000000000000000000000000007"), // ecMul
        Arguments.of("0000000000000000000000000000000000000008"), // ecPairing
        Arguments.of("0000000000000000000000000000000000000009"), // blake2f
        Arguments.of("000000000000000000000000000000000000000a"), // pointEvaluation
        Arguments.of("000000000000000000000000000000000000000b"),
        Arguments.of("000000000000000000000000000000000000000c"),
        Arguments.of("000000000000000000000000000000000000000d"),
        Arguments.of("000000000000000000000000000000000000000e"),
        Arguments.of("000000000000000000000000000000000000000f"),
        Arguments.of("0000000000000000000000000000000000000010"),
        Arguments.of("0000000000000000000000000000000000000011"),
        Arguments.of("0000000000000000000000000000000000000100"));
  }
}
