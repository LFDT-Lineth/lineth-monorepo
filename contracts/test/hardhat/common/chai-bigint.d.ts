// chai 6 compares bigints at runtime (e.g. wei balances), but @types/chai 5.x types the numeric comparers as
// `number | Date`. These overloads let tests assert on bigints without lossy `Number()` conversions.
export {};

declare global {
  namespace Chai {
    interface NumberComparer {
      (value: bigint, message?: string): Assertion;
    }

    interface Assertion {
      within(start: bigint, finish: bigint, message?: string): Assertion;
    }
  }
}
