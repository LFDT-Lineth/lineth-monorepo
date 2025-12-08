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

import java.util.*;

import lombok.extern.slf4j.Slf4j;

@Slf4j
public class ReferenceTestOutcomeRecorderTool {
  public static final String JSON_INPUT_FILENAME =
      System.getenv()
          .getOrDefault("REFERENCE_TEST_OUTCOME_INPUT_FILE", "failedReferenceTests-input.json");
  public static final String JSON_OUTPUT_FILENAME =
      System.getenv()
          .getOrDefault("REFERENCE_TEST_OUTCOME_OUTPUT_FILE", "failedReferenceTests.json");
}
