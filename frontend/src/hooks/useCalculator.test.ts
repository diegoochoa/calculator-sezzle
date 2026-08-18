import { act, renderHook } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { useCalculator } from './useCalculator';
import { CalculatorApiError, CalculatorNetworkError, type CalculatorApi } from '../api/client';
import type { CalculateResponse } from '../api/types';

const response = (overrides: Partial<CalculateResponse> = {}): CalculateResponse => ({
  result: 14,
  formatted: '14',
  expression: '2 + 3 × 4',
  ...overrides,
});

/** An API whose calls resolve immediately with `formatted`. */
function stubApi(formatted = '14'): CalculatorApi & { calculate: ReturnType<typeof vi.fn> } {
  return { calculate: vi.fn(async () => response({ formatted })) };
}

/** An API whose calls stay pending until the test resolves them by index. */
function deferredApi() {
  const resolvers: ((value: CalculateResponse) => void)[] = [];
  const rejecters: ((reason: unknown) => void)[] = [];

  const calculate = vi.fn(
    () =>
      new Promise<CalculateResponse>((resolve, reject) => {
        resolvers.push(resolve);
        rejecters.push(reject);
      }),
  );

  return { api: { calculate } as CalculatorApi, calculate, resolvers, rejecters };
}

const type = async (
  press: (action: { type: 'insert'; text: string }) => void,
  text: string,
) => {
  await act(async () => {
    for (const character of text) press({ type: 'insert', text: character });
  });
};

describe('editing', () => {
  it('appends what each key inserts', async () => {
    const { result } = renderHook(() => useCalculator(stubApi()));

    await type(result.current.press, '12');
    expect(result.current.expression).toBe('12');
  });

  it('starts idle with nothing on screen', () => {
    const { result } = renderHook(() => useCalculator(stubApi()));

    expect(result.current.expression).toBe('');
    expect(result.current.status).toBe('idle');
    expect(result.current.error).toBeNull();
  });

});

describe('evaluating', () => {
  it('submits the expression and shows the formatted result', async () => {
    const api = stubApi('14');
    const { result } = renderHook(() => useCalculator(api));

    await type(result.current.press, '2+3');
    await act(async () => result.current.press({ type: 'evaluate' }));

    expect(api.calculate).toHaveBeenCalledWith({ expression: '2+3' });
    expect(result.current.expression).toBe('14');
    expect(result.current.evaluated).toBe('2+3');
    expect(result.current.status).toBe('idle');
  });

  it('does nothing when there is no expression', async () => {
    const api = stubApi();
    const { result } = renderHook(() => useCalculator(api));

    await act(async () => result.current.press({ type: 'evaluate' }));

    expect(api.calculate).not.toHaveBeenCalled();
    expect(result.current.status).toBe('idle');
  });

  it('reports itself busy while the request is in flight', async () => {
    const { api, resolvers } = deferredApi();
    const { result } = renderHook(() => useCalculator(api));

    await type(result.current.press, '1+1');
    act(() => result.current.press({ type: 'evaluate' }));

    expect(result.current.status).toBe('evaluating');

    await act(async () => resolvers[0](response({ formatted: '2' })));
    expect(result.current.status).toBe('idle');
    expect(result.current.expression).toBe('2');
  });

  // What is on screen must be what was sent, or the error offsets the server
  // returns would point at the wrong character.
  it('closes open brackets and shows them before submitting', async () => {
    const api = stubApi('0.5');
    const { result } = renderHook(() => useCalculator(api));

    await act(async () => result.current.press({ type: 'insert', text: 'sqrt(' }));
    await type(result.current.press, '30');
    await act(async () => result.current.press({ type: 'evaluate' }));

    expect(api.calculate).toHaveBeenCalledWith({ expression: 'sqrt(30)' });
    expect(result.current.evaluated).toBe('sqrt(30)');
  });
});

// A slow first request landing after a fast second one would otherwise
// overwrite it — a bug that never appears when clicking by hand.
describe('out-of-order responses', () => {
  it('keeps only the newest result', async () => {
    const { api, resolvers } = deferredApi();
    const { result } = renderHook(() => useCalculator(api));

    await type(result.current.press, '1');
    act(() => result.current.press({ type: 'evaluate' }));
    act(() => result.current.press({ type: 'evaluate' }));

    // Second request answers first, then the stale first one arrives.
    await act(async () => resolvers[1](response({ formatted: 'second' })));
    await act(async () => resolvers[0](response({ formatted: 'first' })));

    expect(result.current.expression).toBe('second');
  });

  it('ignores a stale failure', async () => {
    const { api, resolvers, rejecters } = deferredApi();
    const { result } = renderHook(() => useCalculator(api));

    await type(result.current.press, '1');
    act(() => result.current.press({ type: 'evaluate' }));
    act(() => result.current.press({ type: 'evaluate' }));

    await act(async () => resolvers[1](response({ formatted: 'fresh' })));
    await act(async () => {
      rejecters[0](new CalculatorNetworkError('stale failure'));
    });

    expect(result.current.error).toBeNull();
    expect(result.current.expression).toBe('fresh');
  });
});

describe('failures', () => {
  it('surfaces a server error with its code and position', async () => {
    const { api, rejecters } = deferredApi();
    const { result } = renderHook(() => useCalculator(api));

    await type(result.current.press, '1/0');
    act(() => result.current.press({ type: 'evaluate' }));
    await act(async () => {
      rejecters[0](
        new CalculatorApiError('DIVISION_BY_ZERO', "Can't divide by zero", 422, 1, 'req-1'),
      );
    });

    expect(result.current.status).toBe('idle');
    expect(result.current.error).toEqual({
      code: 'DIVISION_BY_ZERO',
      message: "Can't divide by zero",
      position: 1,
    });
    // The expression stays put so it can be fixed rather than retyped.
    expect(result.current.expression).toBe('1/0');
  });

  it('surfaces an unreachable service', async () => {
    const { api, rejecters } = deferredApi();
    const { result } = renderHook(() => useCalculator(api));

    await type(result.current.press, '1+1');
    act(() => result.current.press({ type: 'evaluate' }));
    await act(async () => {
      rejecters[0](new CalculatorNetworkError("Can't reach the calculator service"));
    });

    expect(result.current.error?.code).toBe('NETWORK_ERROR');
  });

  it('does not leak an unexpected throw to the user', async () => {
    const { api, rejecters } = deferredApi();
    const { result } = renderHook(() => useCalculator(api));

    await type(result.current.press, '1+1');
    act(() => result.current.press({ type: 'evaluate' }));
    await act(async () => {
      rejecters[0](new TypeError('x.y is not a function'));
    });

    expect(result.current.error).toEqual({
      code: 'UNEXPECTED_ERROR',
      message: 'Something went wrong',
    });
  });

  it('clears the error as soon as the user edits', async () => {
    const { api, rejecters } = deferredApi();
    const { result } = renderHook(() => useCalculator(api));

    await type(result.current.press, '1/0');
    act(() => result.current.press({ type: 'evaluate' }));
    await act(async () => {
      rejecters[0](new CalculatorApiError('DIVISION_BY_ZERO', 'nope', 422));
    });
    expect(result.current.error).not.toBeNull();

    await act(async () => result.current.press({ type: 'delete' }));
    expect(result.current.error).toBeNull();
  });
});
