/*
 * Copyright ConsenSys Inc.
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

package net.consensys.linea.zktracer.module.mul;

import java.util.Comparator;

public class MulOperationComparator implements Comparator<MulOperation> {
  @Override
  public int compare(MulOperation op1, MulOperation op2) {
    // First sort by OpCode
    final int opCodeComp = op1.opCode().compareTo(op2.opCode());
    if (opCodeComp != 0) {
      return opCodeComp;
    }
    // Second sort by Arg1
    final int arg1Comp = op1.arg1().compareTo(op2.arg1());
    if (arg1Comp != 0) {
      return arg1Comp;
    }
    // Third, sort by Arg2
    return op1.arg2().compareTo(op2.arg2());
  }
}
