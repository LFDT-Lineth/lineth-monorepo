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

package net.consensys.linea.zktracer.module.exp;

import java.util.Comparator;

public class ExpOperationComparator implements Comparator<ExpOperation> {
  @Override
  public int compare(ExpOperation op1, ExpOperation op2) {
    final int instructionComp =
        Integer.compare(op1.expCall().expInstruction(), op2.expCall().expInstruction());
    if (instructionComp != 0) {
      return instructionComp;
    }

    return op1.expCall.compareTo(op2.expCall);
  }
}
