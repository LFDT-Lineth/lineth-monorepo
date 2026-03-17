/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.sequencer.txselection.selectors;

import java.util.Collections;
import java.util.HashSet;
import java.util.Set;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.evm.frame.MessageFrame;
import org.hyperledger.besu.evm.tracing.OperationTracer;
import org.hyperledger.besu.evm.worldstate.WorldView;

/**
 * Operation tracer that collects all recipient addresses entered during EVM execution. Used by
 * {@link DenylistExecutionSelector} to check called addresses against the denylist after
 * transaction processing.
 */
public class DenylistOperationTracer implements OperationTracer {

  private final ThreadLocal<Set<Address>> calledAddresses = ThreadLocal.withInitial(HashSet::new);

  @Override
  public void traceStartTransaction(final WorldView worldView, final Transaction transaction) {
    calledAddresses.remove();
  }

  @Override
  public void traceContextEnter(final MessageFrame frame) {
    final Set<Address> addresses = calledAddresses.get();
    addresses.add(frame.getRecipientAddress());
    addresses.add(frame.getContractAddress());
  }

  public Set<Address> getCalledAddresses() {
    return Collections.unmodifiableSet(calledAddresses.get());
  }
}
