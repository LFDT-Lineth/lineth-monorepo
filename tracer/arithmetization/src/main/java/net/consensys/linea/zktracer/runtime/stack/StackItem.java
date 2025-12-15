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

import lombok.Getter;
import lombok.Setter;
import lombok.experimental.Accessors;
import org.apache.tuweni.bytes.Bytes;

/**
 * An atomic operation (read/pop or write/push) on the stack, indexed within a {@link
 * IndexedStackOperation}.
 *
 * <p>Alterations of the stack by an EVM instruction are decomposed in one or two chunks, or {@link
 * StackLine}, made of {@link IndexedStackOperation}, which are then stored, with the associated
 * metadata, in a {@link StackContext}.
 */
@Accessors(fluent = true)
public final class StackItem {
  private static final Bytes MARKER = Bytes.fromHexString("0x1337deadbeef");

  /**
   * The relative height of the element with regard to the stack height just before executing the
   * linked EVM instruction.
   */
  @Getter private final short height;

  /** The value having been popped from/pushed on the stack. */
  @Getter @Setter private Bytes value;

  /** whether this action is a push or a pop. */
  @Getter private final byte action;

  /**
   * The stamp of this operation relative to the stack stamp before executing the linked EVM
   * instruction.
   */
  @Getter private final int stackStamp;

  /** Singleton ``empty stack operation'' object. */
  private static final StackItem EMPTY_STACK_ITEM = new StackItem();

  public static StackItem empty() {
    return EMPTY_STACK_ITEM;
  }

  /** private constructor for singleton definition in {@link StackItem#EMPTY_STACK_ITEM} */
  private StackItem() {
    this.height = 0;
    this.value = Bytes.EMPTY;
    this.action = Stack.NONE;
    this.stackStamp = 0;
  }

  StackItem(short height, Bytes value, byte action, int stackStamp) {
    this.height = height;
    this.value = value;
    this.action = action;
    this.stackStamp = stackStamp;
  }

  public static StackItem pop(short height, Bytes value, int stackStamp) {
    return new StackItem(height, value, Stack.POP, stackStamp);
  }

  public static StackItem push(short height, int stackStamp) {
    return new StackItem(
        height, MARKER /* marker value, erased on unlatching */, Stack.PUSH, stackStamp);
  }

  public static StackItem pushImmediate(short height, Bytes val, int stackStamp) {
    return new StackItem(height, val.copy(), Stack.PUSH, stackStamp);
  }
}
