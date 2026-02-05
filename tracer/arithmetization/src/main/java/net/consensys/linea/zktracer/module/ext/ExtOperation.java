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

package net.consensys.linea.zktracer.module.ext;

import static net.consensys.linea.zktracer.types.Conversions.bigIntegerToBytes32;

import java.math.BigInteger;
import lombok.EqualsAndHashCode;
import lombok.Getter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.container.ModuleOperation;
import net.consensys.linea.zktracer.opcode.OpCode;
import net.consensys.linea.zktracer.types.UnsignedByte;
import org.apache.tuweni.bytes.Bytes32;

@Accessors(fluent = true)
@EqualsAndHashCode(onlyExplicitlyIncluded = true, callSuper = false)
public class ExtOperation extends ModuleOperation {

  public static final short NB_ROWS_EXT = 1;

  @EqualsAndHashCode.Include @Getter private final OpCode opCode;
  @EqualsAndHashCode.Include @Getter private final Bytes32 a;
  @EqualsAndHashCode.Include @Getter private final Bytes32 b;
  @EqualsAndHashCode.Include @Getter private final Bytes32 m;
  @Getter private Bytes32 result;

  public ExtOperation(OpCode opCode, Bytes32 a, Bytes32 b, Bytes32 m) {
    this.opCode = opCode;
    this.a = a;
    this.b = b;
    this.m = m;
  }

  void computeResult() {
    if (m.isZero()) {
      result = Bytes32.ZERO;
      return;
    }
    final BigInteger aBI = a.toUnsignedBigInteger();
    final BigInteger bBI = b.toUnsignedBigInteger();
    final BigInteger mBI = m.toUnsignedBigInteger();
    switch (opCode) {
      case ADDMOD -> result = bigIntegerToBytes32((aBI.add(bBI)).mod(mBI));
      case MULMOD -> result = bigIntegerToBytes32((aBI.multiply(bBI)).mod(mBI));
      default -> throw new IllegalArgumentException("OpCode not supported by EXT module" + opCode);
    }
  }

  @Override
  protected int computeLineCount() {
    return NB_ROWS_EXT;
  }

  void trace(Trace.Ext trace) {
    trace.inst(UnsignedByte.of(opCode.byteValue())).a(a).b(b).m(m).res(result).validateRow();
  }
}
