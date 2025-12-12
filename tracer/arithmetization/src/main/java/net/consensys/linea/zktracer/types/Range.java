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

package net.consensys.linea.zktracer.types;

import static org.hyperledger.besu.evm.internal.Words.clampedToLong;

import net.consensys.linea.zktracer.opcode.OpCodeData;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.evm.frame.MessageFrame;

/**
 * A MemorySpan describes a contiguous region in an account memory.
 *
 * @param offset the region start
 * @param size the region size
 */
public record Range(long offset, long size) {

  private static final Range EMPTY = new Range(0, 0);

  public static Range empty() {
    return EMPTY;
  }

  public static Range fromOffsetAndSize(long offset, long size) {
    return new Range(offset, size);
  }

  /**
   * This method is used when constructing a {@link Range} using offsets and sizes that are
   * potentially large. When the {@link #size} is zero we discard both {@link #offset} and {@link
   * #size} and return {@link #EMPTY}.
   *
   * @param offset
   * @param size
   * @return
   */
  public static Range fromOffsetAndSize(Bytes offset, Bytes size) {
    return size.isZero() ? Range.empty() : new Range(clampedToLong(offset), clampedToLong(size));
  }

  public static Range callDataRange(MessageFrame frame, OpCodeData opCode) {
    final Bytes offset = frame.getStackItem(opCode.callCdoStackIndex());
    final Bytes size = frame.getStackItem(opCode.callCdsStackIndex());
    return fromOffsetAndSize(offset, size);
  }

  public boolean isEmpty() {
    return this.size == 0;
  }

  public Range snapshot() {
    return new Range(this.offset, this.size);
  }

  public boolean besuOverflow() {
    return this.offset >= Integer.MAX_VALUE || this.size >= Integer.MAX_VALUE;
  }

  @Override
  public String toString() {
    return "[%d ..+ %d]".formatted(offset, size);
  }
}
