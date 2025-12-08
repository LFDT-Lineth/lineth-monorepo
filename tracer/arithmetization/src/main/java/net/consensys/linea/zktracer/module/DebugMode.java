/*
 * Copyright ConsenSys AG.
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

package net.consensys.linea.zktracer.module;

import java.util.List;

import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.DelegatingBytes;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Quantity;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.evm.frame.MessageFrame;
import org.hyperledger.besu.evm.log.Log;
import org.hyperledger.besu.evm.operation.Operation;
import org.hyperledger.besu.evm.worldstate.WorldView;
import org.hyperledger.besu.plugin.data.BlockBody;
import org.hyperledger.besu.plugin.data.ProcessableBlockHeader;

@Slf4j
@RequiredArgsConstructor
public class DebugMode {
  public static class PinLevel {
    private static final int CONFLATION = 1;
    private static final int BLOCK = 1 << 2;
    private static final int TX = 1 << 3;
    private static final int INST = 1 << 4;
    private static final int CONTEXT = 1 << 5;
    private static final int STACK = 1 << 6;

    int level;

    private PinLevel(int x) {
      this.level = x;
    }

    public PinLevel() {
      this.level = 0;
    }

    public PinLevel all() {
      this.level = 0xffff;
      return this;
    }

    public boolean none() {
      return this.level == 0;
    }

    public PinLevel conflation(boolean x) {
      if (x) {
        this.level |= CONFLATION;
      } else {
        this.level &= ~CONFLATION;
      }
      return this;
    }

    public PinLevel block(boolean x) {
      if (x) {
        this.level |= BLOCK;
      } else {
        this.level &= ~BLOCK;
      }
      return this;
    }

    public PinLevel tx(boolean x) {
      if (x) {
        this.level |= TX;
      } else {
        this.level &= ~TX;
      }
      return this;
    }

    public PinLevel context(boolean x) {
      if (x) {
        this.level |= CONTEXT;
      } else {
        this.level &= ~CONTEXT;
      }
      return this;
    }

    public PinLevel opCode(boolean x) {
      if (x) {
        this.level |= INST;
      } else {
        this.level &= ~INST;
      }
      return this;
    }

    public PinLevel stack(boolean x) {
      if (x) {
        this.level |= STACK;
      } else {
        this.level &= ~STACK;
      }
      return this;
    }

    public boolean doConflation() {
      return (this.level & CONFLATION) > 0;
    }

    public boolean doBlock() {
      return (this.level & BLOCK) > 0;
    }

    public boolean doTx() {
      return (this.level & TX) > 0;
    }

    public boolean doContext() {
      return (this.level & CONTEXT) > 0;
    }

    public boolean doOpcode() {
      return (this.level & INST) > 0;
    }

    public boolean doStack() {
      return (this.level & STACK) > 0;
    }
  }

  private static final int MAX_STACK_ELT_DISPLAY = 5;
  private final PinLevel l;
  private final Hub hub;

  public void traceStartConflation(long blockCount) {
    if (!this.l.doConflation()) {
      return;
    }
    log.info("=== Start conflation ({} blocks) ===", blockCount);
  }

  public void traceEndConflation() {
    if (!this.l.doConflation()) {
      return;
    }
    log.info("=== Stop conflation ===");
  }

  public void traceStartBlock(
      ProcessableBlockHeader processableBlockHeader,
      final BlockBody body,
      final Address miningBeneficiary) {
    if (!this.l.doBlock()) {
      return;
    }
    log.info(
        "== Enter block {} ({} txs.)",
        processableBlockHeader.getNumber(),
        body.getTransactions().size());
    log.info("    coinbase:  {}", processableBlockHeader.getCoinbase());
    log.info("    gas limit: {}", processableBlockHeader.getGasLimit());
    log.info(
        "    base fee:  {}",
        processableBlockHeader.getBaseFee().map(Quantity::toHexString).orElse("N/A"));
  }

  public void traceEndBlock() {
    if (!this.l.doBlock()) {
      return;
    }
    log.info("== End of block");
  }

  public void tracePrepareTx(WorldView worldView, Transaction tx) {
    if (!this.l.doTx()) {
      return;
    }
    log.info("= Starting transaction {}", tx.getHash());
    log.info(" -- General --");
    log.info("  nonce: {}", tx.getNonce());
    log.info("  from:  {}", tx.getSender());
    log.info("  to:    {}", tx.getTo().map(DelegatingBytes::toString).orElse("NONE"));
    log.info("  type:  {}", tx.getType());
    log.info("  value: {}", tx.getValue().toHexString());
    log.info("  data:  {}B", tx.getPayload().size());
    log.info(" -- Gas info ---");
    log.info("  gas limit:              {}", tx.getGasLimit());
    log.info(
        "  gas price:              {}", tx.getGasPrice().map(Quantity::toHexString).orElse("N/A"));
    log.info(
        "  max. fee per gas:       {}",
        tx.getMaxFeePerGas().map(Quantity::toHexString).orElse("N/A"));
    log.info(
        "  max. prio. fee per gas: {}",
        tx.getMaxPriorityFeePerGas().map(Quantity::toHexString).orElse("N/A"));
  }

  public void traceEndTx(
      WorldView worldView,
      Transaction tx,
      boolean isSuccessful,
      Bytes output,
      List<Log> logs,
      long gasUsed) {
    if (!this.l.doTx()) {
      return;
    }
    log.info("= Ending transaction: {}", isSuccessful ? "SUCCESS" : "FAILURE");
  }

  public void traceContextEnter(final MessageFrame frame) {
    if (!this.l.doContext()) {
      return;
    }
    log.info(
        "--> ID: {} CN: {} {} @ {}",
        hub.currentFrame().id(),
        hub.currentFrame().contextNumber(),
        hub.currentFrame().type(),
        hub.currentFrame().byteCodeAddress().toUnprefixedHexString());
  }

  public void traceContextReEnter(final MessageFrame frame) {
    if (!this.l.doContext()) {
      return;
    }
    log.info(
        "<-> ID: {} CN: {} {} @ {}",
        hub.currentFrame().id(),
        hub.currentFrame().contextNumber(),
        hub.currentFrame().type(),
        hub.currentFrame().byteCodeAddress().toUnprefixedHexString());
  }

  public void traceContextExit(final MessageFrame frame) {
    if (!this.l.doContext()) {
      return;
    }
    log.info(
        "<-- ID: {} CN: {} {} @ {}",
        hub.currentFrame().id(),
        hub.currentFrame().contextNumber(),
        hub.currentFrame().type(),
        hub.currentFrame().byteCodeAddress().toUnprefixedHexString());
  }

  public void tracePreOpcode(final MessageFrame frame) {
    if (!this.l.doOpcode()) {
      return;
    }

    log.info(
        "{}#{} {}",
        Integer.toHexString(hub.currentFrame().pc()),
        hub.currentFrame().id(),
        renderOpCode(hub.opCode(), frame));
  }

  public void tracePostOpcode(final MessageFrame frame, Operation.OperationResult operationResult) {
    if (operationResult.getHaltReason() != null) {
      log.info(
          "{} failed: {}", frame.getCurrentOperation().getName(), operationResult.getHaltReason());
    }

    if (!this.l.doStack()) {
      return;
    }

    final int stackSize = Math.min(MAX_STACK_ELT_DISPLAY, frame.stackSize());
    final StringBuilder s = new StringBuilder(MAX_STACK_ELT_DISPLAY * 33);
    s.append(">>| ");
    for (int i = 0; i < stackSize; i++) {
      s.append(frame.getStackItem(i).toQuantityHexString());
      if (i == stackSize - 1) {
        s.append(" ]");
      } else {
        s.append(" | ");
      }
    }
    log.info("{}", s);
  }

  private static String maybeStackItem(final MessageFrame frame, int i) {
    if (i > frame.stackSize()) {
      return "?";
    }

    return frame.getStackItem(i).toQuantityHexString();
  }

  private static String renderOpCode(final OpCode opCode, final MessageFrame frame) {
    return switch (opCode) {
      case ADD,
          MUL,
          SUB,
          DIV,
          SDIV,
          MOD,
          SMOD,
          EXP,
          SIGNEXTEND,
          LT,
          GT,
          SLT,
          SGT,
          EQ,
          AND,
          OR,
          XOR,
          BYTE -> "%s %s %s".formatted(opCode, maybeStackItem(frame, 0), maybeStackItem(frame, 1));
      case ADDMOD, MULMOD -> "%s %s %s %s"
          .formatted(
              opCode, maybeStackItem(frame, 0), maybeStackItem(frame, 1), maybeStackItem(frame, 2));
      case ISZERO, NOT, CALLDATALOAD, BLOCKHASH, MLOAD, SLOAD -> "%s %s"
          .formatted(opCode, maybeStackItem(frame, 0));
      case SHL -> "SHL %s << %s".formatted(maybeStackItem(frame, 1), maybeStackItem(frame, 0));
      case SHR -> "SHR %s >> %s".formatted(maybeStackItem(frame, 1), maybeStackItem(frame, 0));
      case SAR -> "SAR %s >> %s".formatted(maybeStackItem(frame, 1), maybeStackItem(frame, 0));
      case BALANCE, EXTCODESIZE -> "%s @ %s".formatted(opCode, maybeStackItem(frame, 0));
      case CALLDATACOPY, CODECOPY, RETURNDATACOPY -> "%s [%s ..+ %s] --> %s"
          .formatted(
              opCode, maybeStackItem(frame, 1), maybeStackItem(frame, 2), maybeStackItem(frame, 0));
      case EXTCODECOPY -> "%s [%s ..+ %s] @ %s --> %s"
          .formatted(
              opCode,
              maybeStackItem(frame, 2),
              maybeStackItem(frame, 3),
              maybeStackItem(frame, 0),
              maybeStackItem(frame, 1));
      case EXTCODEHASH -> "EXTCODEHASH @ %s".formatted(maybeStackItem(frame, 0));
      case MSTORE, SSTORE -> "%s %s --> %s"
          .formatted(opCode, maybeStackItem(frame, 1), maybeStackItem(frame, 0));
      case JUMP -> "JUMP %s".formatted(maybeStackItem(frame, 0));
      case JUMPI -> "JUMPI %s ? %s".formatted(maybeStackItem(frame, 1), maybeStackItem(frame, 0));
      case PUSH1,
          PUSH32,
          PUSH31,
          PUSH30,
          PUSH29,
          PUSH28,
          PUSH27,
          PUSH3,
          PUSH4,
          PUSH5,
          PUSH6,
          PUSH7,
          PUSH8,
          PUSH9,
          PUSH10,
          PUSH11,
          PUSH2,
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
          PUSH26 -> {
        final int pushSize = opCode.byteValue() - OpCode.PUSH1.byteValue() + 1;
        int copyStart = frame.getPC() + 1;
        Bytes push;
        if (frame.getCode().getSize() <= copyStart) {
          push = Bytes.EMPTY;
        } else {
          final int copyLength = Math.min(pushSize, frame.getCode().getSize() - frame.getPC() - 1);
          push = frame.getCode().getBytes().slice(copyStart, copyLength);
        }
        yield "%s %s".formatted(opCode, push.toQuantityHexString());
      }
      case DUP1,
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
          DUP16 -> {
        final int stackOffset = opCode.byteValue() - OpCode.DUP1.byteValue();
        yield "%s %s".formatted(opCode, maybeStackItem(frame, stackOffset));
      }
      case SWAP1,
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
          SWAP16 -> {
        final int depth = opCode.byteValue() - OpCode.SWAP1.byteValue() + 1;
        yield "%s %s <--> %s"
            .formatted(opCode, maybeStackItem(frame, 0), maybeStackItem(frame, depth));
      }
      case LOG0, LOG1, LOG2, LOG3, LOG4 -> {
        final int topicCount = opCode.byteValue() - OpCode.LOG0.byteValue();
        final StringBuilder s = new StringBuilder(100);
        s.append(
            "%s %s ..+ %s".formatted(opCode, maybeStackItem(frame, 0), maybeStackItem(frame, 1)));
        for (int i = 0; i < topicCount; i++) {
          s.append(" #");
          s.append(maybeStackItem(frame, i + 2));
        }
        yield s.toString();
      }
      case CALL, CALLCODE -> "%s @%s gas: %s value: %s IN [%s ..+ %s]  OUT [%s ..+ %s]"
          .formatted(
              opCode,
              maybeStackItem(frame, 1),
              maybeStackItem(frame, 0),
              maybeStackItem(frame, 2),
              maybeStackItem(frame, 3),
              maybeStackItem(frame, 4),
              maybeStackItem(frame, 5),
              maybeStackItem(frame, 6));
      case RETURN, REVERT, SHA3 -> "RETURN [%s ..+ %s]"
          .formatted(maybeStackItem(frame, 0), maybeStackItem(frame, 1));
      case DELEGATECALL, STATICCALL -> "%s @%s gas: %s  IN [%s ..+ %s]  OUT [%s ..+ %s]"
          .formatted(
              opCode,
              maybeStackItem(frame, 1),
              maybeStackItem(frame, 0),
              maybeStackItem(frame, 2),
              maybeStackItem(frame, 3),
              maybeStackItem(frame, 4),
              maybeStackItem(frame, 5));
      case CREATE -> "CREATE [%s ..+ %s] value: %s"
          .formatted(maybeStackItem(frame, 1), maybeStackItem(frame, 2), maybeStackItem(frame, 0));
      case CREATE2 -> "CREATE [%s ..+ %s] value: %s salt: %s"
          .formatted(
              maybeStackItem(frame, 1),
              maybeStackItem(frame, 2),
              maybeStackItem(frame, 0),
              maybeStackItem(frame, 3));
      default -> opCode.toString();
    };
  }
}
