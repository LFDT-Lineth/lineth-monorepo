/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.sequencer.txselection.selectors;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.evm.frame.MessageFrame;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

class DenylistOperationTracerTest {

  private static final Address ADDRESS_1 =
      Address.fromHexString("0x1111111111111111111111111111111111111111");
  private static final Address ADDRESS_2 =
      Address.fromHexString("0x2222222222222222222222222222222222222222");
  private static final Address ADDRESS_3 =
      Address.fromHexString("0x3333333333333333333333333333333333333333");

  private DenylistOperationTracer tracer;

  @BeforeEach
  void setUp() {
    tracer = new DenylistOperationTracer();
  }

  @Test
  void collectsRecipientAddressOnContextEnter() {
    final MessageFrame frame = mockFrame(ADDRESS_1, ADDRESS_1);

    tracer.traceContextEnter(frame);

    assertThat(tracer.getCalledAddresses()).containsExactly(ADDRESS_1);
  }

  @Test
  void collectsContractAddressForDelegateCall() {
    // For DELEGATECALL/CALLCODE, recipient is the caller but contract address is the target
    final MessageFrame frame = mockFrame(ADDRESS_1, ADDRESS_2);

    tracer.traceContextEnter(frame);

    assertThat(tracer.getCalledAddresses()).containsExactlyInAnyOrder(ADDRESS_1, ADDRESS_2);
  }

  @Test
  void clearsAddressesOnNewTransaction() {
    tracer.traceContextEnter(mockFrame(ADDRESS_1, ADDRESS_1));
    assertThat(tracer.getCalledAddresses()).isNotEmpty();

    tracer.traceStartTransaction(null, null);

    assertThat(tracer.getCalledAddresses()).isEmpty();
  }

  @Test
  void accumulatesMultipleAddresses() {
    tracer.traceContextEnter(mockFrame(ADDRESS_1, ADDRESS_1));
    tracer.traceContextEnter(mockFrame(ADDRESS_2, ADDRESS_2));
    tracer.traceContextEnter(mockFrame(ADDRESS_3, ADDRESS_3));

    assertThat(tracer.getCalledAddresses())
        .containsExactlyInAnyOrder(ADDRESS_1, ADDRESS_2, ADDRESS_3);
  }

  @Test
  void deduplicatesSameAddress() {
    tracer.traceContextEnter(mockFrame(ADDRESS_1, ADDRESS_1));
    tracer.traceContextEnter(mockFrame(ADDRESS_1, ADDRESS_1));

    assertThat(tracer.getCalledAddresses()).containsExactly(ADDRESS_1);
  }

  @Test
  void addressesAvailableAfterTransactionProcessingCompletes() {
    tracer.traceStartTransaction(null, null);
    tracer.traceContextEnter(mockFrame(ADDRESS_1, ADDRESS_1));

    // Addresses must still be available after execution ends (for post-processing checks)
    assertThat(tracer.getCalledAddresses()).containsExactly(ADDRESS_1);
  }

  @Test
  void nextTransactionClearsAddressesFromPreviousTransaction() {
    tracer.traceContextEnter(mockFrame(ADDRESS_1, ADDRESS_1));
    assertThat(tracer.getCalledAddresses()).isNotEmpty();

    // Starting a new transaction clears the previous one's addresses
    tracer.traceStartTransaction(null, null);

    assertThat(tracer.getCalledAddresses()).isEmpty();
  }

  @Test
  void returnsUnmodifiableSet() {
    tracer.traceContextEnter(mockFrame(ADDRESS_1, ADDRESS_1));

    assertThatThrownBy(() -> tracer.getCalledAddresses().add(ADDRESS_2))
        .isInstanceOf(UnsupportedOperationException.class);
  }

  private MessageFrame mockFrame(final Address recipientAddress, final Address contractAddress) {
    final MessageFrame frame = mock(MessageFrame.class);
    when(frame.getRecipientAddress()).thenReturn(recipientAddress);
    when(frame.getContractAddress()).thenReturn(contractAddress);
    return frame;
  }
}
