/*
 * Copyright ConsenSys Inc.
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

package net.consensys.linea.zktracer;

import static net.consensys.linea.reporting.TracerTestBase.chainConfig;
import static net.consensys.linea.testing.BytecodeCompiler.newProgram;

import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Address;

public class Utils {
  public static final Bytes REVERT =
      newProgram(chainConfig).push(0).push(0).op(OpCode.REVERT).compile();

  public static final Bytes POPULATE_MEMORY =
      newProgram(chainConfig)
          .op(OpCode.CALLDATASIZE) // size
          .push(0) // offset
          .push(0) // dest offset
          .op(OpCode.CALLDATACOPY)
          .compile();

  public static Bytes call(Address address, boolean staticCall) {
    return newProgram(chainConfig)
        .immediate(POPULATE_MEMORY)
        .push(0) // retSize
        .push(0) // retOffset
        .op(OpCode.MSIZE) // arg size
        .push(0) // argOffset
        .push(1) // value
        .push(address) // address
        .push(100000) // gas
        .op(staticCall ? OpCode.STATICCALL : OpCode.CALL)
        .compile();
  }

  public static Bytes delegateCall(Address address) {
    return newProgram(chainConfig)
        .immediate(POPULATE_MEMORY)
        .push(0) // retSize
        .push(0) // retOffset
        .op(OpCode.MSIZE) // arg size
        .push(0) // argOffset
        .push(1) // value
        .push(address) // address
        .push(100000) // gas
        .op(OpCode.DELEGATECALL)
        .compile();
  }
}
