export const isValidNodeTarget = (sourceNode: string, targetNode: string): boolean => {
  try {
    if (sourceNode) {
      new URL(sourceNode);
    }
    new URL(targetNode);
    return true;
  } catch {
    return false;
  }
};

export const isLocalPort = (value: string): boolean => {
  const port = Number(value);
  return !isNaN(port) && port >= 1 && port <= 65535;
};
