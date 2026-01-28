/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.sequencer.txselection.selectors;

import static net.consensys.linea.sequencer.txselection.LineaTransactionSelectionResult.TX_FILTERED_ADDRESS_FROM;
import static net.consensys.linea.sequencer.txselection.LineaTransactionSelectionResult.TX_FILTERED_ADDRESS_TO;
import static org.assertj.core.api.Assertions.assertThat;
import static org.hyperledger.besu.plugin.data.TransactionSelectionResult.SELECTED;
import static org.mockito.Mockito.doReturn;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

import java.util.HashSet;
import java.util.Optional;
import java.util.Set;
import java.util.concurrent.atomic.AtomicReference;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.PendingTransaction;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.plugin.services.txselection.TransactionEvaluationContext;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

class AllowedAddressTransactionSelectorTest {

  private static final Address SENDER_ADDRESS =
      Address.fromHexString("0x1234567890123456789012345678901234567890");
  private static final Address RECIPIENT_ADDRESS =
      Address.fromHexString("0xabcdefabcdefabcdefabcdefabcdefabcdefabcd");

  private AtomicReference<Set<Address>> deniedAddresses;
  private AllowedAddressTransactionSelector selector;

  @BeforeEach
  void setUp() {
    deniedAddresses = new AtomicReference<>(new HashSet<>());
    selector = new AllowedAddressTransactionSelector(deniedAddresses);
  }

  @Test
  void selectsTransactionWhenNeitherAddressIsDenied() {
    final TransactionEvaluationContext context = createContext(SENDER_ADDRESS, RECIPIENT_ADDRESS);

    final var result = selector.evaluateTransactionPreProcessing(context);

    assertThat(result).isEqualTo(SELECTED);
  }

  @Test
  void rejectsTransactionWhenSenderIsDenied() {
    deniedAddresses.set(Set.of(SENDER_ADDRESS));
    final TransactionEvaluationContext context = createContext(SENDER_ADDRESS, RECIPIENT_ADDRESS);

    final var result = selector.evaluateTransactionPreProcessing(context);

    assertThat(result).isEqualTo(TX_FILTERED_ADDRESS_FROM);
  }

  @Test
  void rejectsTransactionWhenRecipientIsDenied() {
    deniedAddresses.set(Set.of(RECIPIENT_ADDRESS));
    final TransactionEvaluationContext context = createContext(SENDER_ADDRESS, RECIPIENT_ADDRESS);

    final var result = selector.evaluateTransactionPreProcessing(context);

    assertThat(result).isEqualTo(TX_FILTERED_ADDRESS_TO);
  }

  @Test
  void senderDenialTakesPrecedenceOverRecipientDenial() {
    deniedAddresses.set(Set.of(SENDER_ADDRESS, RECIPIENT_ADDRESS));
    final TransactionEvaluationContext context = createContext(SENDER_ADDRESS, RECIPIENT_ADDRESS);

    final var result = selector.evaluateTransactionPreProcessing(context);

    assertThat(result).isEqualTo(TX_FILTERED_ADDRESS_FROM);
  }

  @Test
  void selectsContractCreationTransactionWhenSenderNotDenied() {
    final TransactionEvaluationContext context = createContext(SENDER_ADDRESS, null);

    final var result = selector.evaluateTransactionPreProcessing(context);

    assertThat(result).isEqualTo(SELECTED);
  }

  @Test
  void denyListIsReadDynamically() {
    final TransactionEvaluationContext context = createContext(SENDER_ADDRESS, RECIPIENT_ADDRESS);

    // Initially not denied
    var result = selector.evaluateTransactionPreProcessing(context);
    assertThat(result).isEqualTo(SELECTED);

    // Update deny list to include sender
    deniedAddresses.set(Set.of(SENDER_ADDRESS));
    result = selector.evaluateTransactionPreProcessing(context);
    assertThat(result).isEqualTo(TX_FILTERED_ADDRESS_FROM);

    // Update deny list to only include recipient
    deniedAddresses.set(Set.of(RECIPIENT_ADDRESS));
    result = selector.evaluateTransactionPreProcessing(context);
    assertThat(result).isEqualTo(TX_FILTERED_ADDRESS_TO);

    // Clear deny list
    deniedAddresses.set(Set.of());
    result = selector.evaluateTransactionPreProcessing(context);
    assertThat(result).isEqualTo(SELECTED);
  }

  @Test
  void postProcessingAlwaysReturnsSelected() {
    deniedAddresses.set(Set.of(SENDER_ADDRESS, RECIPIENT_ADDRESS));
    final TransactionEvaluationContext context = createContext(SENDER_ADDRESS, RECIPIENT_ADDRESS);

    final var result = selector.evaluateTransactionPostProcessing(context, null);

    assertThat(result).isEqualTo(SELECTED);
  }

  private TransactionEvaluationContext createContext(
      final Address sender, final Address recipient) {
    final Transaction transaction = mock(Transaction.class);
    when(transaction.getSender()).thenReturn(sender);
    doReturn(Optional.ofNullable(recipient)).when(transaction).getTo();

    final PendingTransaction pendingTransaction = mock(PendingTransaction.class);
    when(pendingTransaction.getTransaction()).thenReturn(transaction);

    final TransactionEvaluationContext context = mock(TransactionEvaluationContext.class);
    when(context.getPendingTransaction()).thenReturn(pendingTransaction);

    return context;
  }
}
