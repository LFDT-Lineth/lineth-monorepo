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

package net.consensys.linea.zktracer.module.wcp;

import static net.consensys.linea.zktracer.Trace.EVM_INST_EQ;
import static net.consensys.linea.zktracer.Trace.EVM_INST_GT;
import static net.consensys.linea.zktracer.Trace.EVM_INST_ISZERO;
import static net.consensys.linea.zktracer.Trace.EVM_INST_LT;
import static net.consensys.linea.zktracer.Trace.EVM_INST_SGT;
import static net.consensys.linea.zktracer.Trace.EVM_INST_SLT;
import static net.consensys.linea.zktracer.Trace.WCP_INST_GEQ;
import static net.consensys.linea.zktracer.Trace.WCP_INST_LEQ;
import static net.consensys.linea.zktracer.types.Conversions.reallyToSignedBigInteger;

import java.security.InvalidParameterException;
import lombok.EqualsAndHashCode;
import lombok.Getter;
import lombok.experimental.Accessors;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.container.ModuleOperation;
import net.consensys.linea.zktracer.types.UnsignedByte;
import org.apache.tuweni.bytes.Bytes32;

@Accessors(fluent = true)
@Slf4j
@EqualsAndHashCode(onlyExplicitlyIncluded = true, callSuper = false)
public class WcpOperation extends ModuleOperation {
  public static final byte LEQbv = (byte) WCP_INST_LEQ;
  public static final byte GEQbv = (byte) WCP_INST_GEQ;
  static final byte LTbv = (byte) EVM_INST_LT;
  static final byte GTbv = (byte) EVM_INST_GT;
  static final byte SLTbv = (byte) EVM_INST_SLT;
  static final byte SGTbv = (byte) EVM_INST_SGT;
  static final byte EQbv = (byte) EVM_INST_EQ;
  static final byte ISZERObv = (byte) EVM_INST_ISZERO;

  private final byte wcpInst;
  @EqualsAndHashCode.Include @Getter private final Bytes32 arg1;
  @EqualsAndHashCode.Include @Getter private final Bytes32 arg2;

  public WcpOperation(final byte wcpInst, final Bytes32 arg1, final Bytes32 arg2) {
    this.wcpInst = wcpInst;
    this.arg1 = arg1;
    this.arg2 = arg2;
  }

  private boolean calculateResult(byte opCode, Bytes32 arg1, Bytes32 arg2) {
    return switch (opCode) {
      case EQbv -> arg1.compareTo(arg2) == 0;
      case ISZERObv -> arg1.isZero();
      case SLTbv -> reallyToSignedBigInteger(arg1).compareTo(reallyToSignedBigInteger(arg2)) < 0;
      case SGTbv -> reallyToSignedBigInteger(arg1).compareTo(reallyToSignedBigInteger(arg2)) > 0;
      case LTbv -> arg1.compareTo(arg2) < 0;
      case GTbv -> arg1.compareTo(arg2) > 0;
      case LEQbv -> arg1.compareTo(arg2) <= 0;
      case GEQbv -> arg1.compareTo(arg2) >= 0;
      default -> throw new InvalidParameterException("Invalid opcode");
    };
  }

  void trace(Trace.Wcp trace) {
    // Calculate result
    final boolean res = calculateResult(wcpInst, arg1, arg2);
    final UnsignedByte inst = UnsignedByte.of(wcpInst);
    //
    trace.inst(inst).arg1(arg1).arg2(arg2).res(res).validateRow();
  }

  @Override
  protected int computeLineCount() {
    return 1;
  }
}
