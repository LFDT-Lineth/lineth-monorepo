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

package net.consensys.linea.zktracer.module.hub.defer;

import java.util.*;

import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.runtime.callstack.CallFrame;
import net.consensys.linea.zktracer.runtime.callstack.CallStack;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.evm.frame.MessageFrame;
import org.hyperledger.besu.evm.operation.Operation;
import org.hyperledger.besu.evm.worldstate.WorldView;

/**
 * Stores different categories of actions whose execution must be deferred later in the normal
 * transaction execution process.
 */
public class DeferRegistry
    implements PostOpcodeDefer,
        ContextEntryDefer,
        ContextExitDefer,
        PostRollbackDefer,
        EndTransactionDefer,
        AfterTransactionFinalizationDefer,
        PostBlockDefer,
        PostConflationDefer {

  /** A list of actions deferred until the end of the current opcode execution */
  private final List<PostOpcodeDefer> postOpcodeDefers = new ArrayList<>();

  /** A list of actions deferred to the immediate entry into a child or parent context */
  private final List<ContextEntryDefer> contextEntryDefers = new ArrayList<>();

  /** A list of actions deferred to the end of a given context */
  private final Map<Integer, List<ContextExitDefer>> contextExitDefers = new HashMap<>();

  /** A list of actions deferred to the end of the current opcode execution */
  private final Map<CallFrame, List<ContextReEntryDefer>> contextReEntryDefers = new HashMap<>();

  /** A list of actions deferred to the end of the current transaction */
  private final List<EndTransactionDefer> endTransactionDefers = new ArrayList<>();

  /** A list of actions deferred to after the TX_FINL phase of the current transaction */
  private final List<AfterTransactionFinalizationDefer> afterTransactionFinalizationDefers =
      new ArrayList<>();

  /** A list of actions deferred until the end of the current block execution */
  private final List<PostBlockDefer> postBlockDefers = new ArrayList<>();

  /** A list of actions deferred until the end of the current conflation execution */
  private final List<PostConflationDefer> postConflationDefers = new ArrayList<>();

  /**
   * A collection of actions whose execution is deferred to a hypothetical future rollback. This
   * collection maps a context to all actions that would have to be done if that execution context
   * were to be rolled back.
   */
  private final Map<CallFrame, List<PostRollbackDefer>> rollbackDefers = new HashMap<>();

  /** Schedule an action to be executed after the completion of the current opcode. */
  public void scheduleForContextEntry(ContextEntryDefer defer) {
    contextEntryDefers.add(defer);
  }

  /** Schedule an action to be executed after the completion of the current opcode. */
  public void scheduleForPostExecution(PostOpcodeDefer defer) {
    postOpcodeDefers.add(defer);
  }

  /** Schedule an action to be executed at the exit of a given context. */
  public void scheduleForContextExit(ContextExitDefer defer, Integer callFrameId) {
    if (!contextExitDefers.containsKey(callFrameId)) {
      contextExitDefers.put(callFrameId, new ArrayList<>());
    }
    contextExitDefers.get(callFrameId).add(defer);
  }

  /** Schedule an action to be executed at the end of the current transaction. */
  public void scheduleForEndTransaction(EndTransactionDefer defer) {
    endTransactionDefers.add(defer);
  }

  /** Schedule an action to be executed after TX_FINL. */
  public void scheduleForAfterTransactionFinalization(AfterTransactionFinalizationDefer defer) {
    afterTransactionFinalizationDefers.add(defer);
  }

  /** Schedule an action to be executed at the end of the current block. */
  public void scheduleForPostBlock(PostBlockDefer defer) {
    postBlockDefers.add(defer);
  }

  /** Schedule an action to be executed at the end of the current conflation. */
  public void scheduleForPostConflation(PostConflationDefer defer) {
    postConflationDefers.add(defer);
  }

  /** Schedule an action to be executed at the re-entry in the current context. */
  public void scheduleForContextReEntry(ContextReEntryDefer defer, CallFrame callFrame) {
    if (!contextReEntryDefers.containsKey(callFrame)) {
      contextReEntryDefers.put(callFrame, new ArrayList<>());
    }
    contextReEntryDefers.get(callFrame).add(defer);
  }

  public void scheduleForPostRollback(PostRollbackDefer defer, CallFrame callFrame) {
    if (!rollbackDefers.containsKey(callFrame)) {
      rollbackDefers.put(callFrame, new ArrayList<>());
    }
    rollbackDefers.get(callFrame).add(defer);
  }

  /**
   * Trigger the execution of the actions deferred to the end of the transaction.
   *
   * @param hub the {@link Hub} context
   * @param world a {@link WorldView} on the state
   * @param tx the current {@link Transaction}
   */
  @Override
  public void resolveAtEndTransaction(
      Hub hub, WorldView world, Transaction tx, boolean isSuccessful) {
    /**
     * AccountFragment could be created at resolveAtEndTransaction for some sections. As those
     * accountFragments are too endTransactionDeferred, we need to resolveAtEndTransaction in two
     * steps.
     */
    final List<EndTransactionDefer> endTransactionDefersFirstRound =
        new ArrayList<>(endTransactionDefers);
    endTransactionDefers.clear();
    for (EndTransactionDefer defer : endTransactionDefersFirstRound) {
      defer.resolveAtEndTransaction(hub, world, tx, isSuccessful);
    }
    for (EndTransactionDefer defer : endTransactionDefers) {
      defer.resolveAtEndTransaction(hub, world, tx, isSuccessful);
    }
    endTransactionDefers.clear();
  }

  public void resolveAfterTransactionFinalization(Hub hub, WorldView worldView) {
    for (AfterTransactionFinalizationDefer defer : afterTransactionFinalizationDefers) {
      defer.resolveAfterTransactionFinalization(hub, worldView);
    }
    afterTransactionFinalizationDefers.clear();
  }

  @Override
  public void resolvePostBlock(Hub hub) {
    for (PostBlockDefer defer : postBlockDefers) {
      defer.resolvePostBlock(hub);
    }
    postBlockDefers.clear();
  }

  /**
   * Trigger the execution of the actions deferred to the end of the conflation.
   *
   * @param hub the {@link Hub} context
   */
  @Override
  public void resolvePostConflation(Hub hub, WorldView world) {
    for (PostConflationDefer defer : postConflationDefers) {
      defer.resolvePostConflation(hub, world);
    }
    postConflationDefers.clear();
  }

  /**
   * Trigger the execution of the actions deferred to the end of the current instruction execution.
   *
   * @param hub the {@link Hub} context
   * @param frame the {@link MessageFrame} of the transaction
   * @param result the {@link Operation.OperationResult} of the transaction
   */
  @Override
  public void resolvePostExecution(Hub hub, MessageFrame frame, Operation.OperationResult result) {
    for (PostOpcodeDefer defer : postOpcodeDefers) {
      defer.resolvePostExecution(hub, frame, result);
    }
    postOpcodeDefers.clear();
  }

  /**
   * Should be invoked when precisely after a rollback was acted upon in terms of rolling back
   * modifications to WORLD STATE and ACCRUED STATE but the caller (or creator), if present, hasn't
   * resumed execution yet, and if there isn't one because this is the root context of the
   * transaction, we haven't entered the "transaction finalization phase."
   *
   * <p>Note that the "current messageFrame" is expected to STILL BE the messageFrame responsible
   * for the rollback.
   */
  @Override
  public void resolveUponRollback(
      final Hub hub, final MessageFrame messageFrame, CallFrame currentCallFrame) {

    Optional.ofNullable(hub.defers().rollbackDefers.get(currentCallFrame))
        .ifPresent(
            defers -> {
              defers.forEach(
                  defer -> defer.resolveUponRollback(hub, messageFrame, currentCallFrame));
              defers.clear();
            });

    // recursively roll back child call frames
    final CallStack callStack = hub.callStack();
    currentCallFrame.childFrameIds().stream()
        .map(callStack::getById)
        .forEach(childCallFrame -> resolveUponRollback(hub, messageFrame, childCallFrame));
  }

  @Override
  public void resolveUponContextEntry(Hub hub, MessageFrame frame) {
    for (ContextEntryDefer defer : contextEntryDefers) {
      defer.resolveUponContextEntry(hub, frame);
    }
    contextEntryDefers.clear();
  }

  @Override
  public void resolveUponContextExit(Hub hub, CallFrame callFrame) {
    final Integer frameId = callFrame.id();
    if (contextExitDefers.containsKey(frameId)) {
      for (ContextExitDefer defers : contextExitDefers.get(frameId)) {
        defers.resolveUponContextExit(hub, callFrame);
      }
      contextExitDefers.remove(frameId);
    }
  }

  /**
   * Trigger the execution of the actions deferred to the re-entry in the current context.
   *
   * @param hub the {@link Hub} context
   * @param callFrame the {@link CallFrame} of the transaction
   */
  public void resolveUponContextReEntry(Hub hub, CallFrame callFrame) {
    if (contextReEntryDefers.containsKey(callFrame)) {
      for (ContextReEntryDefer defer : contextReEntryDefers.get(callFrame)) {
        defer.resolveAtContextReEntry(hub, callFrame);
      }
      contextReEntryDefers.remove(callFrame);
    }
  }

  public void unscheduleForContextReEntry(ContextReEntryDefer defer, CallFrame callFrame) {
    contextReEntryDefers.get(callFrame).remove(defer);
  }

  public void unscheduleForPostTransaction(EndTransactionDefer defer) {
    endTransactionDefers.remove(defer);
  }
}
