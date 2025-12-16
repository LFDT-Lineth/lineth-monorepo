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

import java.io.FileWriter;
import java.io.IOException;
import java.lang.reflect.Method;
import java.nio.file.Files;
import java.nio.file.Path;
import java.text.SimpleDateFormat;
import java.util.Date;
import java.util.List;
import java.util.Optional;
import net.consensys.linea.UnitTestWatcher;
import org.junit.jupiter.api.extension.ExtendWith;
import org.junit.jupiter.api.extension.ExtensionContext;
import org.junit.jupiter.api.extension.TestWatcher;

@ExtendWith(UnitTestWatcher.class)
public class EcPairingTestWatcher implements TestWatcher {
  final String timestamp = new SimpleDateFormat("yyyyMMddHHmmss").format(new Date());
  final List<String> testsToWatch =
      List.of(
          "testEcPairingSingleForScenarioUsingMethodSource",
          "testEcPairingSingleForScenarioUsingCsv",
          "testEcPairingGenericForScenarioUsingMethodSource",
          "testEcPairingGenericForScenarioUsingCsv");

  @Override
  public void testSuccessful(ExtensionContext context) {
    logResult(context, true);
  }

  @Override
  public void testFailed(ExtensionContext context, Throwable cause) {
    logResult(context, false);
  }

  private void logResult(ExtensionContext context, boolean successful) {
    String testName = context.getTestMethod().orElseThrow().getName();
    if (!testsToWatch.contains(testName)) {
      return;
    }
    String arguments = EcPairingArgumentsSingleton.getInstance().getArguments();

    // Compute temporary file name based on the name of the test (camel case to snake case) and
    // timestamp
    String fileName =
        System.getProperty("java.io.tmpdir")
            + "/"
            + testName.replaceAll("([a-z])([A-Z])", "$1_$2").toLowerCase()
            + (successful ? "_success_" : "_failure_")
            + timestamp
            + ".csv";
    Path filePath = Path.of(fileName);

    // Create the file in case it does not exist yet
    try {
      if (!Files.exists(filePath)) {
        Files.createFile(filePath);
      }
    } catch (IOException e) {
      throw new RuntimeException(e);
    }
    System.out.println("Logging ecPairing test results to: " + filePath.toAbsolutePath());

    // Write the test parameters to the file
    Optional<Method> testMethod = context.getTestMethod();
    if (testMethod.isPresent()) {
      try (FileWriter writer = new FileWriter(filePath.toFile(), true)) {
        writer.append(arguments).append("\n");
      } catch (IOException e) {
        throw new RuntimeException(e);
      }
    }
  }
}
