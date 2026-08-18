import { useCallback, useEffect, useRef, useReducer } from 'react';
import { CalculatorApiError, CalculatorNetworkError, type CalculatorApi } from '../api/client';
import { balance, expressionReducer, initialState, openDepth } from '../core/expression';
import type { CalculatorError, CalculatorStatus, KeyDispatch } from '../types/calculator';

type UseCalculator = {
  /** The expression on screen; after `=` this is the result. */
  expression: string;
  /** The expression that produced the result, shown above it. */
  evaluated: string | null;
  status: CalculatorStatus;
  error: CalculatorError | null;
  /** Handles every key, routing `evaluate` to the API. */
  press: KeyDispatch;
};

function toCalculatorError(error: unknown): CalculatorError {
  if (error instanceof CalculatorApiError) {
    return { code: error.code, message: error.message, position: error.position };
  }
  if (error instanceof CalculatorNetworkError) {
    return { code: error.code, message: error.message };
  }
  return { code: 'UNEXPECTED_ERROR', message: 'Something went wrong' };
}

/**
 * Binds the expression builder to React and to the calculation API.
 *
 * Nothing here computes. Editing is synchronous and local; evaluation is a
 * round trip, so the hook also owns the busy state and the failure mapping.
 */
export function useCalculator(api: CalculatorApi): UseCalculator {
  const [state, dispatch] = useReducer(expressionReducer, initialState);

  // The latest state, readable from callbacks without making them change
  // identity on every keystroke — which would tear down the keyboard listener.
  const latestState = useRef(state);
  useEffect(() => {
    latestState.current = state;
  }, [state]);

  // Only the newest evaluation may write a result. Without this, a slow first
  // request can land after a fast second one and overwrite it.
  const ticket = useRef(0);

  const evaluate = useCallback(async () => {
    const { expression } = latestState.current;
    const submitted = balance(expression);
    if (submitted.trim() === '') return;

    // Show the auto-closed brackets, so what is on screen is what was sent and
    // any reported error position lines up with it.
    if (openDepth(expression) > 0) {
      dispatch({ type: 'insert', text: ')'.repeat(openDepth(expression)) });
    }

    const current = (ticket.current += 1);
    dispatch({ type: 'evaluating' });

    try {
      const response = await api.calculate({ expression: submitted });
      if (current !== ticket.current) return;
      dispatch({ type: 'evaluated', expression: submitted, formatted: response.formatted });
    } catch (error) {
      if (current !== ticket.current) return;
      dispatch({ type: 'failed', error: toCalculatorError(error) });
    }
  }, [api]);

  const press = useCallback<KeyDispatch>(
    (action) => {
      if (action.type === 'evaluate') {
        void evaluate();
        return;
      }
      dispatch(action);
    },
    [evaluate],
  );

  return {
    expression: state.expression,
    evaluated: state.evaluated,
    status: state.status,
    error: state.error,
    press,
  };
}
