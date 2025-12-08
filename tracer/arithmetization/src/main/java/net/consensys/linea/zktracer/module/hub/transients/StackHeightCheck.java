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
import static net.consensys.linea.zktracer.runtime.stack.Stack.MAX_STACK_SIZE;

import lombok.EqualsAndHashCode;

@EqualsAndHashCode(onlyExplicitlyIncluded = true, callSuper = false)
public class StackHeightCheck {
  private static final Integer SHIFT_FACTOR =
      8; // 5 would suffice but 8 makes it a byte shift, not sure if it matters

  @EqualsAndHashCode.Include final int comparison;

  /**
   * This constructor creates a {@link StackHeightCheck} for stack underflow detection.
   *
   * @param height stack height pre opcode execution
   * @param delta greatest depth at which touched stack items
   */
  public StackHeightCheck(int height, int delta) {
    checkArgument(
        0 <= height && height <= MAX_STACK_SIZE && 0 <= delta && delta <= 17,
        "StackHeightCheck constructor provided with Invalid height %s or delta %s",
        height,
        delta);
    comparison = height << SHIFT_FACTOR | delta;
  }

  /**
   * This constructor creates a {@link StackHeightCheck} for stack overflow detection.
   *
   * @param heightNew stack height post opcode execution
   */
  public StackHeightCheck(int heightNew) {
    checkArgument(
        0 <= heightNew && heightNew <= MAX_STACK_SIZE + 1,
        "StackHeightCheck constructor provided with Invalid heightNew %s",
        heightNew);
    comparison = heightNew;
  }
}
