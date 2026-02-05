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

package net.consensys.linea.zktracer.module.ext;

import java.util.Comparator;

public class ExtOperationComparator implements Comparator<ExtOperation> {
  @Override
  public int compare(ExtOperation op1, ExtOperation op2) {
    // First sort by OpCode
    final int opCodeComp = op1.opCode().compareTo(op2.opCode());
    if (opCodeComp != 0) {
      return opCodeComp;
    }
    // Second sort by Arg1
    final int arg1Comp = op1.a().compareTo(op2.a());
    if (arg1Comp != 0) {
      return arg1Comp;
    }
    // Third, sort by Arg2
    final int arg2Comp = op1.b().compareTo(op2.b());
    if (arg2Comp != 0) {
      return arg2Comp;
    }
    // Fourth, sort by Arg3
    return op1.m().compareTo(op2.m());
  }
}
