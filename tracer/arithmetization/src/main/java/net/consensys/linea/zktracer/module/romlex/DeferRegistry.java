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

package net.consensys.linea.zktracer.module.romlex;

import java.util.ArrayList;
import java.util.List;

public class DeferRegistry {
  private final List<RomLexDefer> defers = new ArrayList<>();

  public void register(RomLexDefer defer) {
    this.defers.add(defer);
  }

  public void clear() {
    this.defers.clear();
  }

  public void trigger(final ContractMetadata contractMetadata) {
    for (RomLexDefer defer : this.defers) {
      defer.updateContractMetadata(contractMetadata);
    }
    this.clear();
  }
}
