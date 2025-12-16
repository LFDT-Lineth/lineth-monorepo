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

package net.consensys.linea.zktracer.module.rom;

import static net.consensys.linea.zktracer.module.ModuleName.ROM;

import java.util.List;
import lombok.RequiredArgsConstructor;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.container.module.Module;
import net.consensys.linea.zktracer.module.ModuleName;
import net.consensys.linea.zktracer.module.romlex.RomLex;
import net.consensys.linea.zktracer.module.romlex.RomOperation;

@RequiredArgsConstructor
public class Rom implements Module {
  private final RomLex romLex;

  @Override
  public ModuleName moduleKey() {
    return ROM;
  }

  @Override
  public void commitTransactionBundle() {}

  @Override
  public void popTransactionBundle() {}

  @Override
  public int lineCount() {
    return romLex.operations().lineCount();
  }

  @Override
  public int spillage(Trace trace) {
    return trace.rom().spillage();
  }

  @Override
  public List<Trace.ColumnHeader> columnHeaders(Trace trace) {
    return trace.rom().headers(this.lineCount());
  }

  @Override
  public void commit(Trace trace) {
    int codeFragmentIndex = 0;
    final int codeFragmentIndexInfinity = romLex.sortedOperations().size();
    for (RomOperation chunk : romLex.sortedOperations()) {
      chunk.trace(trace.rom(), ++codeFragmentIndex, codeFragmentIndexInfinity);
    }
  }
}
