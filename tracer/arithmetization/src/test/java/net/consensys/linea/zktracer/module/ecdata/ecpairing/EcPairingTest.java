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

package net.consensys.linea.zktracer.module.ecdata.ecpairing;

import static net.consensys.linea.zktracer.module.ecdata.ecpairing.EcPairingTestSupport.argumentsListToPairingsAsString;
import static net.consensys.linea.zktracer.module.ecdata.ecpairing.EcPairingTestSupport.csvToArgumentsList;
import static net.consensys.linea.zktracer.module.ecdata.ecpairing.EcPairingTestSupport.info;
import static net.consensys.linea.zktracer.module.ecdata.ecpairing.EcPairingTestSupport.pair;
import static net.consensys.linea.zktracer.module.ecdata.ecpairing.EcPairingTestSupport.pairingsAsStringToArgumentsList;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;

import java.io.IOException;
import java.nio.file.Paths;
import java.time.LocalDate;
import java.util.ArrayList;
import java.util.Collections;
import java.util.List;
import java.util.Random;
import java.util.stream.Collectors;
import java.util.stream.Stream;
import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.BytecodeRunner;
import net.consensys.linea.zktracer.module.ecdata.EcData;
import net.consensys.linea.zktracer.module.ecdata.EcDataOperation;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Address;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Disabled;
import org.junit.jupiter.api.Tag;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.CsvFileSource;
import org.junit.jupiter.params.provider.MethodSource;

// A TestWatcher is used to log the results of testEcPairingSingleForScenario
// into a csv file (one for successful and one for failing cases)
// that can be used to run the same test cases with @CsvFileSource

@ExtendWith(EcPairingTestWatcher.class)
@ExtendWith(UnitTestWatcher.class)
public class EcPairingTest extends TracerTestBase {
  // https://github.com/Consensys/linea-arithmetization/issues/822

  // SMALL POINTS
  static List<Arguments> smallPointsOutOfRange;
  static List<Arguments> smallPointsNotOnC1;
  static List<Arguments> smallPointsOnC1;
  static final Arguments smallPointInfinity = Arguments.of("0", "0");

  // LARGE POINTS
  static List<Arguments> largePointsOutOfRange;
  static List<Arguments> largePointsNotOnC2;
  static List<Arguments> largePointsNotOnG2;
  static List<Arguments> largePointsOnG2;
  static final Arguments largePointInfinity = Arguments.of("0", "0", "0", "0");

  static List<Arguments> smallPoints;
  static List<Arguments> largePoints;

  @BeforeAll
  public static void initConcatenatedLists(TestInfo testInfo) {
    try {
      String basePath = "../arithmetization/src/test/resources/ecpairing/";
      smallPointsOutOfRange =
          csvToArgumentsList(Paths.get(basePath, "small_points_out_of_range.csv").toString());
      smallPointsNotOnC1 =
          csvToArgumentsList(Paths.get(basePath, "small_points_not_on_c1.csv").toString());
      smallPointsOnC1 =
          csvToArgumentsList(Paths.get(basePath, "small_points_on_c1.csv").toString());

      largePointsOutOfRange =
          csvToArgumentsList(Paths.get(basePath, "large_points_out_of_range.csv").toString());
      largePointsNotOnC2 =
          csvToArgumentsList(Paths.get(basePath, "large_points_not_on_c2.csv").toString());
      largePointsNotOnG2 =
          csvToArgumentsList(Paths.get(basePath, "large_points_not_on_g2.csv").toString());
      largePointsOnG2 =
          csvToArgumentsList(Paths.get(basePath, "large_points_on_g2.csv").toString());
    } catch (IOException e) {
      throw new RuntimeException("Failed to read CSV files", e);
    }

    smallPoints =
        Stream.of(
                smallPointsOutOfRange,
                smallPointsNotOnC1,
                smallPointsOnC1,
                List.of(smallPointInfinity))
            .flatMap(List::stream)
            .collect(Collectors.toList());
    largePoints =
        Stream.of(
                largePointsOutOfRange,
                largePointsNotOnC2,
                largePointsNotOnG2,
                largePointsOnG2,
                List.of(largePointInfinity))
            .flatMap(List::stream)
            .collect(Collectors.toList());
  }

