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

package net.consensys.linea.zktracer.module.bin;

import static net.consensys.linea.zktracer.bytestheta.BaseBytes.fromInt;

import lombok.EqualsAndHashCode;
import lombok.Getter;
import lombok.RequiredArgsConstructor;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.bytestheta.BaseBytes;
import net.consensys.linea.zktracer.container.ModuleOperation;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;

@Getter
@Accessors(fluent = true)
@EqualsAndHashCode(onlyExplicitlyIncluded = true, callSuper = false)
@RequiredArgsConstructor
public class BinOperation extends ModuleOperation {
  @EqualsAndHashCode.Include private final OpCode opCode;
  @EqualsAndHashCode.Include private final BaseBytes arg1;
  @EqualsAndHashCode.Include private final BaseBytes arg2;

  @Override
  protected int computeLineCount() {
    return 1;
  }

  private boolean isSmall() {
    return arg1.getBytes32().trimLeadingZeros().bitLength() < 6;
  }

  private BaseBytes getResult() {
    return switch (opCode) {
      case AND -> arg1.and(arg2);
      case OR -> arg1.or(arg2);
      case XOR -> BaseBytes.fromBytes32(arg1.getBytes32().xor(arg2.getBytes32()));
      case NOT -> arg1.not();
      case BYTE -> byteResult();
      case SIGNEXTEND -> signExtensionResult();
      case CLZ -> fromInt(256 - arg1.getBytes32().bitLength());
      default -> throw new IllegalStateException("Bin doesn't support OpCode" + opCode);
    };
  }

  private BaseBytes signExtensionResult() {
    if (!isSmall()) {
      return arg2;
    }
    final int indexLeadingByte = 31 - arg1.getByte(31) & 0xff;
    final byte toSet = (byte) (arg2().getByte(indexLeadingByte) < 0 ? 0xff : 0x00);
    return BaseBytes.fromBytes32(
        Bytes32.leftPad(arg2.getBytes32().slice(indexLeadingByte, 32 - indexLeadingByte), toSet));
  }

  private BaseBytes byteResult() {
    final int result;
    //
    if (!isSmall()) {
      result = 0;
    } else {
      // Convert arg1 into byte value
      int pivot = arg1.getByte(31);
      // Extract byte at given position
      result = arg2.getByte(pivot) & 0xff;
    }
    return BaseBytes.fromBytes32(Bytes32.leftPad(Bytes.ofUnsignedShort(result)));
  }

  public void traceBinOperation(Trace.Bin trace) {
    trace
        .inst(opCode.unsignedByteValue())
        .argument1(arg1.getBytes32())
        .argument2(arg2.getBytes32())
        .res(getResult().getBytes32())
        .validateRow();
  }
}
