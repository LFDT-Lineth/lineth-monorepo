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

package net.consensys.linea.zktracer.opcode;

import static com.google.common.base.Preconditions.*;
import static net.consensys.linea.zktracer.Trace.*;

import net.consensys.linea.zktracer.types.UnsignedByte;

/** Represents the entire set of opcodes that are required by the arithmetization process. */
public enum OpCode {
  STOP(EVM_INST_STOP),
  ADD(EVM_INST_ADD),
  MUL(EVM_INST_MUL),
  SUB(EVM_INST_SUB),
  DIV(EVM_INST_DIV),
  SDIV(EVM_INST_SDIV),
  MOD(EVM_INST_MOD),
  SMOD(EVM_INST_SMOD),
  ADDMOD(EVM_INST_ADDMOD),
  MULMOD(EVM_INST_MULMOD),
  EXP(EVM_INST_EXP),
  SIGNEXTEND(EVM_INST_SIGNEXTEND),
  LT(EVM_INST_LT),
  GT(EVM_INST_GT),
  SLT(EVM_INST_SLT),
  SGT(EVM_INST_SGT),
  EQ(EVM_INST_EQ),
  ISZERO(EVM_INST_ISZERO),
  AND(EVM_INST_AND),
  OR(EVM_INST_OR),
  XOR(EVM_INST_XOR),
  NOT(EVM_INST_NOT),
  BYTE(EVM_INST_BYTE),
  SHL(EVM_INST_SHL),
  SHR(EVM_INST_SHR),
  SAR(EVM_INST_SAR),
  CLZ(EVM_INST_CLZ),
  SHA3(EVM_INST_SHA3),
  ADDRESS(EVM_INST_ADDRESS),
  BALANCE(EVM_INST_BALANCE),
  ORIGIN(EVM_INST_ORIGIN),
  CALLER(EVM_INST_CALLER),
  CALLVALUE(EVM_INST_CALLVALUE),
  CALLDATALOAD(EVM_INST_CALLDATALOAD),
  CALLDATASIZE(EVM_INST_CALLDATASIZE),
  CALLDATACOPY(EVM_INST_CALLDATACOPY),
  CODESIZE(EVM_INST_CODESIZE),
  CODECOPY(EVM_INST_CODECOPY),
  GASPRICE(EVM_INST_GASPRICE),
  EXTCODESIZE(EVM_INST_EXTCODESIZE),
  EXTCODECOPY(EVM_INST_EXTCODECOPY),
  RETURNDATASIZE(EVM_INST_RETURNDATASIZE),
  RETURNDATACOPY(EVM_INST_RETURNDATACOPY),
  EXTCODEHASH(EVM_INST_EXTCODEHASH),
  BLOCKHASH(EVM_INST_BLOCKHASH),
  COINBASE(EVM_INST_COINBASE),
  TIMESTAMP(EVM_INST_TIMESTAMP),
  NUMBER(EVM_INST_NUMBER),
  DIFFICULTY(EVM_INST_DIFFICULTY),
  PREVRANDAO(EVM_INST_PREVRANDAO),
  GASLIMIT(EVM_INST_GASLIMIT),
  CHAINID(EVM_INST_CHAINID),
  SELFBALANCE(EVM_INST_SELFBALANCE),
  BASEFEE(EVM_INST_BASEFEE),
  BLOBBASEFEE(EVM_INST_BLOBBASEFEE),
  BLOBHASH(EVM_INST_BLOBHASH),
  POP(EVM_INST_POP),
  MLOAD(EVM_INST_MLOAD),
  MSTORE(EVM_INST_MSTORE),
  MSTORE8(EVM_INST_MSTORE8),
  SLOAD(EVM_INST_SLOAD),
  SSTORE(EVM_INST_SSTORE),
  JUMP(EVM_INST_JUMP),
  JUMPI(EVM_INST_JUMPI),
  PC(EVM_INST_PC),
  MSIZE(EVM_INST_MSIZE),
  GAS(EVM_INST_GAS),
  JUMPDEST(EVM_INST_JUMPDEST),
  TLOAD(EVM_INST_TLOAD),
  TSTORE(EVM_INST_TSTORE),
  PUSH0(EVM_INST_PUSH0),
  PUSH1(EVM_INST_PUSH1),
  PUSH2(EVM_INST_PUSH2),
  PUSH3(EVM_INST_PUSH3),
  PUSH4(EVM_INST_PUSH4),
  PUSH5(EVM_INST_PUSH5),
  PUSH6(EVM_INST_PUSH6),
  PUSH7(EVM_INST_PUSH7),
  PUSH8(EVM_INST_PUSH8),
  PUSH9(EVM_INST_PUSH9),
  PUSH10(EVM_INST_PUSH10),
  PUSH11(EVM_INST_PUSH11),
  PUSH12(EVM_INST_PUSH12),
  PUSH13(EVM_INST_PUSH13),
  PUSH14(EVM_INST_PUSH14),
  PUSH15(EVM_INST_PUSH15),
  PUSH16(EVM_INST_PUSH16),
  PUSH17(EVM_INST_PUSH17),
  PUSH18(EVM_INST_PUSH18),
  PUSH19(EVM_INST_PUSH19),
  PUSH20(EVM_INST_PUSH20),
  PUSH21(EVM_INST_PUSH21),
  PUSH22(EVM_INST_PUSH22),
  PUSH23(EVM_INST_PUSH23),
  PUSH24(EVM_INST_PUSH24),
  PUSH25(EVM_INST_PUSH25),
  PUSH26(EVM_INST_PUSH26),
  PUSH27(EVM_INST_PUSH27),
  PUSH28(EVM_INST_PUSH28),
  PUSH29(EVM_INST_PUSH29),
  PUSH30(EVM_INST_PUSH30),
  PUSH31(EVM_INST_PUSH31),
  PUSH32(EVM_INST_PUSH32),
  DUP1(EVM_INST_DUP1),
  DUP2(EVM_INST_DUP2),
  DUP3(EVM_INST_DUP3),
  DUP4(EVM_INST_DUP4),
  DUP5(EVM_INST_DUP5),
  DUP6(EVM_INST_DUP6),
  DUP7(EVM_INST_DUP7),
  DUP8(EVM_INST_DUP8),
  DUP9(EVM_INST_DUP9),
  DUP10(EVM_INST_DUP10),
  DUP11(EVM_INST_DUP11),
  DUP12(EVM_INST_DUP12),
  DUP13(EVM_INST_DUP13),
  DUP14(EVM_INST_DUP14),
  DUP15(EVM_INST_DUP15),
  DUP16(EVM_INST_DUP16),
  SWAP1(EVM_INST_SWAP1),
  SWAP2(EVM_INST_SWAP2),
  SWAP3(EVM_INST_SWAP3),
  SWAP4(EVM_INST_SWAP4),
  SWAP5(EVM_INST_SWAP5),
  SWAP6(EVM_INST_SWAP6),
  SWAP7(EVM_INST_SWAP7),
  SWAP8(EVM_INST_SWAP8),
  SWAP9(EVM_INST_SWAP9),
  SWAP10(EVM_INST_SWAP10),
  SWAP11(EVM_INST_SWAP11),
  SWAP12(EVM_INST_SWAP12),
  SWAP13(EVM_INST_SWAP13),
  SWAP14(EVM_INST_SWAP14),
  SWAP15(EVM_INST_SWAP15),
  SWAP16(EVM_INST_SWAP16),
  LOG0(EVM_INST_LOG0),
  LOG1(EVM_INST_LOG1),
  LOG2(EVM_INST_LOG2),
  LOG3(EVM_INST_LOG3),
  LOG4(EVM_INST_LOG4),
  CREATE(EVM_INST_CREATE),
  CALL(EVM_INST_CALL),
  CALLCODE(EVM_INST_CALLCODE),
  RETURN(EVM_INST_RETURN),
  DELEGATECALL(EVM_INST_DELEGATECALL),
  CREATE2(EVM_INST_CREATE2),
  STATICCALL(EVM_INST_STATICCALL),
  REVERT(EVM_INST_REVERT),
  INVALID(EVM_INST_INVALID),
  SELFDESTRUCT(EVM_INST_SELFDESTRUCT),
  MCOPY(EVM_INST_MCOPY);

