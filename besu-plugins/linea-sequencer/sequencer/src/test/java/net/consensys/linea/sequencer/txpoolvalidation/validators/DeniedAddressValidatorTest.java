/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.sequencer.txpoolvalidation.validators;

import static net.consensys.linea.utils.EIP7702TestUtils.addressFromKeyPair;
import static net.consensys.linea.utils.EIP7702TestUtils.createCodeDelegation;
import static net.consensys.linea.utils.EIP7702TestUtils.createDelegateCodeTransaction;
import static net.consensys.linea.utils.EIP7702TestUtils.createKeyPair;
import static org.assertj.core.api.Assertions.assertThat;

import java.util.List;
import java.util.Optional;
import java.util.Set;
import java.util.concurrent.atomic.AtomicReference;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.crypto.KeyPair;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.CodeDelegation;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

class DeniedAddressValidatorTest {

  // Keypairs for creating real CodeDelegation instances
  private static final KeyPair DENIED_KEY_PAIR =
      createKeyPair("1111111111111111111111111111111111111111111111111111111111111111");
  private static final KeyPair NOT_DENIED_KEY_PAIR =
      createKeyPair("2222222222222222222222222222222222222222222222222222222222222222");

  // Addresses derived from the keypairs (authorizer addresses for delegations)
  private static final Address DENIED = addressFromKeyPair(DENIED_KEY_PAIR);
  private static final Address NOT_DENIED = addressFromKeyPair(NOT_DENIED_KEY_PAIR);

  // Additional address for delegation targets
  private static final Address DELEGATION_TARGET =
      Address.fromHexString("0x0000000000000000000000000000000000001234");

  private static final String DENIED_SUFFIX =
      " is blocked as appearing on the SDN or other legally prohibited list";

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

    assertDeniedWithMessage(transaction, "sender " + DENIED + DENIED_SUFFIX);
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

    assertDeniedWithMessage(transaction, "recipient " + DENIED + DENIED_SUFFIX);
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

  // --- EIP-7702 authorization list tests ---

  @Test
  void delegateCodeTxPassesWhenNoAuthEntriesOnDenyList() {
    final CodeDelegation delegation = createCodeDelegation(NOT_DENIED_KEY_PAIR, DELEGATION_TARGET);
    final Transaction transaction =
        createDelegateCodeTransaction(NOT_DENIED_KEY_PAIR, NOT_DENIED, List.of(delegation));

    assertThat(validator.validateTransaction(transaction, false, false)).isEmpty();
  }

  @Test
  void deniedIfAuthorizationAuthorityOnDenyList() {
    final CodeDelegation delegation = createCodeDelegation(DENIED_KEY_PAIR, DELEGATION_TARGET);
    final Transaction transaction =
        createDelegateCodeTransaction(NOT_DENIED_KEY_PAIR, NOT_DENIED, List.of(delegation));

    assertDeniedWithMessage(transaction, "authorization authority " + DENIED + DENIED_SUFFIX);
  }

  @Test
  void deniedIfAuthorizationAddressOnDenyList() {
    final CodeDelegation delegation = createCodeDelegation(NOT_DENIED_KEY_PAIR, DENIED);
    final Transaction transaction =
        createDelegateCodeTransaction(NOT_DENIED_KEY_PAIR, NOT_DENIED, List.of(delegation));

    assertDeniedWithMessage(transaction, "authorization address " + DENIED + DENIED_SUFFIX);
  }

  @Test
  void checksAllTuplesNotJustFirst() {
    final CodeDelegation cleanDelegation =
        createCodeDelegation(NOT_DENIED_KEY_PAIR, DELEGATION_TARGET);
    final CodeDelegation deniedDelegation = createCodeDelegation(NOT_DENIED_KEY_PAIR, DENIED);
    final Transaction transaction =
        createDelegateCodeTransaction(
            NOT_DENIED_KEY_PAIR, NOT_DENIED, List.of(cleanDelegation, deniedDelegation));

    assertDeniedWithMessage(transaction, "authorization address " + DENIED + DENIED_SUFFIX);
  }

  private void assertDeniedWithMessage(
      final Transaction transaction, final String expectedMessage) {
    final Optional<String> result = validator.validateTransaction(transaction, false, false);
    assertThat(result).isPresent();
    assertThat(result.get()).isEqualTo(expectedMessage);
  }
}
