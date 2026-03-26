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
package net.consensys.linea.zktracer.instructionprocessing.selfdestructTests;

import static net.consensys.linea.zktracer.instructionprocessing.utilities.Calls.eoaAddress;
import static net.consensys.linea.zktracer.instructionprocessing.utilities.SelfDestructs.createValueFromContextParameters;
import static net.consensys.linea.zktracer.opcode.OpCode.*;
import static net.consensys.linea.zktracer.opcode.OpCode.SELFDESTRUCT;

import java.util.Optional;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.zktracer.ChainConfig;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Wei;

public enum Heir {
  HEIR_IS_ZERO,
  HEIR_IS_ORIGIN,
  HEIR_IS_CALLER,
  HEIR_IS_SELF,
  HEIR_IS_EOA,
  HEIR_IS_ECREC,
  HEIR_IS_COMPUTED;

  public static Address selfDestructorAddress = Address.fromHexString("0xFFc0deadd7");

  public static ToyAccount basicSelfDestructor(
      Heir heir, Optional<Address> selfDestructAddress, ChainConfig chainConfig) {

    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    switch (heir) {
      case HEIR_IS_ZERO:
        program.push(0);
      case HEIR_IS_ORIGIN:
        program.op(ORIGIN);
      case HEIR_IS_CALLER:
        program.op(CALLER);
      case HEIR_IS_SELF:
        program.op(ADDRESS);
      case HEIR_IS_EOA:
        program.push(eoaAddress);
      case HEIR_IS_ECREC:
        program.push(Address.ECREC.getBytes());
      case HEIR_IS_COMPUTED:
        createValueFromContextParameters(program);
    }
    program.op(SELFDESTRUCT);

    return ToyAccount.builder()
        .address(selfDestructAddress.orElse(selfDestructorAddress))
        .code(program.compile())
        .balance(Wei.fromEth(1))
        .nonce(1776)
        .build();
  }
}
