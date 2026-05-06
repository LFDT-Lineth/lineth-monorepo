export type FunctionOnly<T> = Pick<T, FunctionKeys<T>>;

export type FunctionKeys<T> = {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  [K in keyof T]: T[K] extends (...args: any[]) => any ? K : never;
}[keyof T];

export type StrictFunctionOnly<T, U> = [keyof U] extends [FunctionKeys<T>]
  ? [FunctionKeys<T>] extends [keyof U]
    ? U
    : never
  : never;
