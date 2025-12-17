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

package net.consensys.linea.zktracer.module.rlpUtils;

import static net.consensys.linea.zktracer.Trace.*;
import static net.consensys.linea.zktracer.module.ModuleName.RLP_UTILS;
import static net.consensys.linea.zktracer.types.Utils.rightPadToBytes16;

import java.math.BigInteger;
import java.util.List;
import lombok.Getter;
import lombok.RequiredArgsConstructor;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.container.module.OperationSetModule;
import net.consensys.linea.zktracer.container.stacked.ModuleOperationAdder;
import net.consensys.linea.zktracer.container.stacked.ModuleOperationStackedSet;
import net.consensys.linea.zktracer.module.ModuleName;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Address;

@RequiredArgsConstructor
@Accessors(fluent = true)
public class RlpUtils implements OperationSetModule<RlpUtilsCall> {
  public static final Bytes BYTES_PREFIX_SHORT_INT = Bytes.minimalBytes(RLP_PREFIX_INT_SHORT);
  public static final Bytes BYTES_PREFIX_SHORT_LIST = Bytes.minimalBytes(RLP_PREFIX_LIST_SHORT);
  public static final BigInteger BI_PREFIX_SHORT_INT = BigInteger.valueOf(RLP_PREFIX_INT_SHORT);
  public static final Bytes BYTES16_PREFIX_ADDRESS =
      rightPadToBytes16(Bytes.minimalBytes(RLP_PREFIX_INT_SHORT + Address.SIZE));
  public static final Bytes BYTES16_PREFIX_BYTES32 =
      rightPadToBytes16(Bytes.minimalBytes(RLP_PREFIX_INT_SHORT + WORD_SIZE));

  @Getter
  private final ModuleOperationStackedSet<RlpUtilsCall> operations =
      new ModuleOperationStackedSet<>();

  @Override
  public ModuleName moduleKey() {
    return RLP_UTILS;
  }

  public RlpUtilsCall call(RlpUtilsCall call) {
    final ModuleOperationAdder addedOp = operations.addAndGet(call);
    final RlpUtilsCall addedCall = (RlpUtilsCall) addedOp.op();
    if (addedOp.isNew()) {
      addedCall.compute();
    }
    return addedCall;
  }

  @Override
  public int spillage(Trace trace) {
    return trace.rlputils().spillage();
  }

  @Override
  public List<Trace.ColumnHeader> columnHeaders(Trace trace) {
    return trace.rlputils().headers(lineCount());
  }

  @Override
  public void commit(Trace trace) {
    for (RlpUtilsCall operation : sortOperations(new RlpUtilsOperationComparator())) {
      operation.trace(trace.rlputils());
    }
  }
}
