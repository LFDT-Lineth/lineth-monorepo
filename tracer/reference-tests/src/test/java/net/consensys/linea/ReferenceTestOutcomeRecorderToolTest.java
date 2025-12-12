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
package net.consensys.linea;

import static net.consensys.linea.BlockchainReferenceTestJson.readBlockchainReferenceTestsOutput;
import static net.consensys.linea.ReferenceTestOutcomeRecorderTool.JSON_OUTPUT_FILENAME;
import static org.assertj.core.api.Assertions.assertThat;

import java.io.File;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.concurrent.ConcurrentMap;
import java.util.concurrent.ConcurrentSkipListSet;

import net.consensys.linea.reporting.TestOutcome;
import net.consensys.linea.reporting.TestOutcomeWriterTool;
import net.consensys.linea.reporting.TestState;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

public class ReferenceTestOutcomeRecorderToolTest {

  private final String JSON_OUTPUT_FILENAME_TEST = "JSON_OUTPUT_FILENAME_TEST.json";

  @BeforeEach
  void setup() {
    File outputJsonFile = new File(JSON_OUTPUT_FILENAME);
    if (outputJsonFile.exists()) {
      outputJsonFile.delete();
    }
  }

  @Test
  void multipleModulesAreStoredCorrectly() {
    String module1 = "blockdata";

    String module2 = "txndata";

    List<String> modules = List.of(module1, module2);

    TestOutcomeWriterTool.mapAndStoreTestResult(
        "test1", TestState.FAILED, Map.of("Constraint", Set.of(module1, module2)));
    TestOutcomeWriterTool.mapAndStoreTestResult(
        "test2", TestState.FAILED, Map.of("Constraint", Set.of(module1)));
    TestOutcomeWriterTool.writeToJsonFile(JSON_OUTPUT_FILENAME_TEST);

    readBlockchainReferenceTestsOutput(JSON_OUTPUT_FILENAME_TEST)
        .thenApply(
            jsonString -> {
              TestOutcome blockchainReferenceTestOutcome =
                  TestOutcomeWriterTool.parseTestOutcome(jsonString);

              ConcurrentMap<String, ConcurrentMap<String, ConcurrentSkipListSet<String>>>
                  modulesToConstraints =
                      blockchainReferenceTestOutcome.getModulesToConstraintsToTests();

              assertThat(modulesToConstraints.size()).isEqualTo(modules.size());
              assertThat(modulesToConstraints.keySet()).isEqualTo(Set.of(module1, module2));
              assertThat(modulesToConstraints.get(module1)).isEqualTo(Set.of("test1", "test2"));
              assertThat(modulesToConstraints.get(module2)).isEqualTo(Set.of("test1"));
              return null;
            });
  }

