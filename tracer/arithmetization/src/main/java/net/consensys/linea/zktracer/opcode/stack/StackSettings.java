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

package net.consensys.linea.zktracer.opcode.stack;

import net.consensys.linea.zktracer.opcode.gas.GasConstants;

/**
 * Stores instruction-specific data that are required to generate the stack traces.
 *
 * @param pattern the stack pattern as given in the spec
 * @param alpha alpha as set in the spec
 * @param delta delta as set in the sped
 * @param staticGas the static part of the gas consumed by this operation
 * @param twoLineInstruction whether this operation fills one or two stack lines
 * @param forbiddenInStatic whether this instruction is forbidden in a static context
 * @param addressTrimmingInstruction whether this instruction triggers addres trimming
 * @param oobFlag whether this instruction may trigger an OoB exception
 * @param flag1
 * @param flag2
 * @param flag3
 * @param flag4
 */
public record StackSettings(
    Pattern pattern,
    int alpha,
    int delta,
    GasConstants staticGas,
    boolean twoLineInstruction,
    boolean forbiddenInStatic,
    boolean addressTrimmingInstruction,
    boolean oobFlag,
    boolean flag1,
    boolean flag2,
    boolean flag3,
    boolean flag4) {
  public static final StackSettings DEFAULT =
      new StackSettings(
          Pattern.ZERO_ZERO,
          0,
          0,
          GasConstants.G_ZERO,
          false,
          false,
          false,
          false,
          false,
          false,
          false,
          false);
}
