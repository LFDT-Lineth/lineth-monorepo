/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.sequencer.txselection.selectors;

import static net.consensys.linea.sequencer.txselection.LineaTransactionSelectionResult.TX_FILTERED_ADDRESS_AUTHORIZATION;
import static net.consensys.linea.sequencer.txselection.LineaTransactionSelectionResult.TX_FILTERED_ADDRESS_FROM;
import static net.consensys.linea.sequencer.txselection.LineaTransactionSelectionResult.TX_FILTERED_ADDRESS_TO;
import static net.consensys.linea.utils.EIP7702TestUtils.addressFromKeyPair;
import static net.consensys.linea.utils.EIP7702TestUtils.createCodeDelegation;
import static net.consensys.linea.utils.EIP7702TestUtils.createDelegateCodeTransaction;
import static net.consensys.linea.utils.EIP7702TestUtils.createKeyPair;
import static org.assertj.core.api.Assertions.assertThat;
import static org.hyperledger.besu.plugin.data.TransactionSelectionResult.SELECTED;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

import java.util.HashSet;
import java.util.List;
import java.util.Set;
import net.consensys.linea.config.ReloadableSet;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.crypto.KeyPair;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.CodeDelegation;
import org.hyperledger.besu.datatypes.PendingTransaction;
import org.hyperledger.besu.datatypes.TransactionType;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.hyperledger.besu.plugin.services.txselection.TransactionEvaluationContext;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

class AllowedAddressTransactionSelectorTest {

  // Keypairs for creating real Transaction and CodeDelegation instances
  private static final KeyPair SENDER_KEY_PAIR =
      createKeyPair("1111111111111111111111111111111111111111111111111111111111111111");
  private static final KeyPair AUTHORIZATION_KEY_PAIR =
      createKeyPair("3333333333333333333333333333333333333333333333333333333333333333");

  // Addresses derived from keypairs
  private static final Address SENDER_ADDRESS = addressFromKeyPair(SENDER_KEY_PAIR);
  private static final Address AUTHORIZATION_ADDRESS = addressFromKeyPair(AUTHORIZATION_KEY_PAIR);

  // Fixed addresses for recipient and delegation target
  private static final Address RECIPIENT_ADDRESS =
      Address.fromHexString("0xabcdefabcdefabcdefabcdefabcdefabcdefabcd");
  private static final Address DELEGATION_TARGET =
      Address.fromHexString("0x0000000000000000000000000000000000001234");

  private ReloadableSet<Address> deniedAddresses;
  private AllowedAddressTransactionSelector selector;

  @BeforeEach
  void setUp() {
    deniedAddresses = new ReloadableSet<>(new HashSet<>());
    selector = new AllowedAddressTransactionSelector(deniedAddresses);
  }

  @Test
  void selectsTransactionWhenNeitherAddressIsDenied() {
    final TransactionEvaluationContext context = createContext(RECIPIENT_ADDRESS);

    final var result = selector.evaluateTransactionPreProcessing(context);

    assertThat(result).isEqualTo(SELECTED);
  }

  @Test
  void rejectsTransactionWhenSenderIsDenied() {
    deniedAddresses.swap(Set.of(SENDER_ADDRESS));
    final TransactionEvaluationContext context = createContext(RECIPIENT_ADDRESS);

    final var result = selector.evaluateTransactionPreProcessing(context);

    assertThat(result).isEqualTo(TX_FILTERED_ADDRESS_FROM);
  }

  @Test
  void rejectsTransactionWhenRecipientIsDenied() {
    deniedAddresses.swap(Set.of(RECIPIENT_ADDRESS));
    final TransactionEvaluationContext context = createContext(RECIPIENT_ADDRESS);

    final var result = selector.evaluateTransactionPreProcessing(context);

    assertThat(result).isEqualTo(TX_FILTERED_ADDRESS_TO);
  }

  @Test
  void senderDenialTakesPrecedenceOverRecipientDenial() {
    deniedAddresses.swap(Set.of(SENDER_ADDRESS, RECIPIENT_ADDRESS));
    final TransactionEvaluationContext context = createContext(RECIPIENT_ADDRESS);

    final var result = selector.evaluateTransactionPreProcessing(context);

    assertThat(result).isEqualTo(TX_FILTERED_ADDRESS_FROM);
  }

  @Test
  void selectsContractCreationTransactionWhenSenderNotDenied() {
    final TransactionEvaluationContext context = createContext(null);

    final var result = selector.evaluateTransactionPreProcessing(context);

    assertThat(result).isEqualTo(SELECTED);
  }