  private final int opcode;

  OpCode(int opcode) {
    this.opcode = opcode;
  }

  public int getOpcode() {
    return opcode;
  }

  /**
   * Convert a mnemonic in any case into the matching {@link OpCode}.
   *
   * @param mnemonic the opcode menmonic
   * @return the corresponding OpCode
   */
  public static OpCode fromMnemonic(final String mnemonic) {
    return OpCode.valueOf(mnemonic.toUpperCase());
  }

  /**
   * Returns the {@link OpCode}'s opcode value as a byte.
   *
   * @return the {@link OpCode}'s opcode value as a byte
   */
  public byte byteValue() {
    return (byte) this.opcode;
  }

  /**
   * Returns the {@link OpCode}'s long value as an {@link UnsignedByte} type.
   *
   * @return the {@link OpCode}'s value as an {@link UnsignedByte}
   */
  public UnsignedByte unsignedByteValue() {
    return UnsignedByte.of(byteValue());
  }

  /**
   * Matches if the current {@link OpCode} is contained within a list of {@link OpCode}s.
   *
   * @param opCodes list of {@link OpCode}s to match against.
   * @return if the current {@link OpCode} is contained within the list.
   */
  public boolean isAnyOf(OpCode... opCodes) {
    for (OpCode opCode : opCodes) {
      if (opCode.equals(this)) {
        return true;
      }
    }

    return false;
  }
}
