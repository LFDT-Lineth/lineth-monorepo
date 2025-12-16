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

package net.consensys.linea.zktracer.module.blsdata;

import static net.consensys.linea.zktracer.Fork.isPostCancun;
import static net.consensys.linea.zktracer.module.blsdata.BlsDataOperation.POINT_EVALUATION_PRIME;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.math.BigInteger;
import java.util.List;
import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.BytecodeRunner;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.evm.precompile.KZGPointEvalPrecompiledContract;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;
import org.junit.platform.commons.util.Preconditions;

@ExtendWith(UnitTestWatcher.class)
public class PointEvaluationTest extends TracerTestBase {

  @BeforeAll
  static void setup() {
    // Initialize KZG native library before running tests
    KZGPointEvalPrecompiledContract.init();
  }

  @Test
  void validInputTest(TestInfo testInfo) {
    // source:
    // https://github.com/ethereum/execution-spec-tests/blob/1983444bbe1a471886ef7c0e82253ffe2a4053e1/tests/cancun/eip4844_blobs/point_evaluation_vectors/go_kzg_4844_verify_kzg_proof.json#L312-L321 and Ivo
    BytecodeRunner bytecodeRunner =
        pointEvaluationProgram(
            "010657f37554c781402a22917dee2f75def7ab966d7b770905398eba3c444014",
            "0000000000000000000000000000000000000000000000000000000000000000",
            "0000000000000000000000000000000000000000000000000000000000000000",
            "c00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
            "c00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
            testInfo);
    if (isPostCancun(fork)) {
      final BlsData blsdata = (BlsData) bytecodeRunner.getHub().blsData();
      assertFalse(blsdata.blsDataOperation().malformedDataInternal());
      assertFalse(blsdata.blsDataOperation().malformedDataExternal());
      assertTrue(blsdata.blsDataOperation().successBit());
    }
  }

  @Test
  void mintDueToZNotInRangeTest(TestInfo testInfo) {
    BytecodeRunner bytecodeRunner =
        pointEvaluationProgram(
            "010657f37554c781402a22917dee2f75def7ab966d7b770905398eba3c444014",
            (POINT_EVALUATION_PRIME.add(1)).toHexString().substring(2),
            "0000000000000000000000000000000000000000000000000000000000000000",
            "c00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
            "c00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
            testInfo);
    if (isPostCancun(fork)) {
      final BlsData blsdata = (BlsData) bytecodeRunner.getHub().blsData();
      assertTrue(blsdata.blsDataOperation().malformedDataInternal());
      assertFalse(blsdata.blsDataOperation().malformedDataExternal());
      assertFalse(blsdata.blsDataOperation().successBit());
    }
  }

  @Test
  void mintDueToYNotInRangeTest(TestInfo testInfo) {
    BytecodeRunner bytecodeRunner =
        pointEvaluationProgram(
            "010657f37554c781402a22917dee2f75def7ab966d7b770905398eba3c444014",
            "0000000000000000000000000000000000000000000000000000000000000000",
            (POINT_EVALUATION_PRIME.add(1)).toHexString().substring(2),
            "c00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
            "c00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
            testInfo);
    if (isPostCancun(fork)) {
      final BlsData blsdata = (BlsData) bytecodeRunner.getHub().blsData();
      assertTrue(blsdata.blsDataOperation().malformedDataInternal());
      assertFalse(blsdata.blsDataOperation().malformedDataExternal());
      assertFalse(blsdata.blsDataOperation().successBit());
    }
  }

  @Test
  void mintDueToZAndYNotInRangeTest(TestInfo testInfo) {
    BytecodeRunner bytecodeRunner =
        pointEvaluationProgram(
            "010657f37554c781402a22917dee2f75def7ab966d7b770905398eba3c444014",
            (POINT_EVALUATION_PRIME.add(1)).toHexString().substring(2),
            (POINT_EVALUATION_PRIME.add(1)).toHexString().substring(2),
            "c00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
            "c00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
            testInfo);
    if (isPostCancun(fork)) {
      final BlsData blsdata = (BlsData) bytecodeRunner.getHub().blsData();
      assertTrue(blsdata.blsDataOperation().malformedDataInternal());
      assertFalse(blsdata.blsDataOperation().malformedDataExternal());
      assertFalse(blsdata.blsDataOperation().successBit());
    }
  }

  @Test
  void mextTest(TestInfo testInfo) {
    final String z = "65cec67f404f8c81ef4ff3b08dc93a7f643c84e31d2cf39d094e9fbfcab15c9a";
    final String y = "00371c441de8235d2d858ade58d833ac6c5c9460fd369aea0c9918b6d007ed47";
    Preconditions.condition(
        (new BigInteger(z, 16))
                .compareTo(new BigInteger(POINT_EVALUATION_PRIME.toHexString().substring(2), 16))
            < 0,
        "z not in range");
    Preconditions.condition(
        (new BigInteger(y, 16))
                .compareTo(new BigInteger(POINT_EVALUATION_PRIME.toHexString().substring(2), 16))
            < 0,
        "y not in range");
    BytecodeRunner bytecodeRunner =
        pointEvaluationProgram(
            "0125681886f7d39de0938c4f5d2fb4d94abac545d2c51d242b930c6d667982e4",
            z,
            y,
            "87b470976941e342dba3361216d38797f94a249c89ab8fd29a9512b8cf0be7722eb93ca08dbc33f3bef4b8204f19098e",
            "8870e88b46732c642dc29b2a101fe309285300471f82de8adb40548918b5bcb7e8d9d126a3aa9d80f3559a39baa66a3d",
            testInfo);
    if (isPostCancun(fork)) {
      final BlsData blsdata = (BlsData) bytecodeRunner.getHub().blsData();
      assertFalse(blsdata.blsDataOperation().malformedDataInternal());
      assertTrue(blsdata.blsDataOperation().malformedDataExternal());
      assertFalse(blsdata.blsDataOperation().successBit());
    }
  }

  BytecodeRunner pointEvaluationProgram(
      String versionedHash,
      String z,
      String y,
      String commitment,
      String proof,
      TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    final Address codeOwnerAddress = Address.fromHexString("0xC0DE");
    final ToyAccount codeOwnerAccount =
        ToyAccount.builder()
            .balance(Wei.of(0))
            .nonce(1)
            .address(codeOwnerAddress)
            .code(Bytes.fromHexString(versionedHash + z + y + commitment + proof))
            .build();

    // First place the parameters in memory
    // Copy to targetOffset the code of codeOwnerAccount
    program
        .push(codeOwnerAddress)
        .op(OpCode.EXTCODESIZE) // size
        .push(0) // offset
        .push(0) // targetOffset
        .push(codeOwnerAddress) // address
        .op(OpCode.EXTCODECOPY);

    // Do the call
    program
        .push(64) // retSize
        .push(256) // retOffset
        .push(192) // argSize
        .push(0) // argOffset
        .push(10) // address
        .push(Bytes.fromHexStringLenient("0xFFFFFFFF")) // gas
        .op(OpCode.STATICCALL);
    final BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(List.of(codeOwnerAccount), chainConfig, testInfo);
    return bytecodeRunner;
  }
}
