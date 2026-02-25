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

package net.consensys.linea.zktracer.runtime.callstack;

import com.google.common.base.Preconditions;
import java.util.ArrayList;
import java.util.List;
import java.util.Optional;
import lombok.Getter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.types.Bytecode;
import net.consensys.linea.zktracer.types.MemoryRange;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.evm.Code;
import org.hyperledger.besu.evm.frame.MessageFrame;

/**
 * This class represents the call hierarchy of a transaction.
 *
 * <p>Although it is accessible in a stack-like manner, it is actually a tree, the stack access
 * representing the path from the latest leaf to the root context.
 */
@Accessors(fluent = true)
public final class CallStack {
  /** the maximal depth of the call stack (as defined by Ethereum) */
  static final int MAX_CALLSTACK_SIZE = 1024;

  /** a never-pruned-tree of the {@link CallFrame} executed by the {@link Hub} */
  @Getter
  private final List<CallFrame> callFrames =
      new ArrayList<>(
          50) { // TODO: PERF as the List of TraceSection, we should have an estimate based on
        // gasLimit on the nb of CallFrame a tx might have
        {
          add(CallFrame.EMPTY);
        }
      };

  /** the current depth of the call stack. */
  @Getter private int depth;

  /** a "pointer" to the currentId {@link CallFrame} in <code>frames</code>. */
  private int currentId;

  public void newRootContext(
      int contextNumber,
      Address from,
      Address to,
      Bytecode toCode,
      Wei value,
      long gas,
      int callDataContextNumber,
      Bytes callData,
      int accountDeploymentNumber,
      Address byteCodeAddress,
      int byteCodeDeploymentNumber,
      boolean byteCodeDeploymentStatus,
      int byteCodeDelegationNumber) {
    this.depth = -1;
    this.enter(
        CallFrameType.ROOT,
        contextNumber,
        byteCodeDeploymentStatus,
        value,
        gas,
        to,
        accountDeploymentNumber,
        byteCodeAddress,
        byteCodeDeploymentNumber,
        byteCodeDelegationNumber,
        toCode == null ? Bytecode.EMPTY : toCode,
        from,
        new MemoryRange(callDataContextNumber, 0, callData.size(), callData),
        new MemoryRange(0));
    this.currentId = this.callFrames.size() - 1;
  }

  /**
   * A “mantle” {@link CallFrame} holds the call data for a message call with a non-empty call data
   *
   * @param transactionCallDataContextNumber
   * @param callData
   */
  public void transactionCallDataContext(int transactionCallDataContextNumber, Bytes callData) {
    this.depth = -1;
    this.callFrames.add(new CallFrame(Address.ZERO, callData, transactionCallDataContextNumber));
    this.enter(
        CallFrameType.TRANSACTION_CALL_DATA_HOLDER,
        transactionCallDataContextNumber,
        false,
        Wei.ZERO,
        0,
        Address.ZERO, // useless
        0,
        Address.ZERO, // useless
        0,
        0,
        Bytecode.EMPTY,
        Address.ZERO, // useless
        new MemoryRange(0),
        new MemoryRange(0));
    this.currentId = this.callFrames.size() - 1;
  }

  /**
   * @return the currently executing {@link CallFrame}
   */
  public CallFrame currentCallFrame() {
    return this.callFrames.get(this.currentId);
  }

  public boolean isEmpty() {
    return this.callFrames.isEmpty();
  }

  public int futureId() {
    return this.callFrames.size();
  }

  /**
   * @return the parent {@link CallFrame} of the current frame
   */
  public CallFrame parentCallFrame() {
    if (this.currentCallFrame().parentId() != -1) {
      return this.callFrames.get(this.currentCallFrame().parentId());
    } else {
      return CallFrame.EMPTY;
    }
  }

  public Optional<CallFrame> maybeCurrent() {
    return this.callFrames.isEmpty() ? Optional.empty() : Optional.of(this.currentCallFrame());
  }

