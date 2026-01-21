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

import static net.consensys.linea.zktracer.Trace.EVM_INST_STOP;

import com.google.common.base.Preconditions;
import java.util.ArrayList;
import java.util.List;
import lombok.Getter;
import lombok.Setter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.section.TraceSection;
import net.consensys.linea.zktracer.module.romlex.ContractMetadata;
import net.consensys.linea.zktracer.runtime.stack.Stack;
import net.consensys.linea.zktracer.runtime.stack.StackContext;
import net.consensys.linea.zktracer.types.Bytecode;
import net.consensys.linea.zktracer.types.MemoryRange;
import net.consensys.linea.zktracer.types.Range;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.evm.frame.MessageFrame;

@Accessors(fluent = true)
public class CallFrame {
  public static final CallFrame EMPTY = new CallFrame();

  // various identifiers related to the CallFrame
  @Getter private final int id;
  @Getter private final int contextNumber;
  @Getter private int parentId;
  @Getter private final List<Integer> childFrameIds = new ArrayList<>();

  // general information
  @Getter private final Wei value;
  @Getter private long gasStipend;
  @Getter private final int depth;
  @Getter private boolean isDeployment;
  @Getter private final CallFrameType type;
  @Getter private int delegationNumber;

  public boolean isMessageCall() {
    return !isDeployment;
  }

  // account whose storage and value are accessible
  @Getter private final Address accountAddress;
  @Getter private int accountDeploymentNumber;

  // byte code that is running in the present frame
  @Getter private Address byteCodeAddress = Address.ZERO;
  @Getter private int byteCodeDeploymentNumber;
  @Getter private Bytecode code = Bytecode.EMPTY;

  // caller related information
  @Getter private Address callerAddress = Address.ZERO;

  public int getCodeFragmentIndex(Hub hub) {
    return this == CallFrame.EMPTY || type == CallFrameType.TRANSACTION_CALL_DATA_HOLDER
        ? 0
        : hub.getCodeFragmentIndexByMetaData(
            byteCodeAddress, byteCodeDeploymentNumber, isDeployment, delegationNumber);
  }

  @Getter @Setter private int pc;
  @Getter @Setter private int opCode = EVM_INST_STOP;
  @Getter private MessageFrame frame; // TODO: can we make this final ?

  // various memory ranges
  @Getter private final MemoryRange callDataRange; // immutable
  @Getter private final MemoryRange returnAtRange; // immutable
  @Getter @Setter private MemoryRange returnDataRange = MemoryRange.EMPTY; // mutable
  @Getter @Setter private MemoryRange outputDataRange; // set at exit time

  @Getter private boolean executionPaused = false;
  @Getter @Setter private long lastValidGasNext = 0;

  public void pauseCurrentFrame() {
    Preconditions.checkState(!executionPaused, "cannot pause frame as frame already paused");
    executionPaused = true;
  }

  public void unpauseCurrentFrame() {
    Preconditions.checkState(executionPaused, "cannot unpause frame as frame is not paused");
    executionPaused = false;
  }

  public void rememberGasNextBeforePausing(Hub hub) {
    lastValidGasNext = hub.state.current().traceSections().currentSection().commonValues.gasNext();
  }

  // revert related information
  @Getter @Setter private boolean selfReverts = false;
  @Getter @Setter private boolean getsReverted = false;
  @Getter @Setter private int revertStamp = 0;

  /** this frame {@link Stack}. */
  @Getter private final Stack stack = new Stack();

  /** the latched context of this callframe stack. */
  @Getter @Setter private StackContext pending;

  /**
   * the section responsible for the creation of a child context, either a CALL or a CREATE
   * instruction
   */
  @Getter @Setter private TraceSection childSpanningSection;

  /** Create a MANTLE call frame. */
  CallFrame(final Address origin, final Bytes callDataRange, final int contextNumber) {
    type = CallFrameType.TRANSACTION_CALL_DATA_HOLDER;
    this.contextNumber = contextNumber;
    accountAddress = origin;
    this.callDataRange = new MemoryRange(contextNumber, 0, callDataRange.size(), callDataRange);
    this.returnAtRange = MemoryRange.EMPTY;
    value = Wei.ZERO;
    id = -1;
    depth = -1;
  }

