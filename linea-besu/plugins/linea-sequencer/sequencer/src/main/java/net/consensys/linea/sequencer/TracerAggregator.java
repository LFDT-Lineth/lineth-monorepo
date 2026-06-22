/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.sequencer;

import java.util.ArrayList;
import java.util.List;
import java.util.Optional;
import java.util.Set;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Log;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.evm.frame.ExceptionalHaltReason;
import org.hyperledger.besu.evm.frame.MessageFrame;
import org.hyperledger.besu.evm.operation.Operation;
import org.hyperledger.besu.evm.tracing.OperationTracer;
import org.hyperledger.besu.evm.worldstate.WorldView;

/**
 * Aggregates multiple {@link OperationTracer} instances, allowing them to be treated as a single
 * tracer. This class facilitates the registration and delegation of tracing operations to multiple
 * tracers.
 */
public class TracerAggregator implements OperationTracer {
  private final List<OperationTracer> tracers = new ArrayList<>();

  /**
   * Registers an {@link OperationTracer} instance with the aggregator. If a tracer of the same
   * class is already registered, an {@link IllegalArgumentException} is thrown.
   *
   * @param tracer the tracer to register
   * @throws IllegalArgumentException if a tracer of the same class is already registered
   */
  public void register(OperationTracer tracer) {
    // Check if a tracer of the same class is already registered
    for (OperationTracer existingTracer : tracers) {
      if (existingTracer.getClass().equals(tracer.getClass())) {
        throw new IllegalArgumentException(
            "A tracer of class " + tracer.getClass().getName() + " is already registered.");
      }
    }
    tracers.add(tracer);
  }

  /**
   * Creates a {@link TracerAggregator} instance and registers the provided tracers.
   *
   * @param tracers the tracers to register with the aggregator
   * @return a new {@link TracerAggregator} instance with the provided tracers registered
   */
  public static TracerAggregator create(OperationTracer... tracers) {
    TracerAggregator aggregator = new TracerAggregator();
    for (OperationTracer tracer : tracers) {
      aggregator.register(tracer);
    }
    return aggregator;
  }

  @Override
  public void tracePreExecution(MessageFrame frame) {
    for (OperationTracer tracer : tracers) {
      tracer.tracePreExecution(frame);
    }
  }

  @Override
  public void tracePostExecution(MessageFrame frame, Operation.OperationResult operationResult) {
    for (OperationTracer tracer : tracers) {
      tracer.tracePostExecution(frame, operationResult);
    }
  }

  @Override
  public void tracePrecompileCall(MessageFrame frame, long gasRequirement, Bytes output) {
    for (OperationTracer tracer : tracers) {
      tracer.tracePrecompileCall(frame, gasRequirement, output);
    }
  }

  @Override
  public void traceAccountCreationResult(
      MessageFrame frame, Optional<ExceptionalHaltReason> haltReason) {
    for (OperationTracer tracer : tracers) {
      tracer.traceAccountCreationResult(frame, haltReason);
    }
  }

  @Override
  public void tracePrepareTransaction(WorldView worldView, Transaction transaction) {
    for (OperationTracer tracer : tracers) {
      tracer.tracePrepareTransaction(worldView, transaction);
    }
  }

  @Override
  public void traceStartTransaction(WorldView worldView, Transaction transaction) {
    for (OperationTracer tracer : tracers) {
      tracer.traceStartTransaction(worldView, transaction);
    }
  }

  @Override
  public void traceEndTransaction(
      WorldView worldView,
      Transaction tx,
      boolean status,
      Bytes output,
      List<Log> logs,
      long gasUsed,
      Set<Address> selfDestructs,
      long timeNs) {
    for (OperationTracer tracer : tracers) {
      tracer.traceEndTransaction(
          worldView, tx, status, output, logs, gasUsed, selfDestructs, timeNs);
    }
  }

  @Override
  public void traceContextEnter(MessageFrame frame) {
    for (OperationTracer tracer : tracers) {
      tracer.traceContextEnter(frame);
    }
  }

  @Override
  public void traceContextReEnter(MessageFrame frame) {
    for (OperationTracer tracer : tracers) {
      tracer.traceContextReEnter(frame);
    }
  }

  @Override
  public void traceContextExit(MessageFrame frame) {
    for (OperationTracer tracer : tracers) {
      tracer.traceContextExit(frame);
    }
  }

  @Override
  public boolean isExtendedTracing() {
    for (OperationTracer tracer : tracers) {
      if (tracer.isExtendedTracing()) {
        return true;
      }
    }
    return false;
  }
}