  /**
   * Creates a new call frame.
   *
   * @param type the execution type of call frame
   * @param newContextNumber the context number of the new (call) frame
   * @param isDeployment
   * @param value the value given to this call frame
   * @param gasStipend the gasStipend provided to this call frame
   * @param accountAddress the {@link Address} of the bytecode being executed
   * @param accountDeploymentNumber
   * @param byteCodeDeploymentNumber
   * @param byteCode the {@link Code} being executed
   */
  public void enter(
      CallFrameType type,
      int newContextNumber,
      boolean isDeployment,
      Wei value,
      long gasStipend,
      Address accountAddress,
      int accountDeploymentNumber,
      Address byteCodeAddress,
      int byteCodeDeploymentNumber,
      int byteCodeDelegationNumber,
      Bytecode byteCode,
      Address callerAddress,
      MemoryRange callData,
      MemoryRange returnAt) {
    final int callerId = this.depth == -1 ? -1 : this.currentId;
    final int newCallFrameId = this.callFrames.size();
    this.depth += 1;

    final CallFrame newFrame =
        new CallFrame(
            type,
            newCallFrameId,
            newContextNumber,
            this.depth,
            isDeployment,
            value,
            gasStipend,
            accountAddress,
            accountDeploymentNumber,
            byteCodeAddress,
            byteCodeDeploymentNumber,
            byteCodeDelegationNumber,
            byteCode,
            callerAddress,
            callerId,
            callData,
            returnAt);

    this.callFrames.add(newFrame);
    this.currentId = newCallFrameId;
    if (callerId != -1) {
      this.callFrames.get(callerId).childFrameIds().add(newCallFrameId);
    }
  }

  /**
   * Exit the current context, sets it return data for the caller to read, and marks its last
   * position in the hub traces.
   */
  public void exit() {
    this.depth -= 1;
    Preconditions.checkState(this.depth >= 0, "call stack underflow");
    this.currentId = this.currentCallFrame().parentId();
  }

  /**
   * @return whether the call stack is in an overflow state
   */
  public boolean isOverflow() {
    return this.depth > MAX_CALLSTACK_SIZE;
  }

  /**
   * @return whether the call stack is at its maximum capacity and a new frame would overflow it
   */
  public boolean wouldOverflow() {
    return this.depth >= MAX_CALLSTACK_SIZE;
  }

  /**
   * @return whether the current frame is a static context
   */
  public boolean isStatic() {
    return this.currentCallFrame().type() == CallFrameType.STATIC;
  }

  /**
   * Returns the ith {@link CallFrame} in this call stack.
   *
   * @param i ID of the call frame to fetch
   * @return the ith call frame
   * @throws IndexOutOfBoundsException if the index is out of range
   */
  public CallFrame getById(int i) {
    if (i < 0 || this.callFrames.isEmpty()) {
      return CallFrame.EMPTY;
    }
    return this.callFrames.get(i);
  }

  /**
   * Returns the {@link CallFrame} in this call stack of the given context number.
   *
   * @param i context number of the call frame to fetch
   * @return the call frame with the specifies
   * @throws IndexOutOfBoundsException if the index is out of range
   */
  public CallFrame getByContextNumber(final long i) {
    for (CallFrame f : this.callFrames) {
      if (f.contextNumber() == i) {
        return f;
      }
    }

    throw new IllegalArgumentException(String.format("call frame CN %s not found", i));
  }

  /**
   * Returns the parent of the ith {@link CallFrame} in this call stack.
   *
   * @param id ID of the call frame whose parent to fetch
   * @return the ith call frame parent
   * @throws IndexOutOfBoundsException if the index is out of range
   */
  public CallFrame getParentCallFrameById(int id) {
    if (this.callFrames.isEmpty()) {
      return CallFrame.EMPTY;
    }

    return this.getById(this.callFrames.get(id).parentId());
  }

  /**
   * Retrieves the context number of the parent {@link CallFrame} for a given call frame ID.
   *
   * @param id the ID of the call frame whose parent's context number is to be retrieved.
   * @return the context number of the parent call frame. If the call frame has no parent, or if the
   *     specified ID does not correspond to a valid call frame, this method returns the context
   *     number of the {@link CallFrame#EMPTY} which is typically 0.
   */
  public int getParentContextNumberById(int id) {
    return this.getParentCallFrameById(id).contextNumber();
  }

  public Bytes getFullMemoryOfCaller(Hub hub) {
    final MessageFrame parentFrame = parentCallFrame().frame();
    return currentCallFrame().depth() == 0
        ? hub.txStack().current().getTransactionCallData()
        : parentFrame.shadowReadMemory(0, parentFrame.memoryByteSize());
  }
}
