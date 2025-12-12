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

package net.consensys.linea.zktracer.module.mmu.values;

import lombok.Builder;
import lombok.Getter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.opcode.OpCode;
import net.consensys.linea.zktracer.types.UnsignedByte;
import org.apache.tuweni.bytes.Bytes;

@Builder
@Getter
@Accessors(fluent = true)
public class MmuWcpCallRecord {
  public static final MmuWcpCallRecord EMPTY_CALL = builder().flag(false).build();

  @Builder.Default private boolean flag = true;
  @Builder.Default private UnsignedByte instruction = UnsignedByte.ZERO;
  @Builder.Default private Bytes arg1Hi = Bytes.EMPTY;
  @Builder.Default private Bytes arg1Lo = Bytes.EMPTY;
  @Builder.Default private Bytes arg2Lo = Bytes.EMPTY;
  private boolean result;

  public static MmuWcpCallRecordBuilder instLtBuilder() {
    return builder().instruction(UnsignedByte.of(Trace.EVM_INST_LT));
  }

  public static MmuWcpCallRecordBuilder instEqBuilder() {
    return builder().instruction(UnsignedByte.of(Trace.EVM_INST_EQ));
  }

  public static MmuWcpCallRecordBuilder instIsZeroBuilder() {
    return builder().instruction(OpCode.ISZERO.unsignedByteValue());
  }
}
