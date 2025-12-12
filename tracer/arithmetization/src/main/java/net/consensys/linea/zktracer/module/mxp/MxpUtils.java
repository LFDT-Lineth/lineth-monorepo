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

package net.consensys.linea.zktracer.module.mxp;

import static net.consensys.linea.zktracer.Trace.GAS_CONST_G_MEMORY;
import static net.consensys.linea.zktracer.Trace.WORD_SIZE;
import static net.consensys.linea.zktracer.types.EWord.ONE;
import static org.hyperledger.besu.evm.internal.Words.clampedAdd;
import static org.hyperledger.besu.evm.internal.Words.clampedMultiply;

import net.consensys.linea.zktracer.opcode.OpCode;
import net.consensys.linea.zktracer.opcode.OpCodeData;
import net.consensys.linea.zktracer.types.EWord;
import org.hyperledger.besu.evm.frame.MessageFrame;

public class MxpUtils {

  public static boolean isWordPricingOpcode(OpCodeData opCode) {
    return opCode.mnemonic() == OpCode.SHA3
        || opCode.isCopy()
        || opCode.isCreate()
        || opCode.mnemonic() == OpCode.MCOPY;
  }

  /**
   * Method to retrieve sizes and offsets from the stack based on the current opcode.
   *
   * @param frame where stack items are retrieved
   * @return EWord[] array with [size1, offset1, size2, offset2]
   */
  public static EWord[] getSizesAndOffsets(MessageFrame frame, OpCodeData opCode) {
    EWord size1 = EWord.ZERO;
    EWord offset1 = EWord.ZERO;
    EWord size2 = EWord.ZERO;
    EWord offset2 = EWord.ZERO;
    switch (opCode.mnemonic()) {
      case MSIZE -> {}
      case MLOAD, MSTORE -> {
        offset1 = EWord.of(frame.getStackItem(0));
        size1 = EWord.of(WORD_SIZE);
      }
      case MSTORE8 -> {
        offset1 = EWord.of(frame.getStackItem(0));
        size1 = ONE;
      }
      case REVERT, RETURN, LOG0, LOG1, LOG2, LOG3, LOG4, SHA3 -> {
        offset1 = EWord.of(frame.getStackItem(0));
        size1 = EWord.of(frame.getStackItem(1));
      }
      case CALLDATACOPY, RETURNDATACOPY, CODECOPY -> {
        offset1 = EWord.of(frame.getStackItem(0));
        size1 = EWord.of(frame.getStackItem(2));
      }
      case EXTCODECOPY -> {
        offset1 = EWord.of(frame.getStackItem(1));
        size1 = EWord.of(frame.getStackItem(3));
      }
      case CREATE, CREATE2 -> {
        offset1 = EWord.of(frame.getStackItem(1));
        size1 = EWord.of(frame.getStackItem(2));
      }
      case MCOPY -> {
        offset1 = EWord.of(frame.getStackItem(0));
        size1 = EWord.of(frame.getStackItem(2));
        offset2 = EWord.of(frame.getStackItem(1));
        size2 = EWord.of(frame.getStackItem(2));
      }
      case CALL, CALLCODE -> {
        offset1 = EWord.of(frame.getStackItem(3));
        size1 = EWord.of(frame.getStackItem(4));
        offset2 = EWord.of(frame.getStackItem(5));
        size2 = EWord.of(frame.getStackItem(6));
      }
      case DELEGATECALL, STATICCALL -> {
        offset1 = EWord.of(frame.getStackItem(2));
        size1 = EWord.of(frame.getStackItem(3));
        offset2 = EWord.of(frame.getStackItem(4));
        size2 = EWord.of(frame.getStackItem(5));
      }
      default -> throw new IllegalStateException("Unexpected value: " + opCode);
    }

    return new EWord[] {size1, offset1, size2, offset2};
  }

  // This is a copy and past from FrontierGasCalculator.java
  public static long memoryCost(final long length) {
    final long lengthSquare = clampedMultiply(length, length);
    final long base =
        (lengthSquare == Long.MAX_VALUE)
            ? clampedMultiply(length / 512, length)
            : lengthSquare / 512;
    return clampedAdd(clampedMultiply(GAS_CONST_G_MEMORY, length), base);
  }
}
