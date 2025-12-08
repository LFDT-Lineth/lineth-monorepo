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

import static net.consensys.linea.zktracer.instructionprocessing.utilities.Calls.*;
import static net.consensys.linea.zktracer.opcode.OpCode.*;

import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.zktracer.ChainConfig;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Wei;

public class MultiOpCodeSmcs {

  /**
   * Produces a program that triggers all opcodes from the CONTEXT instruction family.
   *
   * @return
   */
  public static BytecodeCompiler allContextOpCodes(ChainConfig chainConfig) {

    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    program
        .op(ADDRESS)
        .op(CALLDATASIZE)
        .op(RETURNDATASIZE) // will return 0, but will be tested in the caller
        .op(CALLER)
        .op(CALLVALUE);

    // producing some gibberish return data
    appendGibberishReturn(program);

    return program;
  }

  public static ToyAccount allContextOpCodesSmc(ChainConfig chainConfig) {
    return ToyAccount.builder()
        .balance(Wei.fromEth(9))
        .nonce(13)
        .address(Address.fromHexString("c0de"))
        .code(allContextOpCodes(chainConfig).compile())
        .build();
  }
}
