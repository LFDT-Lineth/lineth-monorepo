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

package net.consensys.linea.zktracer.module.hub.state;

import com.google.common.base.Preconditions;
import java.util.ArrayList;
import java.util.List;
import lombok.Getter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.hub.section.TraceSection;

/**
 * Stores all the trace sections associated to one transaction, stored in chronological order of
 * creation.
 */
@Getter
@Accessors(fluent = true)
public class TraceSections {
  /** The {@link TraceSection} of which this transaction trace is made of */
  private final List<TraceSection> trace = new ArrayList<>();

  public int size() {
    return trace.size();
  }

  /**
   * Returns the latest trace section, i.e. the most recent.
   *
   * @return the most recent trace section
   */
  public TraceSection currentSection() {
    return trace.get(size() - 1);
  }

  /**
   * Returns the trace section that is `n` positions before the most recent one.
   *
   * @param n the number of positions before the most recent trace section
   * @return the trace section that is `n` positions before the most recent one
   * @throws IllegalArgumentException if there are fewer than `n + 1` sections in the trace
   */
  public TraceSection previousSection(int n) {
    Preconditions.checkArgument(
        trace.size() > n,
        "Trace has only %s sections, cannot access %s sections back",
        trace.size(),
        n);
    return trace.get(size() - 1 - n);
  }

  /**
   * @return whether this trace is empty
   */
  public boolean isEmpty() {
    return this.trace.isEmpty();
  }

  /**
   * Add a {@link TraceSection} to this transaction trace.
   *
   * @param section the section to append
   */
  public void add(TraceSection section) {
    // Link the current section with the previous and next one
    final TraceSection previousSection = this.trace.isEmpty() ? null : this.trace.getLast();
    if (previousSection != null) {
      previousSection.nextSection(section);
      section.previousSection(previousSection);
    } else {
      // If this section is the first section of the transaction, set the logStamp
      section.commonValues.logStamp(section.commonValues.stamps.log());
    }
    this.trace.add(section);
  }

  /**
   * Generate the final numeric trace from the accumulated information.
   *
   * @param hubTrace where to materialize the trace
   */
  public void commit(Trace.Hub hubTrace) {
    for (TraceSection opSection : this.trace) {
      opSection.seal();
      opSection.trace(hubTrace);
    }
  }

  /**
   * @return the line count in this transaction trace
   */
  public int lineCount() {
    int lineCount = 0;
    for (TraceSection s : trace) {
      if (s.exceptionalContextFragment != null) s.fragments().add(s.exceptionalContextFragment);
      lineCount += s.fragments().size();
    }
    return lineCount;
  }
}
