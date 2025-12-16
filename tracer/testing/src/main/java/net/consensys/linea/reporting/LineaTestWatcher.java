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
package net.consensys.linea.reporting;

import static net.consensys.linea.reporting.TestOutcomeWriterTool.getLogEventMessages;
import static net.consensys.linea.reporting.TestOutcomeWriterTool.mapAndStoreTestResult;
import static net.consensys.linea.reporting.TestState.*;
import static net.consensys.linea.reporting.TestState.ABORTED;

import java.util.Map;
import java.util.Optional;
import java.util.Set;
import lombok.extern.slf4j.Slf4j;
import org.junit.jupiter.api.extension.ExtensionContext;
import org.junit.jupiter.api.extension.TestWatcher;

@Slf4j
public abstract class LineaTestWatcher implements TestWatcher {

  @Override
  public void testFailed(ExtensionContext context, Throwable cause) {
    String testName = getTestName(context);
    log.info("Adding failure for {}", testName);
    Map<String, Set<String>> logEventMessages = getLogEventMessages(cause);
    mapAndStoreTestResult(testName, FAILED, logEventMessages);
    log.info("Failure added for {}", testName);
  }

  protected abstract String getTestName(ExtensionContext context);

  @Override
  public void testSuccessful(ExtensionContext context) {
    String testName = getTestName(context);
    mapAndStoreTestResult(testName, SUCCESS, Map.of());
  }

  @Override
  public void testDisabled(ExtensionContext context, Optional<String> reason) {
    String testName = getTestName(context);
    mapAndStoreTestResult(testName, DISABLED, Map.of());
  }

  @Override
  public void testAborted(ExtensionContext context, Throwable cause) {
    String testName = getTestName(context);
    mapAndStoreTestResult(testName, ABORTED, Map.of());
  }
}
