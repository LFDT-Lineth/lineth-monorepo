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

package net.consensys.linea.zktracer.module.rlptxrcpt;

import static net.consensys.linea.zktracer.types.Utils.fromDataSizeToLimbNbRows;

import java.util.List;
import lombok.Getter;
import lombok.RequiredArgsConstructor;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.container.ModuleOperation;
import org.hyperledger.besu.datatypes.Log;
import org.hyperledger.besu.datatypes.TransactionType;

@RequiredArgsConstructor
@Accessors(fluent = true)
@Getter
public final class RlpTxrcptOperation extends ModuleOperation {
  private final TransactionType txType;
  private final Boolean status;
  private final long gasUsed;
  private final List<Log> logs;

  @Override
  protected int computeLineCount() {
    return lineCountForRlpTxnRcpt(this.logs);
  }

  public static int lineCountForRlpTxnRcpt(List<Log> logs) {
    // Phase 0 is always 1+8=9 row long, Phase 1, 1 row long, Phase 2 8 row long,
    // Phase 3 65 = 1 +
    // 64 row long
    int rowSize = 83;

    // add the number of rows for Phase 4 : Log entry
    if (logs.isEmpty()) {
      rowSize += 1;
    } else {
      // Rlp prefix of the list of log entries is always 8 rows long
      rowSize += 8;

      for (int i = 0; i < logs.size(); i++) {
        // Rlp prefix of a log entry is always 8, Log entry address is always 3 row
        // long, Log topics
        // rlp prefix always 1
        rowSize += 12;

        // Each log Topics is 3 rows long
        rowSize += (short) (3 * logs.get(i).getTopics().size());

        // Row size of data is 1 if empty
        if (logs.get(i).getData().isEmpty()) {
          rowSize += 1;
        }
        // Row size of the data is 8 (RLP prefix)+ integer part (data-size - 1 /16) +1
        else {
          rowSize += (8 + fromDataSizeToLimbNbRows(logs.get(i).getData().size()));
        }
      }
    }

    return rowSize;
  }
}
