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

package net.consensys.linea.zktracer.runtime.stack;

import static com.google.common.base.Preconditions.checkArgument;
import static net.consensys.linea.zktracer.opcode.OpCode.EXTCODECOPY;

import lombok.Getter;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.transients.StackHeightCheck;
import net.consensys.linea.zktracer.opcode.OpCode;
import net.consensys.linea.zktracer.opcode.OpCodeData;
import net.consensys.linea.zktracer.runtime.callstack.CallFrame;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.evm.frame.MessageFrame;

public class Stack {
  public static final short MAX_STACK_SIZE = 1024;
  public static final byte NONE = 0;
  public static final byte PUSH = 1;
  public static final byte POP = 2;

  @Getter short height;
  @Getter short heightNew;
  @Getter OpCodeData currentOpcodeData;
  Status status;
  int stamp;

  public Stack() {
    height = 0;
    heightNew = 0;
    status = Status.NORMAL;
  }

  public Stack snapshot() {
    var snapshot = new Stack();
    snapshot.height = this.height;
    snapshot.heightNew = this.heightNew;
    snapshot.currentOpcodeData = this.currentOpcodeData;
    snapshot.status = this.status;

    return snapshot;
  }

  private int stackStampWithOffset(int offset) {
    return stamp + offset;
  }

  private Bytes getStack(MessageFrame frame, int i) {
    return frame.getStackItem(i);
  }

  private void oneZero(MessageFrame frame, StackContext pending) {
    Bytes val = getStack(frame, 0);
    pending.addLine(
        new IndexedStackOperation(1, StackItem.pop(height, val, stackStampWithOffset(0))));
  }

  private void twoZero(MessageFrame frame, StackContext pending) {
    Bytes val1 = getStack(frame, 0);
    Bytes val2 = getStack(frame, 1);

    pending.addLine(
        new IndexedStackOperation(1, StackItem.pop(height, val1, stackStampWithOffset(0))),
        new IndexedStackOperation(
            2, StackItem.pop((short) (height - 1), val2, stackStampWithOffset(1))));
  }

  private void threeZero(MessageFrame frame, StackContext pending) {
    Bytes val1 = getStack(frame, 0);
    Bytes val2 = getStack(frame, 1);
    Bytes val3 = getStack(frame, 2);

    pending.addLine(
        new IndexedStackOperation(1, StackItem.pop(height, val1, stackStampWithOffset(0))),
        new IndexedStackOperation(
            2, StackItem.pop((short) (height - 1), val2, stackStampWithOffset(1))),
        new IndexedStackOperation(
            3, StackItem.pop((short) (height - 2), val3, stackStampWithOffset(2))));
  }

  private void zeroOne(MessageFrame ignoredFrame, StackContext pending) {
    pending.addArmingLine(
        new IndexedStackOperation(
            4, StackItem.push((short) (height + 1), stackStampWithOffset(0))));
  }

  private void oneOne(MessageFrame frame, StackContext pending) {
    Bytes val = getStack(frame, 0);

    pending.addArmingLine(
        new IndexedStackOperation(1, StackItem.pop(height, val, stackStampWithOffset(0))),
        new IndexedStackOperation(4, StackItem.push(height, stackStampWithOffset(1))));
  }

  private void twoOne(MessageFrame frame, StackContext pending) {
    Bytes val1 = getStack(frame, 0);
    Bytes val2 = getStack(frame, 1);

    pending.addArmingLine(
        new IndexedStackOperation(1, StackItem.pop(height, val1, stackStampWithOffset(0))),
        new IndexedStackOperation(
            2, StackItem.pop((short) (height - 1), val2, stackStampWithOffset(1))),
        new IndexedStackOperation(
            4, StackItem.push((short) (height - 1), stackStampWithOffset(2))));
  }

  private void threeOne(MessageFrame frame, StackContext pending) {
    Bytes val1 = getStack(frame, 0);
    Bytes val2 = getStack(frame, 1);
    Bytes val3 = getStack(frame, 2);

    pending.addArmingLine(
        new IndexedStackOperation(1, StackItem.pop(height, val1, stackStampWithOffset(0))),
        new IndexedStackOperation(
            2, StackItem.pop((short) (height - 1), val2, stackStampWithOffset(1))),
        new IndexedStackOperation(
            3, StackItem.pop((short) (height - 2), val3, stackStampWithOffset(2))),
        new IndexedStackOperation(
            4, StackItem.push((short) (height - 2), stackStampWithOffset(3))));
  }

