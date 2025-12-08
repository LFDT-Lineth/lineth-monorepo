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

package net.consensys.linea.zktracer.types;

import static org.junit.jupiter.api.Assertions.*;

import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import org.apache.tuweni.bytes.Bytes;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;

@ExtendWith(UnitTestWatcher.class)
class BytecodeTest extends TracerTestBase {

  @Test
  void getCodeHash() {
    Bytecode byteCode = new Bytecode(Bytes.wrap("VV".getBytes()));
    assertEquals(
        "0x0f06f483858ae915c1a98ada06b7d12403b2dd45a554eb235555eedfb864e302",
        byteCode.getCodeHash().toString());
  }

  @Test
  void getCodeHashNull() {
    Bytecode byteCode = new Bytecode((Bytes) null);
    assertEquals(
        "0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470",
        byteCode.getCodeHash().toString());
  }
}
