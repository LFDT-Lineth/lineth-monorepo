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

import static net.consensys.linea.zktracer.module.Util.rightPaddedSlice;
import static net.consensys.linea.zktracer.types.Conversions.safeLongToInt;

import lombok.Getter;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.evm.frame.MessageFrame;

/**
 * A {@link MemoryRange} describes a contiguous region in the memory of some execution context. The
 * {@link #contextNumber} serves to identify said execution context. The {@link #range} describes
 * the precise region of memory containing data of interest. The {@link #rawData} is a slice of
 * bytes from which the data can be extracted or reconstructed.
 *
 * <p>Typicallly {@link #rawData} will contain a snapshot of memory at the time of creation of the
 * {@link MemoryRange}, though if {@link #range} is empty, the {@link #rawData} will be discarded.
 * It may or may not contain the actual data as a subset. Regardless, the actual data ought to be
 * {@link #extract}-able from the {@link #rawData} using the {@link #range} and zero-right-padding
 * to the expected size ({@link Range#size()}) if necessary.
 *
 * <p>W
 */
@Getter
public class MemoryRange {

  private final long contextNumber;
  private final Range range;
  private final Bytes rawData;

  public static final MemoryRange EMPTY = new MemoryRange(0);

  /**
   * This method is used when constructing a {@link MemoryRange} using offsets and sizes that are
   * potentially large, e.g. obtained as values popped from the stack.
   *
   * @param contextNumber
   * @param offset
   * @param size
   * @param rawData
   */
  public MemoryRange(final long contextNumber, Bytes offset, Bytes size, final Bytes rawData) {
    this.contextNumber = contextNumber;
    this.range = Range.fromOffsetAndSize(offset, size);
    this.rawData = size.isZero() ? Bytes.EMPTY : rawData;
  }

  /**
   * Constructs an empty memory range associated with {@code contextNumber}.
   *
   * @param contextNumber
   */
  public MemoryRange(long contextNumber) {
    this.contextNumber = contextNumber;
    this.range = Range.empty();
    this.rawData = Bytes.EMPTY;
  }

  public MemoryRange(final long contextNumber, final Range range, final Bytes rawData) {
    this.contextNumber = contextNumber;
    this.range = range;
    this.rawData = range.isEmpty() ? Bytes.EMPTY : rawData;
  }

  public MemoryRange(
      final long contextNumber, final long offset, final long size, final Bytes rawData) {
    this.contextNumber = contextNumber;
    this.range = Range.fromOffsetAndSize(offset, size);
    this.rawData = isEmpty() ? Bytes.EMPTY : rawData;
  }

  public MemoryRange(final long contextNumber, final Range range, final MessageFrame frame) {
    this.contextNumber = contextNumber;
    this.range = range;
    this.rawData =
        range.isEmpty() ? Bytes.EMPTY : frame.shadowReadMemory(0, frame.memoryByteSize());
  }

  public long offset() {
    return range.offset();
  }

  public long size() {
    return range.size();
  }

  public long contextNumber() {
    return contextNumber;
  }

  public Bytes extract() {
    return isEmpty()
        ? Bytes.EMPTY
        : rightPaddedSlice(rawData, safeLongToInt(range.offset()), safeLongToInt(range.size()));
  }

  public boolean isEmpty() {
    return range.isEmpty();
  }

  public MemoryRange snapshot() {
    return new MemoryRange(contextNumber, range.snapshot(), rawData.copy());
  }
}
