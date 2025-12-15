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
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.ReturnScenarioFragment.ReturnScenario.*;

import lombok.Setter;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.hub.fragment.TraceFragment;

@Setter
public class ReturnScenarioFragment implements TraceFragment {

  ReturnScenario scenario;

  public ReturnScenarioFragment() {
    scenario = UNDEFINED;
  }

  public enum ReturnScenario {
    UNDEFINED,
    RETURN_EXCEPTION,
    RETURN_FROM_MESSAGE_CALL_WILL_TOUCH_RAM,
    RETURN_FROM_MESSAGE_CALL_WONT_TOUCH_RAM,
    RETURN_FROM_DEPLOYMENT_EMPTY_CODE_WILL_REVERT,
    RETURN_FROM_DEPLOYMENT_EMPTY_CODE_WONT_REVERT,
    RETURN_FROM_DEPLOYMENT_NONEMPTY_CODE_WILL_REVERT,
    RETURN_FROM_DEPLOYMENT_NONEMPTY_CODE_WONT_REVERT
  }

  @Override
  public Trace.Hub trace(Trace.Hub trace) {
    checkArgument(!scenario.equals(UNDEFINED), "Return scenario not defined");
    return trace
        .peekAtScenario(true)
        // RETURN scenarios
        ///////////////////
        .pScenarioReturnException(scenario.equals(RETURN_EXCEPTION))
        .pScenarioReturnFromMessageCallWillTouchRam(
            scenario.equals(RETURN_FROM_MESSAGE_CALL_WILL_TOUCH_RAM))
        .pScenarioReturnFromMessageCallWontTouchRam(
            scenario.equals(RETURN_FROM_MESSAGE_CALL_WONT_TOUCH_RAM))
        .pScenarioReturnFromDeploymentEmptyCodeWillRevert(
            scenario.equals(RETURN_FROM_DEPLOYMENT_EMPTY_CODE_WILL_REVERT))
        .pScenarioReturnFromDeploymentEmptyCodeWontRevert(
            scenario.equals(RETURN_FROM_DEPLOYMENT_EMPTY_CODE_WONT_REVERT))
        .pScenarioReturnFromDeploymentNonemptyCodeWillRevert(
            scenario.equals(RETURN_FROM_DEPLOYMENT_NONEMPTY_CODE_WILL_REVERT))
        .pScenarioReturnFromDeploymentNonemptyCodeWontRevert(
            scenario.equals(RETURN_FROM_DEPLOYMENT_NONEMPTY_CODE_WONT_REVERT));
  }
}
