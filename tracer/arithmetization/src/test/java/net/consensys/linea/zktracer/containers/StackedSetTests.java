/*
 * Copyright ConsenSys AG.
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

package net.consensys.linea.zktracer.containers;

import java.math.BigInteger;
import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.zktracer.container.stacked.ModuleOperationStackedSet;
import net.consensys.linea.zktracer.module.add.AddOperation;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.junit.jupiter.api.Assertions;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;

@ExtendWith(UnitTestWatcher.class)
public class StackedSetTests extends TracerTestBase {

  private static final AddOperation ONE_PLUS_ONE =
      new AddOperation(
          OpCode.ADD,
          Bytes32.leftPad(Bytes.wrap(BigInteger.ONE.toByteArray())),
          Bytes32.leftPad(Bytes.wrap(BigInteger.ONE.toByteArray())));

  private static final AddOperation ONE_PLUS_TWO =
      new AddOperation(
          OpCode.ADD,
          Bytes32.leftPad(Bytes.wrap(BigInteger.ONE.toByteArray())),
          Bytes32.leftPad(Bytes.wrap(BigInteger.TWO.toByteArray())));

  @Test
  public void push() {
    ModuleOperationStackedSet<AddOperation> chunks = new ModuleOperationStackedSet<>();

    chunks.add(ONE_PLUS_ONE);
    chunks.add(ONE_PLUS_ONE);
    chunks.add(ONE_PLUS_ONE);
    Assertions.assertEquals(1, chunks.size());
    chunks.popTransactionBundle();
    Assertions.assertEquals(0, chunks.size());
  }

  @Test
  public void multiplePushPop() {
    ModuleOperationStackedSet<AddOperation> chunks = new ModuleOperationStackedSet<>();
    chunks.add(ONE_PLUS_ONE);
    chunks.add(ONE_PLUS_ONE);
    Assertions.assertEquals(1, chunks.size());

    chunks.commitTransactionBundle();
    chunks.add(ONE_PLUS_ONE);
    Assertions.assertEquals(1, chunks.size());
    chunks.add(ONE_PLUS_TWO);
    Assertions.assertEquals(2, chunks.size());
    chunks.popTransactionBundle();
    Assertions.assertEquals(1, chunks.size());
  }
}
