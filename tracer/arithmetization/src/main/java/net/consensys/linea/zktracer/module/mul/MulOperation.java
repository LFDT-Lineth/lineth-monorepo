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

package net.consensys.linea.zktracer.module.mul;

import lombok.EqualsAndHashCode;
import lombok.Getter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.container.ModuleOperation;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes32;
import org.apache.tuweni.units.bigints.UInt256;

@Accessors(fluent = true)
@EqualsAndHashCode(onlyExplicitlyIncluded = true, callSuper = false)
public class MulOperation extends ModuleOperation {
  @EqualsAndHashCode.Include @Getter private final OpCode opCode;
  @EqualsAndHashCode.Include @Getter private final Bytes32 arg1;
  @EqualsAndHashCode.Include @Getter private final Bytes32 arg2;

  public MulOperation(OpCode opCode, Bytes32 arg1, Bytes32 arg2) {
    this.opCode = opCode;
    this.arg1 = arg1;
    this.arg2 = arg2;
  }

  void trace(Trace.Mul trace) {
    final UInt256 res =
        switch (opCode) {
          case MUL -> UInt256.fromBytes(arg1).multiply(UInt256.fromBytes(arg2));
          case EXP -> UInt256.fromBytes(arg1).pow(UInt256.fromBytes(arg2));
          default -> throw new IllegalStateException("[MUL module] Unexpected opcode: " + opCode);
        };
    // trace inputs / outputs
    trace.inst(opCode.byteValue()).arg1(arg1).arg2(arg2).res(res).validateRow();
  }

  @Override
  protected int computeLineCount() {
    return 1;
  }
}
