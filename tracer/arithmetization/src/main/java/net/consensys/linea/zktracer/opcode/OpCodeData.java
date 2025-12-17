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
import static net.consensys.linea.zktracer.opcode.InstructionFamily.*;
import static net.consensys.linea.zktracer.opcode.OpCode.MSIZE;

import java.util.Objects;
import net.consensys.linea.zktracer.opcode.gas.Billing;
import net.consensys.linea.zktracer.opcode.gas.MxpType;
import net.consensys.linea.zktracer.opcode.stack.StackSettings;
import net.consensys.linea.zktracer.types.UnsignedByte;

/**
 * Contains the {@link OpCode} and its related metadata.
 *
 * @param mnemonic The type of the opcode represented by {@link OpCode}.
 * @param value The actual unsigned byte value of the opcode.
 * @param instructionFamily The {@link InstructionFamily} to which the opcode belongs.
 * @param stackSettings A {@link StackSettings} instance describing how the opcode alters the EVM
 *     stack.
 * @param billing A {@link Billing} instance describing the billing scheme of the instruction.
 */
public record OpCodeData(
    OpCode mnemonic,
    int value,
    InstructionFamily instructionFamily,
    StackSettings stackSettings,
    Billing billing) {

  public Billing billing() {
    return Objects.requireNonNullElse(billing, Billing.DEFAULT);
  }

  public static OpCodeData forNonOpCodes(int value) {
    return new OpCodeData(OpCode.INVALID, value, INVALID, StackSettings.DEFAULT, Billing.DEFAULT);
  }

  /**
   * A method singling out <code>PUSHx</code> instructions with X != 0.
   *
   * @return <code>true</code> if this opcode is a <code>PUSHx</code>
   */
  public boolean isNonTrivialPush() {
    return (EVM_INST_PUSH1 <= value) && (value <= EVM_INST_PUSH32);
  }

  public boolean isPushZero() {
    return value == EVM_INST_PUSH0;
  }

  public boolean isJumpDest() {
    return value == EVM_INST_JUMPDEST;
  }

  public boolean isJump() {
    return instructionFamily == JUMP;
  }

  /**
   * Returns whether this instruction belong to the HALT family.
   *
   * @return true if {@link InstructionFamily} is HALT
   */
  public boolean isHalt() {
    return instructionFamily == HALT;
  }

  public boolean isCall() {
    return instructionFamily == CALL;
  }

  public boolean isCreate() {
    return instructionFamily == CREATE;
  }

  public boolean isCallOrCreate() {
    return isCall() || isCreate();
  }

  public boolean isLog() {
    return instructionFamily == LOG;
  }

  public boolean isCopy() {
    return instructionFamily == COPY;
  }

  /**
   * Returns whether this instruction belong to the INVALID family.
   *
   * @return true if {@link InstructionFamily} is INVALID
   */
  public boolean isInvalid() {
    return instructionFamily == INVALID;
  }

  public int numberOfStackRows() {
    return stackSettings.twoLineInstruction() ? 2 : 1;
  }

  public boolean isMSize() {
    return mnemonic == MSIZE;
  }

  public boolean isFixedSize32() {
    return mnemonic == OpCode.MLOAD || mnemonic == OpCode.MSTORE;
  }

  public boolean isFixedSize1() {
    return mnemonic == OpCode.MSTORE8;
  }

  public boolean isReturn() {
    return mnemonic == OpCode.RETURN;
  }

  public boolean isMCopy() {
    return instructionFamily == MCOPY;
  }

  public boolean isSingleOffset() {
    return mnemonic == OpCode.MLOAD
        || mnemonic == OpCode.MSTORE
        || mnemonic == OpCode.MSTORE8
        || mnemonic == OpCode.REVERT
        || isReturn()
        || isLog()
        || mnemonic == OpCode.SHA3
        || isCopy()
        || isCreate();
  }

  public boolean isDoubleOffset() {
    return isMCopy() || isCall();
  }

  public boolean isWordPricing() {
    return mnemonic == OpCode.SHA3 || isCopy() || isCreate() || mnemonic == OpCode.MCOPY;
  }

  public boolean isBytePricing() {
    return mnemonic == MSIZE
        || mnemonic == OpCode.MLOAD
        || mnemonic == OpCode.MSTORE
        || mnemonic == OpCode.MSTORE8
        || mnemonic == OpCode.REVERT
        || isReturn()
        || isLog()
        || isCall();
  }

  // Used before Cancun, determined by checking if there is a type
  public boolean isMxpLondon() {
    return billing().type() != MxpType.NONE;
  }

  // Used from Cancun and on, before ixMxp is determined by checking if there is a type
  public boolean isMxp() {
    return isMSize() || isSingleOffset() || isDoubleOffset();
  }

  public boolean callHasValueArgument() {
    checkArgument(isCall(), "Expected any CALL opcode, got %s", this);
    return this.mnemonic == OpCode.CALL || this.mnemonic == OpCode.CALLCODE;
  }

  /**
   * Returns the {@link OpCode}'s opcode value as a byte.
   *
   * @return the {@link OpCode}'s opcode value as a byte
   */
  public byte byteValue() {
    return (byte) this.value;
  }

  /**
   * Returns the {@link OpCode}'s long value as an {@link UnsignedByte} type.
   *
   * @return the {@link OpCode}'s value as an {@link UnsignedByte}
   */
  public UnsignedByte unsignedByteValue() {
    return UnsignedByte.of(byteValue());
  }

  public short callCdoStackIndex() {
    return (short) (callHasValueArgument() ? 3 : 2);
  }

  public short callCdsStackIndex() {
    return (short) (callHasValueArgument() ? 4 : 3);
  }

  public short callReturnAtCapacityStackIndex() {
    return (short) (callHasValueArgument() ? 6 : 5);
  }

  public boolean mayTriggerStackUnderflow() {
    return this.stackSettings().delta() > 0;
  }

  public boolean mayTriggerStackOverflow() {
    return this.stackSettings().alpha() > 0;
  }

  public boolean mayTriggerStaticException() {
    return this.stackSettings().forbiddenInStatic();
  }

  public boolean mayTriggerMemoryExpansionException() {
    return mnemonic != MSIZE && this.isMxp();
  }

  @Override
  public boolean equals(Object o) {
    // Instances of this class are not intended to be compared.  Hence, an exception is explicitly
    // raised to quickly
    // identify situations where this method is accidentally being used.
    throw new UnsupportedOperationException();
  }

  @Override
  public int hashCode() {
    // Instances of this class are not intended to be used in hashmaps, etc.  Hence, an exception is
    // explicitly raised
    // to quickly identify situations where this method is accidentally being used.
    throw new UnsupportedOperationException();
  }
}