  private void loadStore(MessageFrame frame, StackContext pending) {
    if (currentOpcodeData
        .mnemonic()
        .isAnyOf(OpCode.MSTORE, OpCode.MSTORE8, OpCode.SSTORE, OpCode.TSTORE)) {
      Bytes val1 = getStack(frame, 0);
      Bytes val2 = getStack(frame, 1);

      pending.addLine(
          new IndexedStackOperation(1, StackItem.pop(height, val1, stackStampWithOffset(0))),
          new IndexedStackOperation(
              4, StackItem.pop((short) (height - 1), val2, stackStampWithOffset(1))));
    } else {
      Bytes val = getStack(frame, 0);

      pending.addArmingLine(
          new IndexedStackOperation(1, StackItem.pop(height, val, stackStampWithOffset(0))),
          new IndexedStackOperation(4, StackItem.push(height, stackStampWithOffset(1))));
    }
  }

  private void dup(MessageFrame frame, StackContext pending) {
    int depth = currentOpcodeData.stackSettings().delta() - 1;
    Bytes val = getStack(frame, depth);

    pending.addLine(
        new IndexedStackOperation(
            1, StackItem.pop((short) (height - depth), val, stackStampWithOffset(0))),
        new IndexedStackOperation(
            2, StackItem.pushImmediate((short) (height - depth), val, stackStampWithOffset(1))),
        new IndexedStackOperation(
            4, StackItem.pushImmediate((short) (height + 1), val, stackStampWithOffset(2))));
  }

  private void swap(MessageFrame frame, StackContext pending) {
    int depth = currentOpcodeData.stackSettings().delta() - 1;
    Bytes topValue = getStack(frame, 0);
    Bytes botValue = getStack(frame, depth);

    pending.addLine(
        new IndexedStackOperation(
            1, StackItem.pop((short) (height - depth), botValue, stackStampWithOffset(0))),
        new IndexedStackOperation(2, StackItem.pop(height, topValue, stackStampWithOffset(1))),
        new IndexedStackOperation(
            3,
            StackItem.pushImmediate((short) (height - depth), topValue, stackStampWithOffset(2))),
        new IndexedStackOperation(
            4, StackItem.pushImmediate(height, botValue, stackStampWithOffset(3))));
  }

  private void log(MessageFrame frame, StackContext pending) {
    Bytes offset = getStack(frame, 0);
    Bytes size = getStack(frame, 1);

    // Stack line 1
    pending.addLine(
        new IndexedStackOperation(1, StackItem.pop(height, offset, stackStampWithOffset(0))),
        new IndexedStackOperation(
            2, StackItem.pop((short) (height - 1), size, stackStampWithOffset(1))));

    // Stack line 2
    IndexedStackOperation[] line2 = new IndexedStackOperation[] {};
    switch (currentOpcodeData.mnemonic()) {
      case LOG0 -> {}
      case LOG1 -> {
        Bytes topic1 = getStack(frame, 2);

        line2 =
            new IndexedStackOperation[] {
              new IndexedStackOperation(
                  1, StackItem.pop((short) (height - 2), topic1, stackStampWithOffset(2))),
            };
      }
      case LOG2 -> {
        Bytes topic1 = getStack(frame, 2);
        Bytes topic2 = getStack(frame, 3);

        line2 =
            new IndexedStackOperation[] {
              new IndexedStackOperation(
                  1, StackItem.pop((short) (height - 2), topic1, stackStampWithOffset(2))),
              new IndexedStackOperation(
                  2, StackItem.pop((short) (height - 3), topic2, stackStampWithOffset(3))),
            };
      }
      case LOG3 -> {
        Bytes topic1 = getStack(frame, 2);
        Bytes topic2 = getStack(frame, 3);
        Bytes topic3 = getStack(frame, 4);

        line2 =
            new IndexedStackOperation[] {
              new IndexedStackOperation(
                  1, StackItem.pop((short) (height - 2), topic1, stackStampWithOffset(2))),
              new IndexedStackOperation(
                  2, StackItem.pop((short) (height - 3), topic2, stackStampWithOffset(3))),
              new IndexedStackOperation(
                  3, StackItem.pop((short) (height - 4), topic3, stackStampWithOffset(4))),
            };
      }
      case LOG4 -> {
        Bytes topic1 = getStack(frame, 2);
        Bytes topic2 = getStack(frame, 3);
        Bytes topic3 = getStack(frame, 4);
        Bytes topic4 = getStack(frame, 5);

        line2 =
            new IndexedStackOperation[] {
              new IndexedStackOperation(
                  1, StackItem.pop((short) (height - 2), topic1, stackStampWithOffset(2))),
              new IndexedStackOperation(
                  2, StackItem.pop((short) (height - 3), topic2, stackStampWithOffset(3))),
              new IndexedStackOperation(
                  3, StackItem.pop((short) (height - 4), topic3, stackStampWithOffset(4))),
              new IndexedStackOperation(
                  4, StackItem.pop((short) (height - 5), topic4, stackStampWithOffset(5))),
            };
      }
      default -> throw new RuntimeException("not a LOGx");
    }
    pending.addLine(line2);
  }

