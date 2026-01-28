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
import java.util.Set;
import java.util.concurrent.atomic.AtomicReference;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

class DeniedAddressValidatorTest {

  private static final Address DENIED =
      Address.fromHexString("0x0000000000000000000000000000000000001000");
  private static final Address NOT_DENIED =
      Address.fromHexString("0x0000000000000000000000000000000000001001");

  private DeniedAddressValidator validator;

  @BeforeEach
  void setUp() {
    validator = new DeniedAddressValidator(new AtomicReference<>(Set.of(DENIED)));
  }

  @Test
  void validatedIfNeitherAddressOnDenyList() {
    final Transaction transaction =
        Transaction.builder()
            .sender(NOT_DENIED)
            .to(NOT_DENIED)
            .gasPrice(Wei.ZERO)
            .payload(Bytes.EMPTY)
            .build();

    assertThat(validator.validateTransaction(transaction, false, false)).isEmpty();
  }

  @Test
  void deniedIfSenderOnDenyList() {
    final Transaction transaction =
        Transaction.builder()
            .sender(DENIED)
            .to(NOT_DENIED)
            .gasPrice(Wei.ZERO)
            .payload(Bytes.EMPTY)
            .build();

    final Optional<String> result = validator.validateTransaction(transaction, false, false);

    assertThat(result).isPresent();
    assertThat(result.get())
        .isEqualTo(
            "sender 0x0000000000000000000000000000000000001000 is blocked as appearing on the SDN or other legally prohibited list");
  }

  @Test
  void deniedIfRecipientOnDenyList() {
    final Transaction transaction =
        Transaction.builder()
            .sender(NOT_DENIED)
            .to(DENIED)
            .gasPrice(Wei.ZERO)
            .payload(Bytes.EMPTY)
            .build();

    final Optional<String> result = validator.validateTransaction(transaction, false, false);

    assertThat(result).isPresent();
    assertThat(result.get())
        .isEqualTo(
            "recipient 0x0000000000000000000000000000000000001000 is blocked as appearing on the SDN or other legally prohibited list");
  }

  @Test
  void deniedIfBothAddressesOnDenyList() {
    final Transaction transaction =
        Transaction.builder()
            .sender(DENIED)
            .to(DENIED)
            .gasPrice(Wei.ZERO)
            .payload(Bytes.EMPTY)
            .build();

    final Optional<String> result = validator.validateTransaction(transaction, false, false);

    // Sender check happens first
    assertThat(result).isPresent();
    assertThat(result.get()).contains("sender");
  }

  @Test
  void validatedIfDenyListEmpty() {
    validator = new DeniedAddressValidator(new AtomicReference<>(Set.of()));

    final Transaction transaction =
        Transaction.builder()
            .sender(DENIED)
            .to(DENIED)
            .gasPrice(Wei.ZERO)
            .payload(Bytes.EMPTY)
            .build();

    assertThat(validator.validateTransaction(transaction, false, false)).isEmpty();
  }

  @Test
  void validatedForContractCreation() {
    final Transaction transaction =
        Transaction.builder().sender(NOT_DENIED).gasPrice(Wei.ZERO).payload(Bytes.EMPTY).build();

    assertThat(validator.validateTransaction(transaction, false, false)).isEmpty();
  }

  @Test
  void denyListCanBeUpdatedDynamically() {
    final AtomicReference<Set<Address>> denyList = new AtomicReference<>(Set.of());
    validator = new DeniedAddressValidator(denyList);

    final Transaction transaction =
        Transaction.builder()
            .sender(DENIED)
            .to(NOT_DENIED)
            .gasPrice(Wei.ZERO)
            .payload(Bytes.EMPTY)
            .build();

    assertThat(validator.validateTransaction(transaction, false, false)).isEmpty();

    denyList.set(Set.of(DENIED));

    assertThat(validator.validateTransaction(transaction, false, false)).isPresent();
  }
}
