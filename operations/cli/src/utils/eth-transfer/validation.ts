export function validateETHThreshold(input: string) {
  if (parseInt(input) <= 1) {
    throw new Error("Threshold must be higher than 1 ETH");
  }
  return input;
}
