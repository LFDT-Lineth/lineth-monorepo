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

package net.consensys.linea.plugins;

import java.util.function.Supplier;
import lombok.Getter;
import lombok.RequiredArgsConstructor;
import lombok.experimental.Accessors;

@Accessors(fluent = true)
@RequiredArgsConstructor
public class LineaOptionsPluginConfiguration {
  @Getter private final LineaCliOptions cliOptions;
  private final Supplier<LineaOptionsConfiguration> optionsConfigSupplier;
  @Getter private LineaOptionsConfiguration optionsConfig;

  public void initOptionsConfig() {
    if (optionsConfig == null) {
      optionsConfig = optionsConfigSupplier.get();
    }
  }
}
