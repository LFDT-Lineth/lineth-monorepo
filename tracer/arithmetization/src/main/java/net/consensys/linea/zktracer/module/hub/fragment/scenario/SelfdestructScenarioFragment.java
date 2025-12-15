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
package net.consensys.linea.zktracer.module.hub.fragment.scenario;

import static com.google.common.base.Preconditions.*;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.SelfdestructScenarioFragment.SelfdestructScenario.UNDEFINED;

import lombok.Setter;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.hub.fragment.TraceFragment;

@Setter
public class SelfdestructScenarioFragment implements TraceFragment {

  SelfdestructScenario scenario;

  public SelfdestructScenarioFragment() {
    scenario = UNDEFINED;
  }

  public enum SelfdestructScenario {
    UNDEFINED,
    SELFDESTRUCT_EXCEPTION,
    SELFDESTRUCT_WILL_REVERT,
    SELFDESTRUCT_WONT_REVERT_ALREADY_MARKED,
    SELFDESTRUCT_WONT_REVERT_NOT_YET_MARKED
  }

  @Override
  public Trace.Hub trace(Trace.Hub trace) {
    checkArgument(
        !scenario.equals(UNDEFINED),
        "SELFDESTRUCT: scenario %s is UNDEFINED at trace time",
        scenario);
    return trace
        .peekAtScenario(true)
        // // SELFDESTRUCT scenarios
        ////////////////////////////
        .pScenarioSelfdestructException(
            scenario.equals(SelfdestructScenario.SELFDESTRUCT_EXCEPTION))
        .pScenarioSelfdestructWillRevert(
            scenario.equals(SelfdestructScenario.SELFDESTRUCT_WILL_REVERT))
        .pScenarioSelfdestructWontRevertAlreadyMarked(
            scenario.equals(SelfdestructScenario.SELFDESTRUCT_WONT_REVERT_ALREADY_MARKED))
        .pScenarioSelfdestructWontRevertNotYetMarked(
            scenario.equals(SelfdestructScenario.SELFDESTRUCT_WONT_REVERT_NOT_YET_MARKED));
  }
}
