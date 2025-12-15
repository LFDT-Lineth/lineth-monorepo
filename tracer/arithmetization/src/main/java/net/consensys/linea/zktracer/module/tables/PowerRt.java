/*
 * Copyright ConsenSys Inc.
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

package net.consensys.linea.zktracer.module.tables;

import static net.consensys.linea.zktracer.Trace.LLARGE;
import static net.consensys.linea.zktracer.module.ModuleName.POWER_REFERENCE_TABLE;

import java.util.List;

import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.container.module.Module;
import net.consensys.linea.zktracer.module.ModuleName;
import net.consensys.linea.zktracer.types.Bytes16;
import org.apache.tuweni.bytes.Bytes;

public class PowerRt implements Module {
  @Override
  public ModuleName moduleKey() {
    return POWER_REFERENCE_TABLE;
  }

  @Override
  public void popTransactionBundle() {}

  @Override
  public void commitTransactionBundle() {}

  @Override
  public int lineCount() {
    return LLARGE;
  }

  @Override
  public int spillage(Trace trace) {
    return trace.power().spillage();
  }

  @Override
  public List<Trace.ColumnHeader> columnHeaders(Trace trace) {
    return trace.power().headers(lineCount());
  }

  public void commit(Trace trace) {
    for (int exponent = 0; exponent < LLARGE; exponent++) {
      trace
          .power()
          .iomf(true)
          .exponent(exponent)
          .power(Bytes16.leftPad(Bytes.minimalBytes(1)).shiftLeft(8 * exponent))
          .validateRow();
    }
  }
}