  @Test
  void denyListIsReadDynamically() {
    final TransactionEvaluationContext context = createContext(RECIPIENT_ADDRESS);

    // Initially not denied
    var result = selector.evaluateTransactionPreProcessing(context);
    assertThat(result).isEqualTo(SELECTED);

    // Update deny list to include sender
    deniedAddresses.swap(Set.of(SENDER_ADDRESS));
    result = selector.evaluateTransactionPreProcessing(context);
    assertThat(result).isEqualTo(TX_FILTERED_ADDRESS_FROM);

    // Update deny list to only include recipient
    deniedAddresses.swap(Set.of(RECIPIENT_ADDRESS));
    result = selector.evaluateTransactionPreProcessing(context);
    assertThat(result).isEqualTo(TX_FILTERED_ADDRESS_TO);

    // Clear deny list
    deniedAddresses.swap(Set.of());
    result = selector.evaluateTransactionPreProcessing(context);
    assertThat(result).isEqualTo(SELECTED);
  }

  @Test
  void postProcessingAlwaysReturnsSelected() {
    deniedAddresses.swap(Set.of(SENDER_ADDRESS, RECIPIENT_ADDRESS));
    final TransactionEvaluationContext context = createContext(RECIPIENT_ADDRESS);

    final var result = selector.evaluateTransactionPostProcessing(context, null);

    assertThat(result).isEqualTo(SELECTED);
  }

  // --- EIP-7702 authorization list tests ---

  @Test
  void selectsTransactionWhenAuthorizationListEntriesNotDenied() {
    final CodeDelegation delegation = createCodeDelegation(SENDER_KEY_PAIR, DELEGATION_TARGET);
    final TransactionEvaluationContext context =
        createContextWithDelegations(RECIPIENT_ADDRESS, List.of(delegation));

    final var result = selector.evaluateTransactionPreProcessing(context);

    assertThat(result).isEqualTo(SELECTED);
  }

  @Test
  void rejectsTransactionWhenAuthorizationAuthorityDenied() {
    deniedAddresses.swap(Set.of(AUTHORIZATION_ADDRESS));
    final CodeDelegation delegation =
        createCodeDelegation(AUTHORIZATION_KEY_PAIR, DELEGATION_TARGET);
    final TransactionEvaluationContext context =
        createContextWithDelegations(RECIPIENT_ADDRESS, List.of(delegation));

    final var result = selector.evaluateTransactionPreProcessing(context);

    assertThat(result).isEqualTo(TX_FILTERED_ADDRESS_AUTHORIZATION);
  }

  @Test
  void rejectsTransactionWhenAuthorizationAddressDenied() {
    deniedAddresses.swap(Set.of(AUTHORIZATION_ADDRESS));
    final CodeDelegation delegation = createCodeDelegation(SENDER_KEY_PAIR, AUTHORIZATION_ADDRESS);
    final TransactionEvaluationContext context =
        createContextWithDelegations(RECIPIENT_ADDRESS, List.of(delegation));

    final var result = selector.evaluateTransactionPreProcessing(context);

    assertThat(result).isEqualTo(TX_FILTERED_ADDRESS_AUTHORIZATION);
  }

  @Test
  void checksAllDelegationTuplesNotJustFirst() {
    deniedAddresses.swap(Set.of(AUTHORIZATION_ADDRESS));
    final CodeDelegation cleanDelegation = createCodeDelegation(SENDER_KEY_PAIR, DELEGATION_TARGET);
    final CodeDelegation deniedDelegation =
        createCodeDelegation(SENDER_KEY_PAIR, AUTHORIZATION_ADDRESS);
    final TransactionEvaluationContext context =
        createContextWithDelegations(RECIPIENT_ADDRESS, List.of(cleanDelegation, deniedDelegation));

    final var result = selector.evaluateTransactionPreProcessing(context);

    assertThat(result).isEqualTo(TX_FILTERED_ADDRESS_AUTHORIZATION);
  }

  private TransactionEvaluationContext createContext(final Address recipient) {
    final Transaction transaction = createSimpleTransaction(recipient);
    return wrapInContext(transaction);
  }

  private TransactionEvaluationContext createContextWithDelegations(
      final Address recipient, final List<CodeDelegation> delegations) {
    final Transaction transaction =
        createDelegateCodeTransaction(SENDER_KEY_PAIR, recipient, delegations);
    return wrapInContext(transaction);
  }

  private Transaction createSimpleTransaction(final Address recipient) {
    final Transaction.Builder builder =
        Transaction.builder()
            .type(TransactionType.FRONTIER)
            .nonce(0)
            .gasPrice(Wei.ZERO)
            .gasLimit(21_000)
            .value(Wei.ZERO)
            .payload(Bytes.EMPTY);

    if (recipient != null) {
      builder.to(recipient);
    }

    return builder.signAndBuild(SENDER_KEY_PAIR);
  }

  private TransactionEvaluationContext wrapInContext(final Transaction transaction) {
    final PendingTransaction pendingTransaction = mock(PendingTransaction.class);
    when(pendingTransaction.getTransaction()).thenReturn(transaction);

    final TransactionEvaluationContext context = mock(TransactionEvaluationContext.class);
    when(context.getPendingTransaction()).thenReturn(pendingTransaction);

    return context;
  }
}