  /** Create an empty call frame. */
  CallFrame() {
    type = CallFrameType.EMPTY;
    contextNumber = 0;
    accountAddress = Address.ZERO;
    parentId = -1;
    this.callDataRange = MemoryRange.EMPTY;
    this.returnAtRange = MemoryRange.EMPTY;
    depth = 0;
    value = Wei.ZERO;
    id = -1;
  }

  /**
   * Create a non-root call frame. Below we abbreviate Context Number to CN
   *
   * @param type the {@link CallFrameType} of this frame
   * @param id ID of this frame in the {@link CallStack}
   * @param contextNumber of this call frame
   * @param depth call stack depth of the current execution context
   * @param isDeployment whether the executing byteCode is initcode
   * @param value how much ether was given to this frame
   * @param gasStipend how much gasStipend was given to this frame
   * @param accountAddress {@link Address} of this frame executor
   * @param accountDeploymentNumber DN of the account address
   * @param byteCodeAddress address whose byteCode executes in the present frame
   * @param byteCodeDeploymentNumber DN of this call frame in the {@link Hub}
   * @param byteCode byteCode that executes in the present context
   * @param callerAddress either account address of the caller/creator context
   * @param parentId ID of the caller frame in the {@link CallStack}
   * @param callDataRange call data of the current frame
   */
  CallFrame(
      CallFrameType type,
      int id,
      int contextNumber,
      int depth,
      boolean isDeployment,
      Wei value,
      long gasStipend,
      Address accountAddress,
      int accountDeploymentNumber,
      Address byteCodeAddress,
      int byteCodeDeploymentNumber,
      Bytecode byteCode,
      Address callerAddress,
      int parentId,
      MemoryRange callDataRange,
      MemoryRange returnAtRange) {
    this.type = type;
    this.id = id;
    this.contextNumber = contextNumber;
    this.isDeployment = isDeployment;
    this.value = value;
    this.gasStipend = gasStipend;
    this.depth = depth;
    this.accountAddress = accountAddress;
    this.accountDeploymentNumber = accountDeploymentNumber;
    this.byteCodeAddress = byteCodeAddress;
    this.byteCodeDeploymentNumber = byteCodeDeploymentNumber;
    this.code = byteCode;
    this.callerAddress = callerAddress;
    this.parentId = parentId;
    this.callDataRange = callDataRange;
    this.returnAtRange = returnAtRange;
    this.outputDataRange = new MemoryRange(contextNumber);
  }

  public boolean isRoot() {
    return this.depth == 0;
  }

  /**
   * Returns a {@link ContractMetadata} instance representing the executed contract.
   *
   * @return the executed contract metadata
   */
  public ContractMetadata metadata() {
    return ContractMetadata.make(
        byteCodeAddress, byteCodeDeploymentNumber, isDeployment, delegationNumber);
  }

  private void revertChildren(CallStack callStack, int parentRevertStamp) {
    childFrameIds.stream()
        .map(callStack::getById)
        .forEach(
            frame -> {
              frame.getsReverted = true;
              if (!frame.selfReverts) {
                if (frame.revertStamp == 0) {
                  frame.revertStamp = parentRevertStamp;
                } else if (frame.revertStamp > parentRevertStamp) {
                  frame.revertStamp = parentRevertStamp;
                }
              }
              frame.revertChildren(callStack, parentRevertStamp);
            });
  }

  public void setRevertStamps(CallStack callStack, int currentStamp) {
    if (selfReverts) {
      throw new IllegalStateException(
          String.format(
              "a context can not self-revert twice, it already reverts at %s, can't revert again at %s",
              revertStamp, currentStamp));
    }
    selfReverts = true;
    revertStamp = currentStamp;
    this.revertChildren(callStack, revertStamp);
  }

  /** Return true if this call frame is reverted (self reverted or get reverted) */
  public boolean willRevert() {
    return selfReverts() || getsReverted();
  }

  public boolean wontRevert() {
    return !willRevert();
  }

  public void initializeFrame(final MessageFrame frame) {
    this.frame = frame;
  }

  public void frame(MessageFrame frame) {
    this.frame = frame;
    opCode = frame.getCurrentOperation().getOpcode();
    pc = frame.getPC();
  }

  public static Bytes extractContiguousLimbsFromMemory(
      final MessageFrame frame, final Range range) {
    // TODO: optimize me please. Need a review of the MMU operation handling.
    return range.isEmpty() ? Bytes.EMPTY : frame.shadowReadMemory(0, frame.memoryByteSize());
  }
}
