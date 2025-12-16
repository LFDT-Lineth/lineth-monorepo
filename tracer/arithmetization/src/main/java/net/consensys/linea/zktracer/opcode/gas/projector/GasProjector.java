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

package net.consensys.linea.zktracer.opcode.gas.projector;

import static org.hyperledger.besu.evm.internal.Words.clampedToLong;

import lombok.RequiredArgsConstructor;
import net.consensys.linea.zktracer.Fork;
import net.consensys.linea.zktracer.module.hub.transients.OperationAncillaries;
import net.consensys.linea.zktracer.opcode.OpCodeData;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.evm.account.Account;
import org.hyperledger.besu.evm.frame.MessageFrame;
import org.hyperledger.besu.evm.gascalculator.GasCalculator;
import org.hyperledger.besu.evm.internal.Words;

@RequiredArgsConstructor
public class GasProjector {
  final Fork fork;
  final GasCalculator gc;

  public GasProjection of(MessageFrame frame, OpCodeData opCode) {
    return switch (opCode.mnemonic()) {
      case STOP -> new Zero(gc);
      case ADD,
          SUB,
          NOT,
          LT,
          GT,
          SLT,
          SGT,
          EQ,
          ISZERO,
          AND,
          OR,
          XOR,
          BYTE,
          SHL,
          SHR,
          SAR,
          CALLDATALOAD,
          PUSH1,
          PUSH2,
          PUSH3,
          PUSH4,
          PUSH5,
          PUSH6,
          PUSH7,
          PUSH8,
          PUSH9,
          PUSH10,
          PUSH11,
          PUSH12,
          PUSH13,
          PUSH14,
          PUSH15,
          PUSH16,
          PUSH17,
          PUSH18,
          PUSH19,
          PUSH20,
          PUSH21,
          PUSH22,
          PUSH23,
          PUSH24,
          PUSH25,
          PUSH26,
          PUSH27,
          PUSH28,
          PUSH29,
          PUSH30,
          PUSH31,
          PUSH32,
          DUP1,
          DUP2,
          DUP3,
          DUP4,
          DUP5,
          DUP6,
          DUP7,
          DUP8,
          DUP9,
          DUP10,
          DUP11,
          DUP12,
          DUP13,
          DUP14,
          DUP15,
          DUP16,
          SWAP1,
          SWAP2,
          SWAP3,
          SWAP4,
          SWAP5,
          SWAP6,
          SWAP7,
          SWAP8,
          SWAP9,
          SWAP10,
          SWAP11,
          SWAP12,
          SWAP13,
          SWAP14,
          SWAP15,
          SWAP16,
          BLOBHASH ->
          new VeryLow(gc);
      case MUL, DIV, SDIV, MOD, SMOD, SIGNEXTEND, SELFBALANCE, CLZ -> new Low(gc);
      case ADDMOD, MULMOD, JUMP -> new Mid(gc);
      case EXP -> new Exp(gc, frame);
      case SHA3 -> new Sha3(gc, frame);
      case PUSH0,
          ADDRESS,
          ORIGIN,
          CALLER,
          CALLVALUE,
          CALLDATASIZE,
          CODESIZE,
          GASPRICE,
          COINBASE,
          TIMESTAMP,
          NUMBER,
          DIFFICULTY,
          PREVRANDAO,
          GASLIMIT,
          CHAINID,
          RETURNDATASIZE,
          POP,
          PC,
          MSIZE,
          GAS,
          BASEFEE,
          BLOBBASEFEE ->
          new Base(gc);
      case BALANCE, EXTCODESIZE, EXTCODEHASH -> new AccountAccess(fork, gc, frame);
      case CALLDATACOPY, CODECOPY, RETURNDATACOPY -> new DataCopy(gc, frame);
      case MCOPY -> new MCopy(gc, frame);
      case EXTCODECOPY -> new ExtCodeCopy(fork, gc, frame);
      case BLOCKHASH -> new BlockHash(gc);
      case MLOAD, MSTORE -> new MLoadStore(gc, frame);
      case MSTORE8 -> new MStore8(gc, frame);
      case SLOAD -> new SLoad(gc, frame);
      case SSTORE -> new SStore(gc, frame);
      case TLOAD -> new TLoad(gc);
      case TSTORE -> new TStore(gc);
      case JUMPI -> new High(gc);
      case JUMPDEST -> new JumpDest(gc);
      case LOG0 -> new Log(gc, frame, 0);
      case LOG1 -> new Log(gc, frame, 1);
      case LOG2 -> new Log(gc, frame, 2);
      case LOG3 -> new Log(gc, frame, 3);
      case LOG4 -> new Log(gc, frame, 4);
      case CREATE -> new Create(gc, frame);
      case CREATE2 -> new Create2(gc, frame);
      case CALL -> {
        if (frame.stackSize() > 6) {
          final long maxGasAllowance = clampedToLong(frame.getStackItem(0));
          final Address to = Words.toAddress(frame.getStackItem(1));
          final Account recipient = frame.getWorldUpdater().get(to);
          final Wei value = Wei.wrap(frame.getStackItem(2));
          yield new Call(
              fork,
              gc,
              frame,
              maxGasAllowance,
              OperationAncillaries.callDataSegment(frame, opCode),
              OperationAncillaries.returnDataRequestedSegment(frame, opCode),
              value,
              recipient,
              to);
        } else {
          yield Call.invalid();
        }
      }
      case CALLCODE -> {
        if (frame.stackSize() > 6) {
          final long stipend = clampedToLong(frame.getStackItem(0));
          final Account recipient = frame.getWorldUpdater().get(frame.getRecipientAddress());
          final Address to = Words.toAddress(frame.getStackItem(1));
          final Wei value = Wei.wrap(frame.getStackItem(2));
          yield new Call(
              fork,
              gc,
              frame,
              stipend,
              OperationAncillaries.callDataSegment(frame, opCode),
              OperationAncillaries.returnDataRequestedSegment(frame, opCode),
              value,
              recipient,
              to);
        } else {
          yield Call.invalid();
        }
      }
      case DELEGATECALL -> {
        if (frame.stackSize() > 5) {
          final long stipend = clampedToLong(frame.getStackItem(0));
          final Account recipient = frame.getWorldUpdater().get(frame.getRecipientAddress());
          final Address to = Words.toAddress(frame.getStackItem(1));
          yield new Call(
              fork,
              gc,
              frame,
              stipend,
              OperationAncillaries.callDataSegment(frame, opCode),
              OperationAncillaries.returnDataRequestedSegment(frame, opCode),
              Wei.ZERO,
              recipient,
              to);
        } else {
          yield Call.invalid();
        }
      }
      case STATICCALL -> {
        if (frame.stackSize() > 5) {
          final long stipend = clampedToLong(frame.getStackItem(0));
          final Address to = Words.toAddress(frame.getStackItem(1));
          final Account recipient = frame.getWorldUpdater().get(to);
          yield new Call(
              fork,
              gc,
              frame,
              stipend,
              OperationAncillaries.callDataSegment(frame, opCode),
              OperationAncillaries.returnDataRequestedSegment(frame, opCode),
              Wei.ZERO,
              recipient,
              to);
        } else {
          yield Call.invalid();
        }
      }
      case RETURN -> new Return(gc, frame);
      case REVERT -> new Revert(gc, frame);
      case INVALID -> new GasProjection() {};
      case SELFDESTRUCT -> new SelfDestruct(fork, gc, frame);
      default -> throw new IllegalStateException("Unexpected value: " + opCode);
    };
  }
}
