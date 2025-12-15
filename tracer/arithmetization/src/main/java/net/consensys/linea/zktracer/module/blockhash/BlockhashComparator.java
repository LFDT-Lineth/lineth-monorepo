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

package net.consensys.linea.zktracer.module.blockhash;

import java.util.Comparator;

public class BlockhashComparator implements Comparator<BlockhashOperation> {
  @Override
  public int compare(BlockhashOperation o1, BlockhashOperation o2) {
    // First, sort by BLOCK_NUMBER
    final int blockNumberComparison = o1.blockhashArg().compareTo(o2.blockhashArg());
    if (blockNumberComparison != 0) {
      return blockNumberComparison;
    } else {
      // Second, sort by RELATIVE_BLOCK
      return o1.relBlock() - o2.relBlock();
    }
  }
}
