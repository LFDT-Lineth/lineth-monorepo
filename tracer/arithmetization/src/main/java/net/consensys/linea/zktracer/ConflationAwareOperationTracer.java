/*
 * Copyright Consensys Software Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with
 * the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on
 * an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package net.consensys.linea.zktracer;

import java.util.Arrays;
import java.util.List;
import java.util.Optional;
import java.util.Set;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Log;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.evm.frame.ExceptionalHaltReason;
import org.hyperledger.besu.evm.frame.MessageFrame;
import org.hyperledger.besu.evm.operation.Operation;
import org.hyperledger.besu.evm.worldstate.WorldView;
import org.hyperledger.besu.plugin.data.BlockBody;
import org.hyperledger.besu.plugin.data.BlockHeader;
import org.hyperledger.besu.plugin.data.ProcessableBlockHeader;
import org.hyperledger.besu.plugin.services.tracer.BlockAwareOperationTracer;

/**
 * An extended operation tracer that can trace the start and end of a number of blocks in
 * conflation.
 */
public interface ConflationAwareOperationTracer extends BlockAwareOperationTracer {

  /**
   * Trace the start of conflation for a number of blocks.
   *
   * @param numBlocksInConflation blocks in conflation
   */
  void traceStartConflation(final long numBlocksInConflation);

  /** Trace the end of conflation for a number of blocks. */
  void traceEndConflation(final WorldView state);

  /**
   * Construct a (conflation aware) operation tracer which multiplexes one or more existing tracers.
   * The order in which calls are made to the multiplexed tracers follow the order in which they
   * occur within the array.
   *
   * @param tracers The array of tracers to be multiplexed.
   * @return A operation tracer which calls the relevant methods on each of the tracers it is
   *     multiplexing.
   */
  static ConflationAwareOperationTracer sequence(ConflationAwareOperationTracer... tracers) {
    return new SequencingOperationTracer(tracers);
  }

  class SequencingOperationTracer implements ConflationAwareOperationTracer {
    private final List<ConflationAwareOperationTracer> tracers;

    public SequencingOperationTracer(ConflationAwareOperationTracer... tracers) {
      this.tracers = Arrays.asList(tracers);
    }

    @Override
    public void traceStartConflation(long numBlocksInConflation) {
      this.tracers.forEach(tracer -> tracer.traceStartConflation(numBlocksInConflation));
    }

    @Override
    public void traceEndConflation(WorldView state) {
      this.tracers.forEach(tracer -> tracer.traceEndConflation(state));
    }

    public void traceStartBlock(
        final WorldView worldView,
        final BlockHeader blockHeader,
        final BlockBody blockBody,
        final Address miningBeneficiary) {
      this.tracers.forEach(
          tracer -> tracer.traceStartBlock(worldView, blockHeader, blockBody, miningBeneficiary));
    }

    public void traceStartBlock(
        final WorldView worldView,
        final ProcessableBlockHeader processableBlockHeader,
        final Address miningBeneficiary) {
      this.tracers.forEach(
          tracer -> tracer.traceStartBlock(worldView, processableBlockHeader, miningBeneficiary));
    }

    public void traceEndBlock(final BlockHeader blockHeader, final BlockBody blockBody) {
      this.tracers.forEach(tracer -> tracer.traceEndBlock(blockHeader, blockBody));
    }

    @Override
    public void tracePreExecution(final MessageFrame frame) {
      this.tracers.forEach(tracer -> tracer.tracePreExecution(frame));
    }

    @Override
    public void tracePostExecution(
        final MessageFrame frame, final Operation.OperationResult operationResult) {
      this.tracers.forEach(tracer -> tracer.tracePostExecution(frame, operationResult));
    }

    @Override
    public void tracePrecompileCall(
        final MessageFrame frame, final long gasRequirement, final Bytes output) {
      this.tracers.forEach(tracer -> tracer.tracePrecompileCall(frame, gasRequirement, output));
    }

    @Override
    public void traceAccountCreationResult(
        final MessageFrame frame, final Optional<ExceptionalHaltReason> haltReason) {
      this.tracers.forEach(tracer -> tracer.traceAccountCreationResult(frame, haltReason));
    }

    @Override
    public void tracePrepareTransaction(final WorldView worldView, final Transaction transaction) {
      this.tracers.forEach(tracer -> tracer.tracePrepareTransaction(worldView, transaction));
    }

    @Override
    public void traceStartTransaction(final WorldView worldView, final Transaction transaction) {
      this.tracers.forEach(tracer -> tracer.traceStartTransaction(worldView, transaction));
    }

    @Override
    public void traceBeforeRewardTransaction(
        final WorldView worldView, final Transaction tx, final Wei miningReward) {
      this.tracers.forEach(
          tracer -> tracer.traceBeforeRewardTransaction(worldView, tx, miningReward));
    }

    @Override
    public void traceEndTransaction(
        WorldView world,
        Transaction tx,
        boolean status,
        Bytes output,
        List<Log> logs,
        long gasUsed,
        Set<Address> selfDestructs,
        long timeNs) {
      this.tracers.forEach(
          tracer ->
              tracer.traceEndTransaction(
                  world, tx, status, output, logs, gasUsed, selfDestructs, timeNs));
    }

    public void traceContextEnter(final MessageFrame frame) {
      this.tracers.forEach(tracer -> tracer.traceContextEnter(frame));
    }

    public void traceContextReEnter(final MessageFrame frame) {
      this.tracers.forEach(tracer -> tracer.traceContextReEnter(frame));
    }

    public void traceContextExit(final MessageFrame frame) {
      this.tracers.forEach(tracer -> tracer.traceContextExit(frame));
    }
  }
}
