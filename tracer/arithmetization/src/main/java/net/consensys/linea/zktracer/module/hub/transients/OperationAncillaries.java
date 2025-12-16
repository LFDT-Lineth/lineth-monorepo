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

package net.consensys.linea.zktracer.module.hub.transients;

import static com.google.common.base.Preconditions.*;

import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.opcode.OpCodeData;
import net.consensys.linea.zktracer.types.Range;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.evm.frame.MessageFrame;
import org.hyperledger.besu.evm.internal.Words;

/** This class provides facilities to access data that are opcode-lived. */
@Slf4j
@RequiredArgsConstructor
public class OperationAncillaries {
  private final Hub hub;

  private static Bytes maybeShadowReadMemory(final Range span, final MessageFrame frame) {
    // Accesses to huge offset with 0-size are valid
    if (span.isEmpty()) {
      return Bytes.EMPTY;
    }

    // Besu is limited to i32 for memory offset/size
    if (span.besuOverflow()) {
      log.warn("Overflowing memory access: {}", span);
      return Bytes.EMPTY;
    }

    return frame.shadowReadMemory(span.offset(), span.size());
  }

  /**
   * Returns the RAM segment of the caller containing the calldata if the {@link MessageFrame}
   * operation is a call, throws otherwise.
   *
   * @param frame the execution context
   * @return the input data segment
   */
  public static Range callDataSegment(final MessageFrame frame, OpCodeData opCode) {
    switch (opCode.mnemonic()) {
      case CALL, CALLCODE -> {
        long offset = Words.clampedToLong(frame.getStackItem(3));
        long length = Words.clampedToLong(frame.getStackItem(4));
        return Range.fromOffsetAndSize(offset, length);
      }
      case DELEGATECALL, STATICCALL -> {
        long offset = Words.clampedToLong(frame.getStackItem(2));
        long length = Words.clampedToLong(frame.getStackItem(3));
        return Range.fromOffsetAndSize(offset, length);
      }
      default ->
          throw new IllegalArgumentException(
              "callDataSegment called outside of a CALL-type instruction");
    }
  }

  public static Range initCodeSegment(final MessageFrame frame, OpCodeData opCode) {
    switch (opCode.mnemonic()) {
      case CREATE, CREATE2 -> {
        long offset = Words.clampedToLong(frame.getStackItem(1));
        long length = Words.clampedToLong(frame.getStackItem(2));
        return Range.fromOffsetAndSize(offset, length);
      }
      default ->
          throw new IllegalArgumentException("callDataSegment called outside of a CREATE(2)");
    }
  }

  /**
   * Returns the RAM segment of the caller containing the calldata if the current operation is a
   * call, throws otherwise.
   *
   * @return the input data segment
   */
  public Range callDataSegment() {
    return callDataSegment(hub.messageFrame(), hub.opCodeData());
  }

  public Range initCodeSegment() {
    return initCodeSegment(hub.messageFrame(), hub.opCodeData());
  }

  /**
   * Return the bytes of the calldata if the current operation is a call, throws otherwise.
   *
   * @return the calldata content
   */
  public Bytes callData() {
    final Range callDataSegment = callDataSegment();
    return maybeShadowReadMemory(callDataSegment, hub.messageFrame());
  }

  /**
   * Return the bytes of the calldata if the current operation is a call, throws otherwise.
   *
   * @param frame the execution context
   * @return the calldata content
   */
  public static Bytes callData(final MessageFrame frame, OpCodeData opCode) {
    final Range callDataSegment = callDataSegment(frame, opCode);
    return maybeShadowReadMemory(callDataSegment, frame);
  }

  public static Bytes initCode(final MessageFrame frame, OpCodeData opCode) {
    final Range initCodeSegment = initCodeSegment(frame, opCode);
    return maybeShadowReadMemory(initCodeSegment, frame);
  }

  /**
   * Returns the RAM segment offered by the caller for the return data if the current operation is a
   * call, throws otherwise.
   *
   * @param frame the execution context
   * @return the return data target
   */
  public static Range returnDataRequestedSegment(final MessageFrame frame, OpCodeData opCode) {
    switch (opCode.mnemonic()) {
      case CALL, CALLCODE -> {
        long offset = Words.clampedToLong(frame.getStackItem(5));
        long length = Words.clampedToLong(frame.getStackItem(6));
        return Range.fromOffsetAndSize(offset, length);
      }
      case DELEGATECALL, STATICCALL -> {
        long offset = Words.clampedToLong(frame.getStackItem(4));
        long length = Words.clampedToLong(frame.getStackItem(5));
        return Range.fromOffsetAndSize(offset, length);
      }
      default ->
          throw new IllegalArgumentException(
              "returnDataRequestedSegment called outside of a *CALL");
    }
  }
}
