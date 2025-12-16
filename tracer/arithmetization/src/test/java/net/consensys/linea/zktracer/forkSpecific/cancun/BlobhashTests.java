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

package net.consensys.linea.zktracer.forkSpecific.cancun;

import static net.consensys.linea.zktracer.Trace.LINEA_BLOB_PER_TRANSACTION_MAXIMUM;

import java.util.ArrayList;
import java.util.List;
import java.util.stream.Stream;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeRunner;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.MethodSource;

public class BlobhashTests extends TracerTestBase {
  public static Stream<Arguments> stackArgument() {
    final List<Arguments> arguments = new ArrayList<>();
    for (int stackArgument = 0;
        stackArgument <= LINEA_BLOB_PER_TRANSACTION_MAXIMUM + 2;
        stackArgument++) {
      arguments.add(Arguments.of(Bytes32.leftPad(Bytes.ofUnsignedInt(stackArgument))));
    }
    arguments.add(Arguments.of(Bytes32.repeat((byte) 0xFF)));
    return arguments.stream();
  }

  // just run the EVM with the BLOBHASH opcode, and a stack item from 0 to
  // LINEA_BLOB_PER_TRANSACTION_MAXIMUM+2 and a ridiculously large value
  @ParameterizedTest
  @MethodSource("stackArgument")
  void trivialPrevBlobhash(final Bytes32 stackArgument, TestInfo testInfo) {
    BytecodeRunner.of(
            Bytes.concatenate(
                Bytes.fromHexString("0x7F"), // PUSH32
                stackArgument, // the stack argument
                Bytes.fromHexString("49"))) // BLOBHASH
        .run(chainConfig, testInfo);
  }
}