  private void copy(MessageFrame frame, StackContext pending, OpCode mnemonic) {
    if (mnemonic == EXTCODECOPY) {
      Bytes val0 = getStack(frame, 0);
      Bytes val1 = getStack(frame, 1);
      Bytes val2 = getStack(frame, 2);
      Bytes val3 = getStack(frame, 3);

      pending.addLine(
          new IndexedStackOperation(
              1, StackItem.pop((short) (height - 1), val1, stackStampWithOffset(1))),
          new IndexedStackOperation(
              2, StackItem.pop((short) (height - 3), val3, stackStampWithOffset(2))),
          new IndexedStackOperation(
              3, StackItem.pop((short) (height - 2), val2, stackStampWithOffset(3))),
          new IndexedStackOperation(4, StackItem.pop(height, val0, stamp)));

    } else {
      // this is the CALLDATACOPY, CODECOPY and RETURNDATACOPY case
      Bytes val1 = getStack(frame, 0);
      Bytes val2 = getStack(frame, 2);
      Bytes val3 = getStack(frame, 1);

      pending.addLine(
          new IndexedStackOperation(1, StackItem.pop(height, val1, stackStampWithOffset(1))),
          new IndexedStackOperation(
              2, StackItem.pop((short) (height - 2), val2, stackStampWithOffset(2))),
          new IndexedStackOperation(
              3, StackItem.pop((short) (height - 1), val3, stackStampWithOffset(3))));
    }
  }

  private void call(MessageFrame frame, StackContext pending) {
    Bytes val1 = getStack(frame, 0);
    Bytes val2 = getStack(frame, 1);
    Bytes val3 = getStack(frame, 2);
    Bytes val4 = getStack(frame, 3);
    Bytes val5 = getStack(frame, 4);
    Bytes val6 = getStack(frame, 5);

    boolean callCanTransferValue = currentOpcodeData.callHasValueArgument();

    if (callCanTransferValue) {
      Bytes val7 = getStack(frame, 6);

      pending.addLine(
          new IndexedStackOperation(
              1, StackItem.pop((short) (height - 3), val4, stackStampWithOffset(3))),
          new IndexedStackOperation(
              2, StackItem.pop((short) (height - 4), val5, stackStampWithOffset(4))),
          new IndexedStackOperation(
              3, StackItem.pop((short) (height - 5), val6, stackStampWithOffset(5))),
          new IndexedStackOperation(
              4, StackItem.pop((short) (height - 6), val7, stackStampWithOffset(6))));
      pending.addArmingLine(
          new IndexedStackOperation(1, StackItem.pop(height, val1, stackStampWithOffset(0))),
          new IndexedStackOperation(
              2, StackItem.pop((short) (height - 1), val2, stackStampWithOffset(1))),
          new IndexedStackOperation(
              3, StackItem.pop((short) (height - 2), val3, stackStampWithOffset(2))),
          new IndexedStackOperation(
              4, StackItem.push((short) (height - 6), stackStampWithOffset(7))));
    } else {

      pending.addLine(
          new IndexedStackOperation(
              1, StackItem.pop((short) (height - 2), val3, stackStampWithOffset(3))),
          new IndexedStackOperation(
              2, StackItem.pop((short) (height - 3), val4, stackStampWithOffset(4))),
          new IndexedStackOperation(
              3, StackItem.pop((short) (height - 4), val5, stackStampWithOffset(5))),
          new IndexedStackOperation(
              4, StackItem.pop((short) (height - 5), val6, stackStampWithOffset(6))));

      pending.addArmingLine(
          new IndexedStackOperation(1, StackItem.pop(height, val1, stackStampWithOffset(0))),
          new IndexedStackOperation(
              2, StackItem.pop((short) (height - 1), val2, stackStampWithOffset(1))),
          new IndexedStackOperation(
              4, StackItem.push((short) (height - 5), stackStampWithOffset(7))));
    }
  }

