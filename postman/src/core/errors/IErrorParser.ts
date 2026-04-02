export type ParsedError = {
  retryable: boolean;
  message: string;
};

export interface IErrorParser {
  parse(error: unknown): ParsedError;
}
