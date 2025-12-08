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

package net.consensys.linea.zktracer.runtime;

import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.runtime.callstack.CallFrame;
import net.consensys.linea.zktracer.types.EWord;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.evm.frame.MessageFrame;
import org.hyperledger.besu.evm.internal.Words;

public class LogData {
  public final CallFrame callFrame;
  public final Bytes ramSourceBytes;
  public final EWord offset;
  public final long size;

  public LogData(Hub hub) {
    this.callFrame = hub.currentFrame();
    final MessageFrame messageFrame = callFrame.frame();
    offset = EWord.of(messageFrame.getStackItem(0));
    size = Words.clampedToLong(messageFrame.getStackItem(1));
    this.ramSourceBytes =
        size == 0 ? Bytes.EMPTY : messageFrame.shadowReadMemory(0, messageFrame.memoryByteSize());
  }

  public boolean nontrivialLog() {
    return this.size != 0;
  }
}
