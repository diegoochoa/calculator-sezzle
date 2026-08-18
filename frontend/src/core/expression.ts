import type { CalculatorAction, CalculatorState } from '../types/calculator';

/**
 * The expression builder: a pure reducer over text.
 *
 * There is deliberately no arithmetic here. Every keypress edits a string, and
 * the Go API is the only thing that knows what that string means. That is what
 * keeps the two halves from drifting apart.
 */

export const initialState: CalculatorState = {
  expression: '',
  result: null,
  evaluated: null,
  replaceOnInput: false,
  status: 'idle',
  error: null,
};

/**
 * Tokens backspace removes whole rather than one character at a time. Deleting
 * the `(` of `sin(` and leaving `sin` behind could only ever be a syntax error.
 *
 * Power shorthands (`^2`, `10^`) are deliberately absent: `3^2` typed by hand
 * ends in the same characters, and removing two of them would surprise.
 */
export const MULTI_CHAR_TOKENS: readonly string[] = [
  'sqrt(',
  'abs(',
  ' + ',
  ' − ',
  ' × ',
  ' ÷ ',
].sort((a, b) => b.length - a.length);

function countOccurrences(text: string, character: string): number {
  let total = 0;
  for (const current of text) if (current === character) total += 1;
  return total;
}

/** How many brackets are still open. */
export function openDepth(expression: string): number {
  return countOccurrences(expression, '(') - countOccurrences(expression, ')');
}

/**
 * `' × '` when appending `text` would put a value straight after another value,
 * `''` otherwise.
 *
 * The grammar has no implicit multiplication, deliberately: `1/2pi` would have
 * to mean either `(1/2)×pi` or `1/(2×pi)` and every choice is wrong for
 * someone. So rather than let the server guess, the keypad writes the operator
 * out — pressing `2` then `log₂` shows `2 × log2(`, and what is on screen is
 * exactly what gets evaluated.
 */
export function productGlue(expression: string, text: string): string {
  const left = expression.slice(-1);
  const right = text.slice(0, 1);

  // A literal still being typed: another digit continues it, a name multiplies.
  const leftIsNumber = /[0-9.]/.test(left);
  // A finished value: `(1+2)`, `50%`, `5!`, or a constant such as `pi`.
  const leftIsClosed = /[)%!a-z]/i.test(left);

  const rightIsName = /[a-z(√]/i.test(right);
  const rightIsNumber = /[0-9.]/.test(right);

  const collides =
    (leftIsNumber && rightIsName) || (leftIsClosed && (rightIsName || rightIsNumber));

  return collides ? ' × ' : '';
}

/** Removes one token, falling back to one character. */
export function deleteLast(expression: string): string {
  for (const token of MULTI_CHAR_TOKENS) {
    if (expression.endsWith(token)) return expression.slice(0, -token.length);
  }
  return expression.slice(0, -1);
}

/** Closes brackets the user left open, so `=` does the obvious thing. */
export function balance(expression: string): string {
  const depth = openDepth(expression);
  return depth > 0 ? expression + ')'.repeat(depth) : expression;
}

/**
 * True when an entry continues from the previous result rather than replacing
 * it — pressing `×` after `= 42` means `42 ×`, while pressing `7` starts over.
 */
function continuesFromResult(text: string): boolean {
  return /^[+\-−×÷*/^!%,]/.test(text.trimStart());
}

/**
 * Decides whether an entry extends the previous result or starts fresh.
 */
function startingPoint(state: CalculatorState, text: string): CalculatorState {
  if (!state.replaceOnInput) return state;

  // Either way the result stops being framed as a result; it is now just the
  // left-hand side of what the user is building, or it is gone.
  return continuesFromResult(text)
    ? { ...state, evaluated: null, result: null }
    : { ...state, expression: '', evaluated: null, result: null };
}

export function expressionReducer(
  state: CalculatorState,
  action: CalculatorAction,
): CalculatorState {
  switch (action.type) {
    case 'clear':
      return initialState;

    case 'insert': {
      const base = startingPoint(state, action.text);
      return {
        ...base,
        expression: base.expression + productGlue(base.expression, action.text) + action.text,
        replaceOnInput: false,
        error: null,
      };
    }

    case 'delete': {
      // A displayed result is cleared whole rather than edited character by
      // character — half a result is not a meaningful thing to hold.
      if (state.replaceOnInput) {
        return {
          ...state,
          expression: '',
          result: null,
          evaluated: null,
          replaceOnInput: false,
          error: null,
        };
      }
      return { ...state, expression: deleteLast(state.expression), error: null };
    }

    case 'evaluating':
      return { ...state, status: 'evaluating', error: null };

    case 'evaluated':
      return {
        ...state,
        status: 'idle',
        expression: action.formatted,
        evaluated: action.expression,
        result: action.formatted,
        replaceOnInput: true,
        error: null,
      };

    case 'failed':
      return { ...state, status: 'idle', error: action.error };
  }
}