  // Tests
  @Test
  void testEcPairingWithSingleTrivialPairing(TestInfo testInfo) {
    BytecodeCompiler program =
        BytecodeCompiler.newProgram(chainConfig)
            .push(0x20) // retSize
            .push(0) // retOffset
            .push(192) // argSize
            .push(0) // argOffset
            .push(Address.ALTBN128_PAIRING) // address
            .push(Bytes.fromHexStringLenient("0xFFFFFFFF")) // gas
            .op(OpCode.STATICCALL);

    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    assertLineCount(bytecodeRunner);
  }

  @Test
  void testEcPairingWithSingleNonTrivialValidPairing(TestInfo testInfo) {
    // small point: (Ax,Ay)
    // large point: (BxRe + i*BxIm, ByRe + i*ByIm)
    BytecodeCompiler program =
        BytecodeCompiler.newProgram(chainConfig)
            // random point in C1
            .push("26d7d8759964ac70b4d5cdf698ad5f70da246752481ea37da637551a60a2a57f") // Ax
            .push(0)
            .op(OpCode.MSTORE)
            .push("13991eda70bd3bd91c43e7c93f8f89c949bd271ba8f9f5a38bce7248f1f6056b") // Ay
            .push(0x20)
            .op(OpCode.MSTORE)
            // random point in G2
            .push("13eb8555f322c3885846df8a505693a0012493a30c34196a529f964a684c0cb2") // BxIm
            .push(0x40)
            .op(OpCode.MSTORE)
            .push("18335a998f3db61d3d0b63cd1371789a9a8a5ed4fb1a4adaa20ab9573251a9d0") // BxRe
            .push(0x60)
            .op(OpCode.MSTORE)
            .push("20494259608bfb4cd04716ba62e1350ce90d00d8af00a5170f46f59ae72d060c") // ByIm
            .push(0x80)
            .op(OpCode.MSTORE)
            .push("257a64fbc5a9cf9c3f7be349d09efa099305fe61f364c31c082ef6a81c815c1d") // ByRe
            .push(0xA0)
            .op(OpCode.MSTORE)
            // Do the call
            .push(0x20) // retSize
            .push(0) // retOffset
            .push(192) // argSize
            .push(0) // argOffset
            .push(8) // address
            .push(Bytes.fromHexStringLenient("0xFFFFFFFF")) // gas
            .op(OpCode.STATICCALL);

    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    assertLineCount(bytecodeRunner);
  }

  @ParameterizedTest
  @MethodSource("ecPairingSingleSource")
  void testEcPairingSingleForScenarioUsingMethodSource(
      String Ax, String Ay, String BxIm, String BxRe, String ByIm, String ByRe, TestInfo testInfo) {
    testEcPairingSingleForScenarioBody(Ax, Ay, BxIm, BxRe, ByIm, ByRe, testInfo);
  }

  @Tag("nightly")
  @ParameterizedTest
  @MethodSource("ecPairingSingleSourceNightly")
  void testEcPairingSingleForScenarioUsingMethodSourceNightly(
      String Ax, String Ay, String BxIm, String BxRe, String ByIm, String ByRe, TestInfo testInfo) {
    testEcPairingSingleForScenarioBody(Ax, Ay, BxIm, BxRe, ByIm, ByRe, testInfo);
  }

  @Disabled // Useful only to test specific test cases in a CSV file
  @ParameterizedTest
  @CsvFileSource(
      resources =
          "/ecpairing/test_ec_pairing_single_for_scenario_using_method_source_failed_placeholder.csv")
  void testEcPairingSingleForScenarioUsingCsvNightly(
      String Ax, String Ay, String BxIm, String BxRe, String ByIm, String ByRe, TestInfo testInfo) {
    testEcPairingSingleForScenarioBody(Ax, Ay, BxIm, BxRe, ByIm, ByRe, testInfo);
  }

