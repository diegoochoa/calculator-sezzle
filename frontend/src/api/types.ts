/** Wire types for the calculation API. Mirrors the Go server's contract. */

export type ApiErrorBody = {
  code: string;
  message: string;
  /** Rune offset into the submitted expression. */
  position?: number;
};

export type ApiErrorResponse = {
  error: ApiErrorBody;
  requestId?: string;
};

export type TokenResponse = {
  token: string;
  tokenType: string;
  expiresIn: number;
  expiresAt: string;
};

export type CalculateRequest = {
  expression: string;
  precision?: number;
};

export type CalculateResponse = {
  result: number;
  /** Display string, so the client never re-derives the server's rounding. */
  formatted: string;
  expression: string;
};
