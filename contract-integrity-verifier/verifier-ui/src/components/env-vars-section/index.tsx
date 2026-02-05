"use client";

import { useVerifierStore } from "@/stores/verifier";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { MAX_ENV_VAR_VALUE_LENGTH } from "@/lib/constants";
import styles from "./env-vars-section.module.scss";

export function EnvVarsSection() {
  const { envVarFields, envVarValues, setEnvVar } = useVerifierStore();

  if (envVarFields.length === 0) {
    return null;
  }

  return (
    <Card
      title="Environment Variables"
      description="Provide values for the environment variables referenced in your config"
    >
      <div className={styles.form}>
        {envVarFields.map((field) => (
          <Input
            key={field.name}
            name={field.name}
            type={field.type === "password" ? "password" : "text"}
            label={field.label}
            placeholder={field.placeholder}
            value={envVarValues[field.name] || ""}
            onChange={(e) => setEnvVar(field.name, e.target.value)}
            required={field.required}
            maxLength={MAX_ENV_VAR_VALUE_LENGTH}
            hint={`Config variable: \${${field.name}}`}
          />
        ))}
      </div>
    </Card>
  );
}