  // Body of:
  // testEcPairingSingleForScenarioUsingMethodSource
  // testEcPairingSingleForScenarioUsingCsv
  private static void testEcPairingSingleForScenarioBody(
      String Ax, String Ay, String BxIm, String BxRe, String ByIm, String ByRe, TestInfo testInfo) {
    // small point: (Ax,Ay)
    // large point: (BxRe + i*BxIm, ByRe + i*ByIm)
    BytecodeCompiler program =
        BytecodeCompiler.newProgram(chainConfig)
            // point supposed to be in C1
            .push(Ax) // Ax
            .push(0)
            .op(OpCode.MSTORE)
            .push(Ay) // Ay
            .push(0x20)
            .op(OpCode.MSTORE)
            // point supposed to be in G2
            .push(BxIm) // BxIm
            .push(0x40)
            .op(OpCode.MSTORE)
            .push(BxRe) // BxRe
            .push(0x60)
            .op(OpCode.MSTORE)
            .push(ByIm) // ByIm
            .push(0x80)
            .op(OpCode.MSTORE)
            .push(ByRe) // ByRe
            .push(0xA0)
            .op(OpCode.MSTORE)
            .push(0x20) // retSize
            .push(0) // retOffset
            .push(192) // argSize
            .push(0) // argOffset
            .push(8) // address
            .push(Bytes.fromHexStringLenient("0xFFFFFFFF")) // gas
            .op(OpCode.STATICCALL);

    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    assertLineCount(bytecodeRunner);

    // Set arguments for TestWatcher
    EcPairingArgumentsSingleton.getInstance()
        .setArguments(Ax + "," + Ay + "," + BxIm + "," + BxRe + "," + ByIm + "," + ByRe);
  }

  // Method source of testEcPairingSingleForScenarioUsingMethodSource
  private static Stream<Arguments> ecPairingSingleSource() {
    List<Arguments> arguments = new ArrayList<>(ecPairingSingleSourceNightly().toList());
    Collections.shuffle(arguments, new Random(LocalDate.now().toEpochDay()));
    return arguments.stream().limit(arguments.size() / 20); // Execute 5 % of the tests
  }

  private static Stream<Arguments> ecPairingSingleSourceNightly() {
    List<Arguments> arguments = new ArrayList<>();
    for (Arguments smallPoint : smallPoints) {
      for (Arguments largePoint : largePoints) {
        arguments.add(pair(smallPoint, largePoint));
      }
    }
    return arguments.stream();
  }

  @ParameterizedTest
  @MethodSource({"ecPairingGenericSource", "ecPairingSuccessfulNonTrivialSource"})
  void testEcPairingGenericForScenarioUsingMethodSource(
      String description, String pairingsAsString, TestInfo testInfo) {
    testEcPairingGenericForScenarioBody(description, pairingsAsString, testInfo);
  }

  @Tag("nightly")
  @ParameterizedTest
  @MethodSource({"ecPairingGenericSourceNightly", "ecPairingSuccessfulNonTrivialSource"})
  void testEcPairingGenericForScenarioUsingMethodSourceNightly(
      String description, String pairingsAsString, TestInfo testInfo) {
    testEcPairingGenericForScenarioBody(description, pairingsAsString, testInfo);
  }

  @Disabled // Useful only to test specific test cases in a CSV file
  @ParameterizedTest
  @CsvFileSource(
      resources =
          "/ecpairing/test_ec_pairing_generic_for_scenario_using_method_source_failed_placeholder.csv",
      maxCharsPerColumn = 100000)
  void testEcPairingGenericForScenarioUsingCsvNightly(
      String description, String pairingsAsString, TestInfo testInfo) {
    testEcPairingGenericForScenarioBody(description, pairingsAsString, testInfo);
  }

