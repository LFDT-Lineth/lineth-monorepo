import { useMemo, type JSX } from "react";

import { formatUnits } from "viem";

export function useFormattedDigit(value: bigint, decimals: number = 18): JSX.Element {
  return useMemo(() => {
    if (value <= 0n) return <>0.00</>;

    const valueStr = formatUnits(value, decimals);
    const num = Number(valueStr);

    if (!isFinite(num) || num === 0) return <>0.00</>;
    if (num >= 1e-8) return <>{num.toFixed(8)}</>;

    const match = /^0\.0+(?=\d)/.exec(valueStr);
    if (!match) return <>{num.toFixed(8)}</>;

    const zeroCount = match[0].length - 2;
    const remainderDigits = valueStr.slice(match[0].length);
    const rounded = Math.round(parseFloat("0." + remainderDigits) * 100)
      .toString()
      .padStart(2, "0");

    return (
      <>
        0.0<sub>{zeroCount}</sub>
        {rounded}
      </>
    );
  }, [value, decimals]);
}
