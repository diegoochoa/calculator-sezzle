/**
 * Domain types for the calculator shell.
 *
 * All arithmetic lives in the Go API. What remains here is text editing: the
 * shell composes an expression, the server evaluates it.
 */

/** What the server said went wrong, or what the network did. */
export type CalculatorError = {
  code: string;
  message: string;
  /** Offset in the expression, when the server could pinpoint it. */
  position?: number;
};

export type CalculatorStatus = 'idle' | 'evaluating';

export type CalculatorState = {
  /** The expression being composed. */
  expression: string;
  /** Formatted result of the last successful evaluation. */
  result: string | null;
  /** The expression that produced `result`, shown above it. */
  evaluated: string | null;
  /** When true the next entry starts a fresh expression. */
  replaceOnInput: boolean;
  status: CalculatorStatus;
  error: CalculatorError | null;
};

/** Edits to the expression. Pure, synchronous, no arithmetic. */
export type EditAction =
  | { type: 'insert'; text: string }
  | { type: 'delete' }
  | { type: 'clear' };

/** What a key can ask for. `evaluate` is async, so the hook intercepts it. */
export type KeyAction = EditAction | { type: 'evaluate' };

/** Everything the reducer handles, including the async lifecycle. */
export type CalculatorAction =
  | EditAction
  | { type: 'evaluating' }
  | { type: 'evaluated'; expression: string; formatted: string }
  | { type: 'failed'; error: CalculatorError };

export type KeyDispatch = (action: KeyAction) => void;