  // Body of:
  // testEcPairingGenericForScenarioUsingMethodSource
  // testEcPairingGenericForScenarioUsingCsv
  private void testEcPairingGenericForScenarioBody(
      String description, String pairingsAsString, TestInfo testInfo) {
    assertFalse(description.contains(" "), "Description cannot contain spaces");

    List<Arguments> pairings = pairingsAsStringToArgumentsList(pairingsAsString);

    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    for (int i = 0; i < pairings.size(); i++) {
      Arguments pair = pairings.get(i);

      // Extract the values of the pair
      String Ax = (String) pair.get()[0];
      String Ay = (String) pair.get()[1];
      String BxIm = (String) pair.get()[2];
      String BxRe = (String) pair.get()[3];
      String ByIm = (String) pair.get()[4];
      String ByRe = (String) pair.get()[5];

      // small point: (Ax,Ay)
      // large point: (BxRe + i*BxIm, ByRe + i*ByIm)

      // Add the pair to the program
      final int bytesOffset = 192 * i;

      // point supposed to be in C1
      program
          .push(Ax) // Ax
          .push(bytesOffset)
          .op(OpCode.MSTORE)
          .push(Ay) // Ay
          .push(0x20 + bytesOffset)
          .op(OpCode.MSTORE)
          // point supposed to be in G2
          .push(BxIm) // BxIm
          .push(0x40 + bytesOffset)
          .op(OpCode.MSTORE)
          .push(BxRe) // BxRe
          .push(0x60 + bytesOffset)
          .op(OpCode.MSTORE)
          .push(ByIm) // ByIm
          .push(0x80 + bytesOffset)
          .op(OpCode.MSTORE)
          .push(ByRe) // ByRe
          .push(0xA0 + bytesOffset)
          .op(OpCode.MSTORE);
    }

    // Once all the pairs are added, do the call
    program
        .push(0x20) // retSize
        .push(0) // retOffset
        .push(192 * pairings.size()) // argSize
        .push(0) // argOffset
        .push(8) // address
        .push(Bytes.fromHexStringLenient("0xFFFFFFFF")) // gas
        .op(OpCode.STATICCALL);

    // Run the program
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    // Set arguments for TestWatcher
    EcPairingArgumentsSingleton.getInstance().setArguments(description + "," + pairingsAsString);
  }

  // Method source of testEcPairingGenericForScenarioUsingMethodSource
  private static Stream<Arguments> ecPairingGenericSource() {
    List<Arguments> arguments = new ArrayList<>(ecPairingGenericSourceNightly().toList());
    Collections.shuffle(arguments, new Random(LocalDate.now().toEpochDay()));
    return arguments.stream().limit(arguments.size() / 20); // Execute 5 % of the tests
  }

