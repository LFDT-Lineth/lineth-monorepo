"use client";

import { useVerifierStore } from "@/stores/verifier";
import { Card } from "@/components/ui/card";
import { Toggle } from "@/components/ui/toggle";
import { Input } from "@/components/ui/input";
import styles from "./options-section.module.scss";

export function OptionsSection() {
  const { options, setOption, parsedConfig } = useVerifierStore();

  return (
    <Card title="Verification Options" description="Configure which verification checks to run">
      <div className={styles.options}>
        <div className={styles.toggles}>
          <Toggle
            checked={options.verbose}
            onChange={(checked) => setOption("verbose", checked)}
            label="Verbose output"
            description="Show detailed verification information"
          />
          <Toggle
            checked={options.skipBytecode}
            onChange={(checked) => setOption("skipBytecode", checked)}
            label="Skip bytecode verification"
            description="Don't compare deployed bytecode against artifacts"
          />
          <Toggle
            checked={options.skipAbi}
            onChange={(checked) => setOption("skipAbi", checked)}
            label="Skip ABI verification"
            description="Don't verify function selectors match the ABI"
          />
          <Toggle
            checked={options.skipState}
            onChange={(checked) => setOption("skipState", checked)}
            label="Skip state verification"
            description="Don't verify contract storage state"
          />
        </div>

        <div className={styles.filters}>
          <Input
            name="contractFilter"
            label="Contract filter"
            placeholder="Filter by contract name"
            value={options.contractFilter || ""}
            onChange={(e) => setOption("contractFilter", e.target.value || undefined)}
            hint={parsedConfig ? `Available: ${parsedConfig.contracts.join(", ")}` : undefined}
          />
          <Input
            name="chainFilter"
            label="Chain filter"
            placeholder="Filter by chain name"
            value={options.chainFilter || ""}
            onChange={(e) => setOption("chainFilter", e.target.value || undefined)}
            hint={parsedConfig ? `Available: ${parsedConfig.chains.join(", ")}` : undefined}
          />
        </div>
      </div>
    </Card>
  );
}