  private void create(MessageFrame frame, StackContext pending) {
    final Bytes offset = getStack(frame, 1);
    final Bytes size = getStack(frame, 2);
    final Bytes value = getStack(frame, 0);

    pending.addLine(
        new IndexedStackOperation(
            1, StackItem.pop((short) (height - 1), offset, stackStampWithOffset(1))),
        new IndexedStackOperation(
            2, StackItem.pop((short) (height - 2), size, stackStampWithOffset(2))));

    if (currentOpcodeData.stackSettings().flag2()) {
      // case CREATE2
      final Bytes salt = getStack(frame, 3);

      pending.addArmingLine(
          new IndexedStackOperation(
              2, StackItem.pop((short) (height - 3), salt, stackStampWithOffset(3))),
          new IndexedStackOperation(3, StackItem.pop(height, value, stackStampWithOffset(0))),
          new IndexedStackOperation(
              4, StackItem.push((short) (height - 3), stackStampWithOffset(4))));
    } else
    // case CREATE
    {
      pending.addArmingLine(
          new IndexedStackOperation(3, StackItem.pop(height, value, stackStampWithOffset(0))),
          new IndexedStackOperation(
              4, StackItem.push((short) (height - 2), stackStampWithOffset(4))));
    }
  }

  /**
   * @return true if no stack exception has been raised
   */
  public boolean isOk() {
    return status == Status.NORMAL;
  }

  /**
   * @return true if a stack underflow exception has been raised
   */
  public boolean isUnderflow() {
    return status == Status.UNDERFLOW;
  }

  /**
   * @return true if a stack underflow exception has been raised
   */
  public boolean isOverflow() {
    return status == Status.OVERFLOW;
  }

  public void processInstruction(final Hub hub, MessageFrame frame, int stackStamp) {
    final CallFrame callFrame = hub.currentFrame();
    stamp = stackStamp;
    currentOpcodeData = hub.opCodeData();
    callFrame.pending(new StackContext(currentOpcodeData));

    final short delta = (short) currentOpcodeData.stackSettings().delta();
    final short alpha = (short) currentOpcodeData.stackSettings().alpha();

    checkArgument(
        heightNew == frame.stackSize(), "stack height prediction and frame value mismatch");
    height = (short) frame.stackSize();
    heightNew -= delta;
    heightNew += alpha;

    if (frame.stackSize() < delta) { // Testing for underflow
      status = Status.UNDERFLOW;
    } else if (heightNew > MAX_STACK_SIZE) { // Testing for overflow
      status = Status.OVERFLOW;
    }

    // stack underflow checks happen for every opcode
    final StackHeightCheck checkForUnderflow = new StackHeightCheck(height, delta);
    final boolean isNewCheckForStackUnderflow =
        hub.transients().conflation().stackHeightChecksForStackUnderflows().add(checkForUnderflow);
    if (isNewCheckForStackUnderflow) {
      final boolean underflowDetected = hub.wcp().callLT(height, delta);
      checkArgument(
          underflowDetected == (status == Status.UNDERFLOW), "stack underflow detection mismatch");
    }

    // stack overflow checks happen only if no stack underflow was detected
    if (!this.isUnderflow()) {
      final StackHeightCheck checkForOverflow = new StackHeightCheck(heightNew);
      final boolean isNewCheckForStackOverflow =
          hub.transients().conflation().stackHeightChecksForStackOverflows().add(checkForOverflow);
      if (isNewCheckForStackOverflow) {
        final boolean overflowDetected = hub.wcp().callGT(heightNew, MAX_STACK_SIZE);
        checkArgument(
            overflowDetected == (status == Status.OVERFLOW), "stack overflow detection mismatch");
      }
    }

    if (status.isFailure()) {
      heightNew = 0;
      final int numberOfStackRows = currentOpcodeData.numberOfStackRows();
      callFrame.pending().addEmptyLines(numberOfStackRows);

      return;
    }

    switch (currentOpcodeData.stackSettings().pattern()) {
      case ZERO_ZERO -> stamp += callFrame.pending().addEmptyLines(1);
      case ONE_ZERO -> this.oneZero(frame, callFrame.pending());
      case TWO_ZERO -> this.twoZero(frame, callFrame.pending());
      case THREE_ZERO -> this.threeZero(frame, callFrame.pending());
      case ZERO_ONE -> this.zeroOne(frame, callFrame.pending());
      case ONE_ONE -> this.oneOne(frame, callFrame.pending());
      case TWO_ONE -> this.twoOne(frame, callFrame.pending());
      case THREE_ONE -> this.threeOne(frame, callFrame.pending());
      case LOAD_STORE -> this.loadStore(frame, callFrame.pending());
      case DUP -> this.dup(frame, callFrame.pending());
      case SWAP -> this.swap(frame, callFrame.pending());
      case LOG -> this.log(frame, callFrame.pending());
      case COPY -> this.copy(frame, callFrame.pending(), currentOpcodeData.mnemonic());
      case CALL -> this.call(frame, callFrame.pending());
      case CREATE -> this.create(frame, callFrame.pending());
    }
  }
}