  private static Stream<Arguments> ecPairingGenericSourceNightly() {
    List<Arguments> allPairings = new ArrayList<>();
    EcPairingTestSupport util = new EcPairingTestSupport();

    final int REPETITIONS = 3;
    for (int r = 1; r <= REPETITIONS; r++) {
      for (Integer totalPairings : List.of(3, 4, 5)) {
        // PRECOMPILE_ECPAIRING_MILLER_LOOPS = 64
        // This test has totalPairings fixed to 64
        allPairings.add(
            util.generatePairings(
                "maxGenericPairings" + info(64, r), 64, smallPoints, largePoints));

        // Focus on small points scenarios
        allPairings.add(
            util.generatePairings(
                "smallPointsOutOfRange" + info(totalPairings, r),
                totalPairings,
                smallPointsOutOfRange,
                largePoints));

        allPairings.add(
            util.generatePairingsSmallPointsMixed(
                "smallPointsOutOfRangeMixed" + info(totalPairings, r),
                totalPairings,
                smallPointsOutOfRange,
                smallPoints,
                largePoints));

        allPairings.add(
            util.generatePairings(
                "smallPointsNotOnC1" + info(totalPairings, r),
                totalPairings,
                smallPointsNotOnC1,
                largePoints));

        allPairings.add(
            util.generatePairingsSmallPointsMixed(
                "smallPointsNotOnC1Mixed" + info(totalPairings, r),
                totalPairings,
                smallPointsNotOnC1,
                smallPoints,
                largePoints));

        allPairings.add(
            util.generatePairings(
                "smallPointsOnC1" + info(totalPairings, r),
                totalPairings,
                smallPointsOnC1,
                largePoints));

        allPairings.add(
            util.generatePairingsSmallPointsMixed(
                "smallPointsOnC1Mixed" + info(totalPairings, r),
                totalPairings,
                smallPointsOnC1,
                smallPoints,
                largePoints));

        allPairings.add(
            util.generatePairings(
                "smallPointInfinity" + info(totalPairings, r),
                totalPairings,
                List.of(smallPointInfinity),
                largePoints));

        allPairings.add(
            util.generatePairingsSmallPointsMixed(
                "smallPointInfinityMixed" + info(totalPairings, r),
                totalPairings,
                List.of(smallPointInfinity),
                smallPoints,
                largePoints));

        // Focus on large points scenarios
        allPairings.add(
            util.generatePairings(
                "largePointsOutOfRange" + info(totalPairings, r),
                totalPairings,
                smallPoints,
                largePointsOutOfRange));

        allPairings.add(
            util.generatePairingsLargePointsMixed(
                "largePointsOutOfRangeMixed" + info(totalPairings, r),
                totalPairings,
                smallPoints,
                largePointsOutOfRange,
                largePoints));

        allPairings.add(
            util.generatePairings(
                "largePointsNotOnC2" + info(totalPairings, r),
                totalPairings,
                smallPoints,
                largePointsNotOnC2));

        allPairings.add(
            util.generatePairingsLargePointsMixed(
                "largePointsNotOnC2Mixed" + info(totalPairings, r),
                totalPairings,
                smallPoints,
                largePointsNotOnC2,
                largePoints));

        allPairings.add(
            util.generatePairings(
                "largePointsNotOnG2" + info(totalPairings, r),
                totalPairings,
                smallPoints,
                largePointsNotOnG2));

        allPairings.add(
            util.generatePairingsLargePointsMixed(
                "largePointsNotOnG2Mixed" + info(totalPairings, r),
                totalPairings,
                smallPoints,
                largePointsNotOnG2,
                largePoints));

        allPairings.add(
            util.generatePairings(
                "largePointsOnG2" + info(totalPairings, r),
                totalPairings,
                smallPoints,
                largePointsOnG2));

        allPairings.add(
            util.generatePairingsLargePointsMixed(
                "largePointsOnG2Mixed" + info(totalPairings, r),
                totalPairings,
                smallPoints,
                largePointsOnG2,
                largePoints));

        allPairings.add(
            util.generatePairings(
                "largePointInfinity" + info(totalPairings, r),
                totalPairings,
                smallPoints,
                List.of(largePointInfinity)));

        allPairings.add(
            util.generatePairingsLargePointsMixed(
                "largePointInfinityMixed" + info(totalPairings, r),
                totalPairings,
                smallPoints,
                List.of(largePointInfinity),
                largePoints));

        // Focus on combinations of small and large points scenarios
        allPairings.add(
            util.generatePairingsLargePointsMixed(
                "largePointNotOnG2Exists" + info(totalPairings, r),
                totalPairings,
                smallPointsOnC1,
                largePointsNotOnG2,
                largePoints));

        allPairings.add(
            util.generatePairings(
                "trivialPairingAllSmallPointsOnC1" + info(totalPairings, r),
                totalPairings,
                smallPointsOnC1,
                List.of(largePointInfinity)));

        allPairings.add(
            util.generatePairingsSmallPointsMixed(
                "trivialPairingSmallPointsOnC1AndInfinity" + info(totalPairings, r),
                totalPairings,
                smallPointsOnC1,
                List.of(smallPointInfinity),
                List.of(largePointInfinity)));

        allPairings.add(
            util.generatePairings(
                "nonTrivialPairingExcludingInfinity" + info(totalPairings, r),
                totalPairings,
                smallPointsOnC1,
                largePointsOnG2));

        allPairings.add(
            util.generatePairingsMixed(
                "nonTrivialPairingIncludingInfinity" + info(totalPairings, r),
                totalPairings,
                smallPointsOnC1,
                List.of(smallPointInfinity),
                largePointsOnG2,
                List.of(largePointInfinity)));
      }
    }
    return allPairings.stream();
  }

