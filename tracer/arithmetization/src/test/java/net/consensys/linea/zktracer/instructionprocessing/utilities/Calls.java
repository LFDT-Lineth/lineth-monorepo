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
package net.consensys.linea.zktracer.instructionprocessing.utilities;

import static com.google.common.base.Preconditions.checkArgument;
import static net.consensys.linea.zktracer.opcode.OpCode.*;

import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.zktracer.opcode.OpCode;
import net.consensys.linea.zktracer.opcode.OpCodeData;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.datatypes.Address;

public class Calls {

  public static final String fullEoaAddress = "000000000000000000000000abcdef0123456789";
  public static final String toTrim12 = "aaaaaaaaaaaaaaaaaaaaaaaa";
  public static final String untrimmedEoaAddress = toTrim12 + fullEoaAddress;
  public static final String eoaAddress = "c0ffeef00d";
  public static final String eoaAddress2 = "badbeef";

  public static Bytes32 randRao =
      Bytes32.fromHexString("0b03478988fb194f3ddd922bbc4e9fb415fbdb99818f88186ccfa206337b023d");
  public static Bytes32 randCdo =
      Bytes32.fromHexString("1a3b88fc78471a5d0ce2df8a5799299b7eefd8e6bfd6d6afb0e437e0a6311878");

  public static void appendFullGasCall(
      BytecodeCompiler program,
      OpCodeData callOpcode,
      Address to,
      int value,
      int cdo,
      int cds,
      int rao,
      int rac) {
    program.push(rac).push(rao).push(cds).push(cdo);
    if (callOpcode.callHasValueArgument()) {
      program.push(value);
    }
    program.push(to).op(GAS).op(callOpcode.mnemonic());
  }

  public static void fullBalanceCall(
      BytecodeCompiler program, OpCode callOpcode, Address to, int cdo, int cds, int rao, int rac) {
    program.push(rac).push(rao).push(cds).push(cdo);
    if (program.opCodeData(callOpcode).callHasValueArgument()) {
      program.op(BALANCE);
    }
    program.push(to).op(GAS).op(callOpcode);
  }

  public static void appendRevert(BytecodeCompiler program, int rdo, int rds) {
    program.push(rds).push(rdo).op(REVERT);
  }

  public static void appendCall(
      BytecodeCompiler program,
      OpCode callOpcode,
      int gas,
      Address to,
      int value,
      int cdo,
      int cds,
      int rao,
      int rac) {
    program.push(rac).push(rao).push(cds).push(cdo);
    if (program.opCodeData(callOpcode).callHasValueArgument()) {
      program.push(value);
    }
    program.push(to).push(gas).op(callOpcode);
  }

  public static void appendExtremalCall(
      BytecodeCompiler program,
      OpCode callOpcode,
      int gas,
      ToyAccount toAccount,
      int value,
      boolean emptyCallData,
      boolean emptyReturnAt) {

    // return at parameters
    if (emptyReturnAt) {
      program.push(0).push(randRao);
    } else {
      program.push(256).push(257);
    }

    // call data parameters
    if (emptyCallData) {
      program.push(0).push(randCdo);
    } else {
      program.push(258).push(259);
    }

    if (program.opCodeData(callOpcode).callHasValueArgument()) {
      program.push(value);
    }
    program.push(toAccount.getAddress()).push(gas).op(callOpcode);
  }

  public static void appendInsufficientBalanceCall(
      BytecodeCompiler program,
      OpCode callOpcode,
      int gas,
      Address to,
      int cdo,
      int cds,
      int rao,
      int rac) {
    OpCodeData callInfo = program.opCodeData(callOpcode);
    checkArgument(callInfo.callHasValueArgument());
    program
        .push(rac)
        .push(rao)
        .push(cds)
        .push(cdo)
        .op(BALANCE)
        .push(1)
        .op(ADD) // puts balance + 1 on the stack
        .push(to)
        .push(gas)
        .op(callOpcode);
  }

  public static void appendRecursiveSelfCall(BytecodeCompiler program, OpCode callOpCode) {
    OpCodeData callInfo = program.opCodeData(callOpCode);
    checkArgument(callInfo.isCall());
    program.push(0).push(0).push(0).push(0);
    if (callInfo.callHasValueArgument()) {
      program.push("1000"); // value
    }
    program
        .op(ADDRESS) // current address
        .op(GAS) // providing all available gas
        .op(callOpCode); // self-call
  }

  /**
   * Pushing, in order: h, v, r, s; values produce a valid signature.
   *
   * @param program
   */
  public static void validEcrecoverData(BytecodeCompiler program) {
    program
        .push("279d94621558f755796898fc4bd36b6d407cae77537865afe523b79c74cc680b")
        .push(0)
        .op(MSTORE)
        .push("1b")
        .push(32)
        .op(MSTORE)
        .push("c2ff96feed8749a5ad1c0714f950b5ac939d8acedbedcbc2949614ab8af06312")
        .push(64)
        .op(MSTORE)
        .push("1feecd50adc6273fdd5d11c6da18c8cfe14e2787f5a90af7c7c1328e7d0a2c42")
        .push(96)
        .op(MSTORE);
  }

  public static void appendGibberishReturn(BytecodeCompiler program) {
    program.op(CALLER).op(EXTCODEHASH).op(DUP1);
    program.push(1).push(0).op(SUB); // writes 0xffff...ff onto the stack
    program.op(XOR);
    program.push(11).op(MSTORE);
    program.push(50).op(MSTORE);
    program.push(77).push(3).op(RETURN); // returning some of that with zeros at the start
  }

  public static class ProgramIncrement {

    public final BytecodeCompiler program;
    public final int initialSize;

    public ProgramIncrement(BytecodeCompiler program) {
      this.program = program;
      this.initialSize = program.compile().size();
    }

    public int sizeDelta() {
      return program.compile().size() - initialSize;
    }
  }
}
