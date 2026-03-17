/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.sequencer.txselection.selectors;

import static net.consensys.linea.sequencer.txselection.LineaTransactionSelectionResult.TX_FILTERED_ADDRESS_CALLED;
import static org.assertj.core.api.Assertions.assertThat;
import static org.hyperledger.besu.plugin.data.TransactionSelectionResult.SELECTED;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

import java.util.HashSet;
import java.util.Set;
import java.util.concurrent.atomic.AtomicReference;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.datatypes.PendingTransaction;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.evm.frame.MessageFrame;
import org.hyperledger.besu.plugin.data.TransactionProcessingResult;
import org.hyperledger.besu.plugin.services.txselection.TransactionEvaluationContext;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

class DenylistExecutionSelectorTest {

  private static final Address CLEAN_ADDRESS =
      Address.fromHexString("0x1111111111111111111111111111111111111111");
  private static final Address DENIED_ADDRESS =
      Address.fromHexString("0x2222222222222222222222222222222222222222");

  private AtomicReference<Set<Address>> deniedAddresses;
  private DenylistOperationTracer tracer;
  private DenylistExecutionSelector selector;

  @BeforeEach
  void setUp() {
    deniedAddresses = new AtomicReference<>(new HashSet<>());
    tracer = new DenylistOperationTracer();
    selector = new DenylistExecutionSelector(deniedAddresses, tracer);
  }

  @Test
  void preProcessingAlwaysReturnsSelected() {
    deniedAddresses.set(Set.of(DENIED_ADDRESS));

    final var result = selector.evaluateTransactionPreProcessing(mockContext());

    assertThat(result).isEqualTo(SELECTED);
  }

  @Test
  void postProcessingReturnsSelectedWhenNoCalledAddressIsDenied() {
    deniedAddresses.set(Set.of(DENIED_ADDRESS));
    tracer.traceStartTransaction(null, null);
    tracer.traceContextEnter(mockFrame(CLEAN_ADDRESS));

    final var result =
        selector.evaluateTransactionPostProcessing(mockContext(), mock(TransactionProcessingResult.class));

    assertThat(result).isEqualTo(SELECTED);
  }

  @Test
  void postProcessingRejectsWhenCalledAddressIsDenied() {
    deniedAddresses.set(Set.of(DENIED_ADDRESS));
    tracer.traceStartTransaction(null, null);
    tracer.traceContextEnter(mockFrame(CLEAN_ADDRESS));
    tracer.traceContextEnter(mockFrame(DENIED_ADDRESS));

    final var result =
        selector.evaluateTransactionPostProcessing(mockContext(), mock(TransactionProcessingResult.class));

    assertThat(result).isEqualTo(TX_FILTERED_ADDRESS_CALLED);
  }

  @Test
  void postProcessingReturnsSelectedWhenNoAddressesCalled() {
    deniedAddresses.set(Set.of(DENIED_ADDRESS));
    tracer.traceStartTransaction(null, null);

    final var result =
        selector.evaluateTransactionPostProcessing(mockContext(), mock(TransactionProcessingResult.class));

    assertThat(result).isEqualTo(SELECTED);
  }

  @Test
  void denylistUpdatesDynamically() {
    tracer.traceStartTransaction(null, null);
    tracer.traceContextEnter(mockFrame(DENIED_ADDRESS));

    // Initially not denied
    var result =
        selector.evaluateTransactionPostProcessing(mockContext(), mock(TransactionProcessingResult.class));
    assertThat(result).isEqualTo(SELECTED);

    // Update denylist
    deniedAddresses.set(Set.of(DENIED_ADDRESS));
    result =
        selector.evaluateTransactionPostProcessing(mockContext(), mock(TransactionProcessingResult.class));
    assertThat(result).isEqualTo(TX_FILTERED_ADDRESS_CALLED);

    // Clear denylist
    deniedAddresses.set(Set.of());
    result =
        selector.evaluateTransactionPostProcessing(mockContext(), mock(TransactionProcessingResult.class));
    assertThat(result).isEqualTo(SELECTED);
  }

  private TransactionEvaluationContext mockContext() {
    final Transaction transaction = mock(Transaction.class);
    when(transaction.getHash()).thenReturn(Hash.ZERO);

    final PendingTransaction pendingTransaction = mock(PendingTransaction.class);
    when(pendingTransaction.getTransaction()).thenReturn(transaction);

    final TransactionEvaluationContext context = mock(TransactionEvaluationContext.class);
    when(context.getPendingTransaction()).thenReturn(pendingTransaction);

    return context;
  }

  private MessageFrame mockFrame(final Address address) {
    final MessageFrame frame = mock(MessageFrame.class);
    when(frame.getRecipientAddress()).thenReturn(address);
    when(frame.getContractAddress()).thenReturn(address);
    return frame;
  }
}
