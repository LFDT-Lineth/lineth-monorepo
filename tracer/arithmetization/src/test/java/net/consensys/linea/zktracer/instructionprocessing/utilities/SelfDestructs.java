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

import static net.consensys.linea.zktracer.opcode.OpCode.*;
import static org.hyperledger.besu.datatypes.Address.ECREC;

import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.zktracer.ChainConfig;
import net.consensys.linea.zktracer.opcode.OpCode;

public class SelfDestructs {

  /**
   * The zero selfDestructorAddress will likely be cold.
   *
   * @param program
   */
  public static void zeroRecipientSelfDestruct(BytecodeCompiler program) {
    program.push(0).op(OpCode.SELFDESTRUCT);
  }

  public static BytecodeCompiler seldestructWithRecipientLoadedFromStorage(
      BytecodeCompiler program) {
    return program
        .push(0)
        .op(OpCode.SLOAD) // value will be interpreted as recipient; recipient will be cold;
        .op(OpCode.SELFDESTRUCT);
  }

  // will have to be tested in conjunction with DELEGATECALL and CALLCODE
  public static BytecodeCompiler selfReferentialSelfDestruct(BytecodeCompiler program) {
    return program
        .op(ADDRESS) // one self, thus already warm
        .op(OpCode.SELFDESTRUCT);
  }

  // will have to be tested in conjunction with DELEGATECALL and CALLCODE
  public static BytecodeCompiler recipientIsCallerSelfDestruct(BytecodeCompiler program) {
    return program
        .op(CALLER) // warm caller;
        .op(OpCode.SELFDESTRUCT);
  }

  // will have to be tested in conjunction with DELEGATECALL and CALLCODE
  public static BytecodeCompiler recipientIsOriginSelfDestruct(BytecodeCompiler program) {
    return program
        .op(ORIGIN) // warm origin;
        .op(OpCode.SELFDESTRUCT);
  }

  // will have to be tested in conjunction with DELEGATECALL and CALLCODE
  public static int recipientIsPrecompileSelfDestruct(BytecodeCompiler program) {
    Calls.ProgramIncrement increment = new Calls.ProgramIncrement(program);
    program
        .push(ECREC.getBytes()) // precompiles are warm by default
        .op(OpCode.SELFDESTRUCT);
    return increment.sizeDelta();
  }

  public static int createValueFromContextParameters(BytecodeCompiler program) {
    Calls.ProgramIncrement increment = new Calls.ProgramIncrement(program);

    program
        .push(256)
        .op(CALLDATASIZE)
        .push(5003)
        .op(ADD)
        .op(CALLVALUE)
        .push(1789)
        .op(ADD)
        .op(MUL)
        .op(MOD);

    return increment.sizeDelta();
  }

  public static BytecodeCompiler storageTouchingSelfDestructorRewardsZeroAddress(
      ChainConfig chainConfig) {

    BytecodeCompiler selfDestructor = BytecodeCompiler.newProgram(chainConfig);
    loadAndStoreValues(selfDestructor);
    // selfDestructWithZeroRecipient(selfDestructor);

    return selfDestructor;
  }

  public static BytecodeCompiler variableRecipientStorageTouchingSelfDestructor(
      ChainConfig chainConfig) {

    BytecodeCompiler selfDestructor = BytecodeCompiler.newProgram(chainConfig);
    loadAndStoreValues(selfDestructor);
    seldestructWithRecipientLoadedFromStorage(selfDestructor);

    return selfDestructor;
  }

  /**
   * The following code snippet
   *
   * <p>- computes a value from context parameters and duplicates it
   *
   * <p>- loads a values from storage slot 0, adds the previous value to it and stores the result in
   * storage slot 0
   *
   * <p>- does the same for storage slot 1 (but adds 1 to the result) uses that value to over write
   * these values in storage.
   *
   * @return code
   */
  public static void loadAndStoreValues(BytecodeCompiler loadAndOverWriteValuesInStorage) {

    SelfDestructs.createValueFromContextParameters(loadAndOverWriteValuesInStorage);
    loadAndOverWriteValuesInStorage.op(DUP1);
    loadAndOverWriteValuesInStorage.push(0);
    loadAndOverWriteValuesInStorage.op(SLOAD); // load σ[acc]_s[0]
    loadAndOverWriteValuesInStorage.op(ADD); // adding computed value to current storage value
    loadAndOverWriteValuesInStorage.push(0).op(SSTORE); // overwriting storage valueσ σ[acc]_s[0]
    loadAndOverWriteValuesInStorage.push(1);
    loadAndOverWriteValuesInStorage.op(SLOAD); // load σ[acc]_s[1]
    loadAndOverWriteValuesInStorage.op(ADD); // adding computed value to current storage value
    loadAndOverWriteValuesInStorage.push(1).op(ADD);
    loadAndOverWriteValuesInStorage.push(1).op(SSTORE); // overwriting storage valueσ σ[acc]_s[1]
  }
}
