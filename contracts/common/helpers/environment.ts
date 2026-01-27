export function getRequiredEnvVar(name: string): string {
  const envValue = process.env[name];
  if (!envValue) {
    throw new Error(`Required environment variable "${name}" is missing or empty.`);
  }
  console.log(`Using environment variable ${name}=${envValue}`);
  return envValue;
}

export function getEnvVarOrDefault(envVar: string, defaultValue: unknown) {
  const envValue = process.env[envVar];

  if (!envValue) {
    console.log(`Using default ${envVar}`);
    return defaultValue;
  }

  console.log(`Using provided ${envVar} environment variable`);
  try {
    const parsedValue = JSON.parse(envValue);
    if (typeof parsedValue === "object" && !Array.isArray(parsedValue)) {
      return parsedValue;
    }

    if (Array.isArray(parsedValue) && parsedValue.every((item) => typeof item === "object")) {
      return parsedValue;
    }
  } catch {
    console.log(`Unable to parse ${envVar}, returning as string.`);
  }
  return envValue;
}
