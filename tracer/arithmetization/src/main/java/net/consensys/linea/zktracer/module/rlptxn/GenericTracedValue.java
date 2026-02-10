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

package net.consensys.linea.zktracer.module.rlptxn;

import static com.google.common.base.Preconditions.*;
import static org.hyperledger.besu.datatypes.TransactionType.*;

import lombok.Getter;
import lombok.Setter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.types.TransactionProcessingMetadata;

@Getter
@Setter
@Accessors(fluent = true)
public class GenericTracedValue {
  private final TransactionProcessingMetadata tx;
  private final boolean type0;
  private final boolean type1;
  private final boolean type2;
  private final boolean type3;
  private final boolean type4;
  private int rlpLtByteSize;
  private int rlpLxByteSize;
  @Setter @Getter private int userTxnNumberMax;
  private int listRlpSize;
  private int itemRlpSize;

  public GenericTracedValue(TransactionProcessingMetadata tx) {
    this.tx = tx;
    type0 = tx.getBesuTransaction().getType() == FRONTIER;
    type1 = tx.getBesuTransaction().getType() == ACCESS_LIST;
    type2 = tx.getBesuTransaction().getType() == EIP1559;
    type3 = tx.getBesuTransaction().getType() == BLOB;
    type4 = tx.getBesuTransaction().getType() == DELEGATE_CODE;
  }

  public void setListRlpSize(int listRlpSize) {
    checkArgument(this.listRlpSize == 0);
    this.listRlpSize = listRlpSize;
  }

  public void setItemRlpSize(int itemRlpSize) {
    checkArgument(this.itemRlpSize == 0);
    this.itemRlpSize = itemRlpSize;
  }

  public void decrementAllCountersBy(short size) {
    listRlpSize -= size;
    itemRlpSize -= size;
  }

  public void decrementListRlpSizeBy(short size) {
    listRlpSize -= size;
  }

  public void decrementLtAndLxSizeBy(int size) {
    decrementLtSizeBy(size);
    decrementLxSizeBy(size);
  }

  public void decrementLtSizeBy(int size) {
    rlpLtByteSize -= size;
  }

  public void decrementLxSizeBy(int size) {
    rlpLxByteSize -= size;
  }
}