  @Test
  public void extractConstraints() {
    String message =
        "\u001B[1m\u001B[31mrlptxrcpt.phase3\u001B[39m\u001B[0m failed:\n"
            + "\u001B[1m\u001B[97m                        \u001B[39m\u001B[0m16                                                                            17                                                                            \u001B[1m\u001B[31m18  \u001B[39m\u001B[0m19                                                                            20                                                                            \n"
            + "\u001B[1m\u001B[97m ACC_SIZE               \u001B[39m\u001B[0m2                                                                             3                                                                             \u001B[1m\u001B[31m4   \u001B[39m\u001B[0m0                                                                             0                                                                             \n"
            + "\u001B[1m\u001B[97m BIT                    \u001B[39m\u001B[0m1                                                                             1                                                                             \u001B[1m\u001B[31m0   \u001B[39m\u001B[0m0                                                                             0                                                                             \n"
            + "\u001B[1m\u001B[97m C/INV[(.- DONE 1)]     \u001B[39m\u001B[0m8444461749428370424248824938781546531375899335154063827935233455917409239040  8444461749428370424248824938781546531375899335154063827935233455917409239040  \u001B[1m\u001B[31m0   \u001B[39m\u001B[0m0                                                                             8444461749428370424248824938781546531375899335154063827935233455917409239040  \n"
            + "\u001B[1m\u001B[97m C/INV[(.- PHASE_3 1)]  \u001B[39m\u001B[0m0                                                                             0                                                                             \u001B[1m\u001B[31m0   \u001B[39m\u001B[0m8444461749428370424248824938781546531375899335154063827935233455917409239040  8444461749428370424248824938781546531375899335154063827935233455917409239040  \n"
            + "\u001B[1m\u001B[97m DONE                   \u001B[39m\u001B[0m0                                                                             0                                                                             \u001B[1m\u001B[31m1   \u001B[39m\u001B[0m1                                                                             0                                                                             \n"
            + "\u001B[1m\u001B[97m LIMB_CONSTRUCTED       \u001B[39m\u001B[0m0                                                                             0                                                                             \u001B[1m\u001B[31m1   \u001B[39m\u001B[0m1                                                                             0                                                                             \n"
            + "\u001B[1m\u001B[97m PHASE_3                \u001B[39m\u001B[0m1                                                                             1                                                                             \u001B[1m\u001B[31m1   \u001B[39m\u001B[0m0                                                                             0                                                                             \n"
            + "\n"
            + "\u001B[32m(* \u001B[39m\u001B[33m(* \u001B[39m\u001B[34m(- \u001B[39m\u001B[1m\u001B[37m1\u001B[39m\u001B[0m\n"
            + "   \u001B[90mâ”‚\u001B[39m  \u001B[90m(* \u001B[39m\u001B[90m(.- \u001B[39m\u001B[1m\u001B[90mPHASE_3\u001B[39m\u001B[0m\u001B[90m<1>\u001B[39m \u001B[1m\u001B[90m1\u001B[39m\u001B[0m\u001B[90m)\u001B[39m \u001B[1m\u001B[90mC/INV[(.- PHASE_3 1)]\u001B[39m\u001B[0m\u001B[90m<0>\u001B[39m\u001B[90m)\u001B[39m) â†’ \u001B[1m\u001B[90m0\u001B[39m\u001B[0mâ†’ \u001B[1m\u001B[90m0\u001B[39m\u001B[0mâ†’ \u001B[1m\u001B[37m1\u001B[39m\u001B[0m\n"
            + "   \u001B[36m(* \u001B[39m\u001B[91m(- \u001B[39m\u001B[1m\u001B[37m1\u001B[39m\u001B[0m\n"
            + "   \u001B[90mâ”‚\u001B[39m  \u001B[90mâ”‚\u001B[39m  \u001B[90m(* \u001B[39m\u001B[90m(.- \u001B[39m\u001B[1m\u001B[90mDONE\u001B[39m\u001B[0m\u001B[90m<1>\u001B[39m \u001B[1m\u001B[90m1\u001B[39m\u001B[0m\u001B[90m)\u001B[39m \u001B[1m\u001B[90mC/INV[(.- DONE 1)]\u001B[39m\u001B[0m\u001B[90m<0>\u001B[39m\u001B[90m)\u001B[39m) â†’ \u001B[1m\u001B[90m0\u001B[39m\u001B[0mâ†’ \u001B[1m\u001B[90m0\u001B[39m\u001B[0mâ†’ \u001B[1m\u001B[37m1\u001B[39m\u001B[0m\n"
            + "   \u001B[90mâ”‚\u001B[39m  \u001B[93m(+ \u001B[39m\u001B[1m\u001B[37mBIT\u001B[39m\u001B[0m\u001B[37mâ‚‹â‚‡\u001B[39m\u001B[37m<0>\u001B[39m \u001B[93m(- \u001B[39m\u001B[1m\u001B[37mACC_SIZE\u001B[39m\u001B[0m\u001B[37m<4>\u001B[39m \u001B[1m\u001B[37m1\u001B[39m\u001B[0m\u001B[93m)\u001B[39m\u001B[93m)\u001B[39m)) â†’ \u001B[1m\u001B[37m3\u001B[39m\u001B[0mâ†’ \u001B[1m\u001B[37m3\u001B[39m\u001B[0mâ†’ \u001B[1m\u001B[37m3\u001B[39m\u001B[0mâ†’ \u001B[1m\u001B[37m3\u001B[39m\u001B[0m\n"
            + " \u001B[95m(.- \u001B[39m\u001B[90m(+ \u001B[39m\u001B[1m\u001B[90mLIMB_CONSTRUCTED\u001B[39m\u001B[0m\u001B[90mâ‚‹â‚‚\u001B[39m\u001B[90m<0>\u001B[39m \u001B[1m\u001B[90mLIMB_CONSTRUCTED\u001B[39m\u001B[0m\u001B[90mâ‚‹â‚�\u001B[39m\u001B[90m<0>\u001B[39m\u001B[90m)\u001B[39m \u001B[1m\u001B[37m1\u001B[39m\u001B[0m\u001B[95m)\u001B[39m) â†’ \u001B[1m\u001B[90m0\u001B[39m\u001B[0mâ†’ \u001B[1m\u001B[37m8444461749428370424248824938781546531375899335154063827935233455917409239040\u001B[39m\u001B[0mâ†’ \u001B[1m\u001B[31m8444461749428370424248824938781546531375899335154063827935233455917409239038\u001B[39m\u001B[0m\n"
            + "\n"
            + "\u001B[1m\u001B[31mtxndata.cumulative-gas\u001B[39m\u001B[0m failed:\n"
            + "\u001B[1m\u001B[97m                   \u001B[39m\u001B[0m\u001B[1m\u001B[31m0  \u001B[39m\u001B[0m1                     2                     \n"
            + "\u001B[1m\u001B[97m GAS_CUMULATIVE    \u001B[39m\u001B[0m\u001B[1m\u001B[31m0  \u001B[39m\u001B[0m18446744071562092862  18446744071562092862  \n"
            + "\u001B[1m\u001B[97m GAS_LIMIT         \u001B[39m\u001B[0m\u001B[1m\u001B[31m0  \u001B[39m\u001B[0m4294967296            4294967296            \n"
            + "\u001B[1m\u001B[97m REFUND_EFFECTIVE  \u001B[39m\u001B[0m\u001B[1m\u001B[31m0  \u001B[39m\u001B[0m2147458754            2147458754            \n"
            + "\u001B[1m\u001B[97m REL_BLOCK         \u001B[39m\u001B[0m\u001B[1m\u001B[31m0  \u001B[39m\u001B[0m1                     1                     \n"
            + "\n"
            + "\u001B[32m(* \u001B[39m\u001B[33m(.- \u001B[39m\u001B[1m\u001B[37mREL_BLOCK\u001B[39m\u001B[0m\u001B[37mâ‚Š\u001B[39m\u001B[37mâ‚�\u001B[39m\u001B[37m<1>\u001B[39m \u001B[1m\u001B[37mREL_BLOCK\u001B[39m\u001B[0m\u001B[37m<0>\u001B[39m\u001B[33m)\u001B[39m â†’ \u001B[1m\u001B[37m1\u001B[39m\u001B[0m\n"
            + " \u001B[33m(.- \u001B[39m\u001B[1m\u001B[37mGAS_CUMULATIVE\u001B[39m\u001B[0m\u001B[37mâ‚Š\u001B[39m\u001B[37mâ‚�\u001B[39m\u001B[37m<18446744071562092862>\u001B[39m \u001B[33m(- \u001B[39m\u001B[1m\u001B[37mGAS_LIMIT\u001B[39m\u001B[0m\u001B[37mâ‚Š\u001B[39m\u001B[37mâ‚�\u001B[39m\u001B[37m<4294967296>\u001B[39m \u001B[1m\u001B[37mREFUND_EFFECTIVE\u001B[39m\u001B[0m\u001B[37mâ‚Š\u001B[39m\u001B[37mâ‚�\u001B[39m\u001B[37m<2147458754>\u001B[39m\u001B[33m)\u001B[39m\u001B[33m)\u001B[39m) â†’ \u001B[1m\u001B[37m2147508542\u001B[39m\u001B[0mâ†’ \u001B[1m\u001B[31m18446744069414584320\u001B[39m\u001B[0mâ†’ \u001B[1m\u001B[31m18446744069414584320\u001B[39m\u001B[0m\n"
            + "\n"
            + "\u001B[1m\u001B[31mrlptxrcpt.phase-transition\u001B[39m\u001B[0m failed:\n"
            + "\u001B[1m\u001B[97m                          \u001B[39m\u001B[0m82                                                                            83                                                                            \u001B[1m\u001B[31m84  \u001B[39m\u001B[0m85   86   \n"
            + "\u001B[1m\u001B[97m C/INV[(.- PHASE_5 1)]    \u001B[39m\u001B[0m8444461749428370424248824938781546531375899335154063827935233455917409239040  8444461749428370424248824938781546531375899335154063827935233455917409239040  \u001B[1m\u001B[31m0   \u001B[39m\u001B[0mnil  nil  \n"
            + "\u001B[1m\u001B[97m C/INV[(.- PHASE_END 1)]  \u001B[39m\u001B[0m8444461749428370424248824938781546531375899335154063827935233455917409239040  0                                                                             \u001B[1m\u001B[31m0   \u001B[39m\u001B[0mnil  nil  \n"
            + "\u001B[1m\u001B[97m PHASE_5                  \u001B[39m\u001B[0m0                                                                             0                                                                             \u001B[1m\u001B[31m1   \u001B[39m\u001B[0mnil  nil  \n"
            + "\u001B[1m\u001B[97m PHASE_END                \u001B[39m\u001B[0m0                                                                             1                                                                             \u001B[1m\u001B[31m1   \u001B[39m\u001B[0mnil  nil  \n"
            + "\u001B[1m\u001B[97m TXRCPT_SIZE              \u001B[39m\u001B[0m22                                                                            6                                                                             \u001B[1m\u001B[31m5   \u001B[39m\u001B[0mnil  nil  \n"
            + "\n"
            + "\u001B[32m(* \u001B[39m\u001B[33m(* \u001B[39m\u001B[34m(- \u001B[39m\u001B[1m\u001B[37m1\u001B[39m\u001B[0m\n"
            + "   \u001B[90mâ”‚\u001B[39m  \u001B[90m(* \u001B[39m\u001B[90m(.- \u001B[39m\u001B[1m\u001B[90mPHASE_END\u001B[39m\u001B[0m\u001B[90m<1>\u001B[39m \u001B[1m\u001B[90m1\u001B[39m\u001B[0m\u001B[90m)\u001B[39m \u001B[1m\u001B[90mC/INV[(.- PHASE_END 1)]\u001B[39m\u001B[0m\u001B[90m<0>\u001B[39m\u001B[90m)\u001B[39m) â†’ \u001B[1m\u001B[90m0\u001B[39m\u001B[0mâ†’ \u001B[1m\u001B[90m0\u001B[39m\u001B[0mâ†’ \u001B[1m\u001B[37m1\u001B[39m\u001B[0m\n"
            + "   \u001B[36m(- \u001B[39m\u001B[1m\u001B[37m1\u001B[39m\u001B[0m\n"
            + "   \u001B[90mâ”‚\u001B[39m  \u001B[90m(* \u001B[39m\u001B[90m(.- \u001B[39m\u001B[1m\u001B[90mPHASE_5\u001B[39m\u001B[0m\u001B[90m<1>\u001B[39m \u001B[1m\u001B[90m1\u001B[39m\u001B[0m\u001B[90m)\u001B[39m \u001B[1m\u001B[90mC/INV[(.- PHASE_5 1)]\u001B[39m\u001B[0m\u001B[90m<0>\u001B[39m\u001B[90m)\u001B[39m)) â†’ \u001B[1m\u001B[90m0\u001B[39m\u001B[0mâ†’ \u001B[1m\u001B[90m0\u001B[39m\u001B[0mâ†’ \u001B[1m\u001B[37m1\u001B[39m\u001B[0mâ†’ \u001B[1m\u001B[37m1\u001B[39m\u001B[0m\n"
            + " \u001B[1m\u001B[31mTXRCPT_SIZE\u001B[39m\u001B[0m\u001B[31m<5>\u001B[39m) â†’ \u001B[1m\u001B[31m5\u001B[39m\u001B[0m\n"
            + "\n"
            + "txndata-into-rlptxrcpt failed:\n"
            + "mismatch line 3:\n"
            + "\u001B[32mrlptxrcpt.ABS_TX_NUM_MAX\u001B[39m - \u001B[32mtxndata.C/#EXPAND[(* ABS_TX_NUM_MAX (~ PHASE_RLP_TXNRCPT))]\u001B[39m: 1\n"
            + "\u001B[32mrlptxrcpt.ABS_TX_NUM\u001B[39m - \u001B[32mtxndata.C/#EXPAND[(* ABS_TX_NUM (~ PHASE_RLP_TXNRCPT))]\u001B[39m: 1\n"
            + "\u001B[32mrlptxrcpt.PHASE_ID\u001B[39m - \u001B[32mtxndata.C/#EXPAND[(* PHASE_RLP_TXNRCPT (~ PHASE_RLP_TXNRCPT))]\u001B[39m: 3\n"
            + "\u001B[32mrlptxrcpt.INPUT_1\u001B[39m - \u001B[32mtxndata.C/#EXPAND[(* OUTGOING_RLP_TXNRCPT (~ PHASE_RLP_TXNRCPT))]\u001B[39m: 18446744071562092862\n"
            + "\n"
            + "txndata-into-wcp failed:\n"
            + "mismatch line 9:\n"
            + "\u001B[32mwcp.ARGUMENT_1_HI\u001B[39m - \u001B[32mtxndata.C/#EXPAND[0]\u001B[39m: 0\n"
            + "\u001B[32mwcp.ARGUMENT_1_LO\u001B[39m - \u001B[32mtxndata.C/#EXPAND[(* WCP_FLAG ARG_ONE_LO)]\u001B[39m: 18446744071562092862\n"
            + "\u001B[32mwcp.ARGUMENT_2_HI\u001B[39m - \u001B[32mtxndata.C/#EXPAND[0]\u001B[39m: 0\n"
            + "\u001B[32mwcp.ARGUMENT_2_LO\u001B[39m - \u001B[32mtxndata.C/#EXPAND[(* WCP_FLAG ARG_TWO_LO)]\u001B[39m: 4294967296\n"
            + "\u001B[32mwcp.RESULT\u001B[39m - \u001B[32mtxndata.C/#EXPAND[(* WCP_FLAG RES)]\u001B[39m: 1\n"
            + "\u001B[32mwcp.INST\u001B[39m - \u001B[32mtxndata.C/#EXPAND[(* WCP_FLAG INST)]\u001B[39m: 15\n"
            + "\n"
            + "Error: while checking \u001B[1m\u001B[97mC:\\Users\\huc_c\\AppData\\Local\\Temp\\1780917781437601421.lt\u001B[39m\u001B[0m\n"
            + "\n"
            + "Caused by:\n"
            + "    constraints failed: \u001B[31m\u001B[1mrlptxrcpt.phase3\u001B[0m\u001B[39m, \u001B[31m\u001B[1mtxndata-into-wcp\u001B[0m\u001B[39m, \u001B[31m\u001B[1mtxndata-into-rlptxrcpt\u001B[0m\u001B[39m, \u001B[31m\u001B[1mtxndata.cumulative-gas\u001B[0m\u001B[39m, \u001B[31m\u001B[1mrlptxrcpt.phase-transition\u001B[0m\u001B[39m\n"
            + "]"
            + "      at app//net.consensys.linea.testing.ExecutionEnvironment.checkTracer(ExecutionEnvironment.java:72)\n"
            + "      at app//net.consensys.linea.GeneralStateReferenceTestTools.executeTest(GeneralStateReferenceTestTools.java:235)\n"
            + "      at app//net.consensys.linea.generated.generalstate.GeneralStateReferenceTest_200.execution(GeneralStateReferenceTest_200.java:60)\n"
            + "      at java.base@21.0.2/java.lang.reflect.Method.invoke(Method.java:580)\n"
            + "      at java.base@21.0.2/java.util.concurrent.ForkJoinTask.doExec(ForkJoinTask.java:387)\n"
            + "      at java.base@21.0.2/java.util.concurrent.ForkJoinPool$WorkQueue.topLevelExec(ForkJoinPool.java:1312)\n"
            + "      at java.base@21.0.2/java.util.concurrent.ForkJoinPool.scan(ForkJoinPool.java:1843)\n"
            + "      at java.base@21.0.2/java.util.concurrent.ForkJoinPool.runWorker(ForkJoinPool.java:1808)\n"
            + "      at java.base@21.0.2/java.util.concurrent.ForkJoinWorkerThread.run(ForkJoinWorkerThread.java:188)";

    Map<String, Set<String>> res = TestOutcomeWriterTool.extractConstraints(message);
    assertThat(res.size()).isEqualTo(2);
    assertThat(res.get("txndata")).isEqualTo(Set.of("wcp", "rlptxrcpt", "cumulative-gas"));
    assertThat(res.get("rlptxrcpt")).isEqualTo(Set.of("phase3", "phase-transition"));
  }

  @Test
  void parseBlockchainReferenceTestOutcome() {
    TestOutcome outcome =
        TestOutcomeWriterTool.parseTestOutcome(
            """
            {
              "abortedCounter": 20,
              "disabledCounter": 0,
              "failedCounter": 4,
              "modulesToConstraintsToTests": {
                "ASSERTION_FAILED": {
                  "expected: true but was: fals": [
                    "InitCollision_d0g0v0_London[London]",
                    "sstore_combinations_initial21_d9g0v0_London[London]"
                    ]
                 }
              },
              "successCounter": 0
            }""");
    assertThat(outcome.getAbortedCounter()).isEqualTo(20);
    assertThat(outcome.getModulesToConstraintsToTests().size()).isEqualTo(1);
  }
}
