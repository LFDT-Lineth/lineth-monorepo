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

import static net.consensys.linea.reporting.TestState.*;

import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.reporting.LineaTestWatcher;
import org.junit.jupiter.api.extension.ExtensionContext;

@Slf4j
public class ReferenceTestWatcher extends LineaTestWatcher {
  @Override
  public String getTestName(ExtensionContext context) {
    return context.getTestClass().map(c -> c.getSimpleName()).orElse("")
        + ":"
        + context.getDisplayName().split(": ")[1];
  }
}
