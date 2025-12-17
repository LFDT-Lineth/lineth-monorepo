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

package net.consensys.linea.zktracer;

import static com.google.common.base.Preconditions.checkArgument;
import static org.assertj.core.api.Assertions.assertThat;

import java.util.ArrayList;
import java.util.stream.Stream;
import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;

@ExtendWith(UnitTestWatcher.class)
public class ZkTracerTest extends TracerTestBase {

  @Test
  public void createNewTracer() {
    final ZkTracer zkTracer = new ZkTracer(chainConfig);
    assertThat(zkTracer.isExtendedTracing()).isTrue();
  }

  @Test
  void tracedModuleForFork() {
    final ZkTracer zkTracer = new ZkTracer(chainConfig);
    final int totalNumberOfModules =
        new ArrayList<>(
                Stream.concat(
                        zkTracer.getHub().realModule().stream(),
                        zkTracer.getHub().refTableModules().stream())
                    .toList())
            .size();
    final int numberOfTracedModules = zkTracer.getHub().getModulesToTrace().size();
    checkArgument(totalNumberOfModules == numberOfTracedModules, "no missing modules expected");
  }
}