  // Method source of testEcPairingGenericForScenarioUsingMethodSource
  // This source is already small so no need to reduce it for non-nightly runs
  private static Stream<Arguments> ecPairingSuccessfulNonTrivialSource() {
    List<Arguments> allPairings = new ArrayList<>();

    // case totalPairings = 2
    List<Arguments> successfulNonTrivialPairingTotalPairings2 = new ArrayList<>();
    successfulNonTrivialPairingTotalPairings2.add(
        Arguments.of(
            "01395d002b3ca9180fb924650ef0656ead838fd027d487fed681de0d674c30da",
            "097c3a9a072f9c85edf7a36812f8ee05e2cc73140749dcd7d29ceb34a8412188",
            "2bd3295ff81c577fe772543783411c36f463676d9692ca4250588fbad0b44dc7",
            "07d8d8329e62324af8091e3a4ffe5a57cb8664d1f5f6838c55261177118e9313",
            "230f1851ba0d3d7d36c8603c7118c86bd2b6a7a1610c4af9e907cb702beff1d8",
            "12843e703009c1c1a2f1088dcf4d91e9ed43189aa6327cae9a68be22a1aee5cb"));
    successfulNonTrivialPairingTotalPairings2.add(
        Arguments.of(
            "05dcb6449ff95e1a04c3132ce3be82a897811d2087e082e0399985449942a45b",
            "0cb5122006e9b7ceb5307fa4015b132b3945bb972c83459f598659fc4b5a9d32",
            "127a664dd11342beb666506dac296731e404de80a25e05f40b2405c4c00c28fc",
            "2bd236cb7a7b0e0543e6b6e0d7308576aeeec4dea2f740654854215d7813826f",
            "1b28f411c2931b52b2ad62de524be4eaac555dfed67d59e2d0f6c4607b23526b",
            "181c4319cc974dd174c5918ac1892326badb2603a04bc8f565221c06eec8a126"));
    allPairings.add(
        Arguments.of(
            "successfulNonTrivialPairingTotalPairings2",
            argumentsListToPairingsAsString(successfulNonTrivialPairingTotalPairings2)));

    // case totalPairings = 3
    List<Arguments> successfulNonTrivialPairingTotalPairings3 = new ArrayList<>();
    successfulNonTrivialPairingTotalPairings3.add(
        Arguments.of(
            "01395d002b3ca9180fb924650ef0656ead838fd027d487fed681de0d674c30da",
            "097c3a9a072f9c85edf7a36812f8ee05e2cc73140749dcd7d29ceb34a8412188",
            "2bd3295ff81c577fe772543783411c36f463676d9692ca4250588fbad0b44dc7",
            "07d8d8329e62324af8091e3a4ffe5a57cb8664d1f5f6838c55261177118e9313",
            "230f1851ba0d3d7d36c8603c7118c86bd2b6a7a1610c4af9e907cb702beff1d8",
            "12843e703009c1c1a2f1088dcf4d91e9ed43189aa6327cae9a68be22a1aee5cb"));

    successfulNonTrivialPairingTotalPairings3.add(
        Arguments.of(
            "05dcb6449ff95e1a04c3132ce3be82a897811d2087e082e0399985449942a45b",
            "0cb5122006e9b7ceb5307fa4015b132b3945bb972c83459f598659fc4b5a9d32",
            "12618811f3e9fa06644d43cfe3f69c6c17a738128de60f8f3ebb4266bab29be6",
            "00a5a6c2ec01c4d1374078ae1bbea91dea8e938c1275226a1ce51db5e7de53d1",
            "2da43ecc11a0095a72454bb08fb4d1116facadcab482a1107ae67a12bb3c19f2",
            "1e2f128bf79945a370324b82c36c1e63509b122c023bd8163495526bb030a216"));

    successfulNonTrivialPairingTotalPairings3.add(
        Arguments.of(
            "1296d042f33ccbb814746e187aa20af49bd503356de4846abee08da9e32ae2ac",
            "0b980019d2af83b353aa8c2efda45f16ce523b99452118be7ae5dd1e92e0e4ec",
            "012cd1b1242354b35cd8d2493c487b52411d111c80e8cfb97080e2db1af5f705",
            "20ca232ed2582feeca2d56a589eec30c27075a44aced8382d87cd43c011aeeb0",
            "2df7d0d9ae47467ca500b528f38ac38433885d6a59db6ecd7d27d37145e75360",
            "1e4d8d4d8878cad4de8dbb31ec11d1ecd7b40617c46bb7189ccab201b9bdbaab"));

    allPairings.add(
        Arguments.of(
            "successfulNonTrivialPairingTotalPairings3",
            argumentsListToPairingsAsString(successfulNonTrivialPairingTotalPairings3)));

    // case totalPairings = 4
    List<Arguments> successfulNonTrivialPairingTotalPairings4 = new ArrayList<>();

    successfulNonTrivialPairingTotalPairings4.add(
        Arguments.of(
            "01395d002b3ca9180fb924650ef0656ead838fd027d487fed681de0d674c30da",
            "097c3a9a072f9c85edf7a36812f8ee05e2cc73140749dcd7d29ceb34a8412188",
            "2bd3295ff81c577fe772543783411c36f463676d9692ca4250588fbad0b44dc7",
            "07d8d8329e62324af8091e3a4ffe5a57cb8664d1f5f6838c55261177118e9313",
            "230f1851ba0d3d7d36c8603c7118c86bd2b6a7a1610c4af9e907cb702beff1d8",
            "12843e703009c1c1a2f1088dcf4d91e9ed43189aa6327cae9a68be22a1aee5cb"));

    successfulNonTrivialPairingTotalPairings4.add(
        Arguments.of(
            "05dcb6449ff95e1a04c3132ce3be82a897811d2087e082e0399985449942a45b",
            "0cb5122006e9b7ceb5307fa4015b132b3945bb972c83459f598659fc4b5a9d32",
            "12618811f3e9fa06644d43cfe3f69c6c17a738128de60f8f3ebb4266bab29be6",
            "00a5a6c2ec01c4d1374078ae1bbea91dea8e938c1275226a1ce51db5e7de53d1",
            "2da43ecc11a0095a72454bb08fb4d1116facadcab482a1107ae67a12bb3c19f2",
            "1e2f128bf79945a370324b82c36c1e63509b122c023bd8163495526bb030a216"));

    successfulNonTrivialPairingTotalPairings4.add(
        Arguments.of(
            "1296d042f33ccbb814746e187aa20af49bd503356de4846abee08da9e32ae2ac",
            "0b980019d2af83b353aa8c2efda45f16ce523b99452118be7ae5dd1e92e0e4ec",
            "0cdc14ed1231209df350fbf71c359fa338955c8e4b10678b33237e54473e7b0f",
            "2c5be308b741fee6607eea4980779483339372c80494a43189ab6613f2ac6b00",
            "0a56a2a107cc154cade228f3da75a714a7de0738b9ebd455f3f73c4c3c43136b",
            "1d98bf24e00f830334bdf3334135c5aa55b6fd4e88e87b1aee72fb1550970879"));

    successfulNonTrivialPairingTotalPairings4.add(
        Arguments.of(
            "0baab17525d29e5d7c34a5cdb6558e8427f5c79206e009b4b1e8b91a4c827f1c",
            "18cb7444434d59d126ccc48244daa9a3709140bfa58bff81751d42b647211a91",
            "27f28ec95a4937c625a4fa28551ba8b7010511313699acb72d5eb0a0a208df2b",
            "2e04f5f992061adf169ef7423bc2f8748ca8fe437036e44a5a24b32bdbae8754",
            "056d39d4c664d7bf3417673a870f94749311be2f4e4c5fdfe6563b1593fcc00e",
            "18b849ea2b045d9b94d2e1d8daa843ef2c98577bb7c98d6004eb8982f561fd0f"));

    allPairings.add(
        Arguments.of(
            "successfulNonTrivialPairingTotalPairings4",
            argumentsListToPairingsAsString(successfulNonTrivialPairingTotalPairings4)));

    return allPairings.stream();
  }

  private static void assertLineCount(BytecodeRunner bytecodeRunner) {
    // Check that the line count is made
    final EcData ecData = bytecodeRunner.getHub().ecData();
    final EcDataOperation ecDataOperation = ecData.operations().get(0);
    final boolean internalChecksPassed = ecDataOperation.internalChecksPassed();
    final int circuitSelectorEcPairingCounter = ecDataOperation.circuitSelectorEcPairingCounter();
    final int circuitSelectorG2MembershipCounter =
        ecDataOperation.circuitSelectorG2MembershipCounter();

    assertEquals(
        circuitSelectorEcPairingCounter > 0 ? 1 : 0,
        bytecodeRunner.getHub().ecPairingFinalExponentiations().lineCount());

    assertEquals(
        internalChecksPassed ? circuitSelectorEcPairingCounter : 0,
        bytecodeRunner.getHub().ecPairingMillerLoops().lineCount());

    assertEquals(
        internalChecksPassed ? circuitSelectorG2MembershipCounter : 0,
        bytecodeRunner.getHub().ecPairingG2MembershipCalls().lineCount());
  }

  // Tests for support methods
  @Tag("nightly")
  @Test
  public void testPair(TestInfo testInfo) {
    Arguments smallPoint = Arguments.of("Ax", "Ay");
    Arguments largePoint = Arguments.of("BxIm", "BxRe", "ByIm", "ByRe");
    Arguments pair = pair(smallPoint, largePoint);
    assertEquals(6, pair.get().length);
    assertEquals("Ax", pair.get()[0]);
    assertEquals("Ay", pair.get()[1]);
    assertEquals("BxIm", pair.get()[2]);
    assertEquals("BxRe", pair.get()[3]);
    assertEquals("ByIm", pair.get()[4]);
    assertEquals("ByRe", pair.get()[5]);
  }

  @Tag("nightly")
  @Test
  public void testArgumentsListToPairingsAsString(TestInfo testInfo) {
    List<Arguments> pairings = new ArrayList<>();
    pairings.add(Arguments.of("Ax1", "Ay1", "BxIm1", "BxRe1", "ByIm1", "ByRe1"));
    pairings.add(Arguments.of("Ax2", "Ay2", "BxIm2", "BxRe2", "ByIm2", "ByRe2"));
    pairings.add(Arguments.of("Ax3", "Ay3", "BxIm3", "BxRe3", "ByIm3", "ByRe3"));
    assertEquals(
        "Ax1_Ay1_BxIm1_BxRe1_ByIm1_ByRe1_Ax2_Ay2_BxIm2_BxRe2_ByIm2_ByRe2_Ax3_Ay3_BxIm3_BxRe3_ByIm3_ByRe3",
        argumentsListToPairingsAsString(pairings));
  }
}
