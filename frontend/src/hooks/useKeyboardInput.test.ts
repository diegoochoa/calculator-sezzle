import { act, renderHook } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { useKeyboardInput } from './useKeyboardInput';
import type { KeyDefinition } from '../types/keys';

const KEYS: KeyDefinition[] = [
  {
    id: 'digit-7',
    label: '7',
    ariaLabel: '7',
    action: { type: 'insert', text: '7' },
    variant: 'digit',
    bindings: ['7'],
  },
  {
    id: 'equals',
    label: '=',
    ariaLabel: 'Evaluate',
    action: { type: 'evaluate' },
    variant: 'equals',
    bindings: ['Enter', '='],
  },
  {
    id: 'power',
    label: 'xʸ',
    ariaLabel: 'Power',
    action: { type: 'insert', text: '^' },
    variant: 'compact',
    bindings: ['^'],
    secondary: {
      id: 'sqrt',
      label: '√',
      ariaLabel: 'Square root',
      action: { type: 'insert', text: '√' },
      variant: 'compact',
      bindings: ['q'],
    },
  },
];

function press(key: string, init: KeyboardEventInit = {}) {
  const event = new KeyboardEvent('keydown', { key, cancelable: true, ...init });
  act(() => {
    window.dispatchEvent(event);
  });
  return event;
}

describe('useKeyboardInput', () => {
  it('dispatches the action bound to a key', () => {
    const dispatch = vi.fn();
    renderHook(() => useKeyboardInput(dispatch, KEYS));

    press('7');
    expect(dispatch).toHaveBeenCalledWith({ type: 'insert', text: '7' });
  });

  it('honours every binding on a face', () => {
    const dispatch = vi.fn();
    renderHook(() => useKeyboardInput(dispatch, KEYS));

    press('Enter');
    press('=');
    expect(dispatch).toHaveBeenCalledTimes(2);
    expect(dispatch).toHaveBeenLastCalledWith({ type: 'evaluate' });
  });

  // Both faces are reachable, so a 2nd-function shortcut works without the
  // toggle being on screen.
  it('binds secondary faces too', () => {
    const dispatch = vi.fn();
    renderHook(() => useKeyboardInput(dispatch, KEYS));

    press('q');
    expect(dispatch).toHaveBeenCalledWith({ type: 'insert', text: '√' });
  });

  it('ignores keys nothing is bound to', () => {
    const dispatch = vi.fn();
    renderHook(() => useKeyboardInput(dispatch, KEYS));

    press('z');
    expect(dispatch).not.toHaveBeenCalled();
  });

  // Otherwise Cmd+R would type a digit instead of reloading.
  it('ignores modified keystrokes', () => {
    const dispatch = vi.fn();
    renderHook(() => useKeyboardInput(dispatch, KEYS));

    press('7', { metaKey: true });
    press('7', { ctrlKey: true });
    press('7', { altKey: true });
    expect(dispatch).not.toHaveBeenCalled();
  });

  // Enter would otherwise re-trigger whichever button was last focused.
  it('prevents the default for a bound key', () => {
    renderHook(() => useKeyboardInput(vi.fn(), KEYS));

    expect(press('Enter').defaultPrevented).toBe(true);
    expect(press('z').defaultPrevented).toBe(false);
  });

  it('reports the pressed face, then clears it', () => {
    const { result } = renderHook(() => useKeyboardInput(vi.fn(), KEYS));

    press('7');
    expect(result.current).toBe('digit-7');

    act(() => {
      window.dispatchEvent(new KeyboardEvent('keyup', { key: '7' }));
    });
    expect(result.current).toBeNull();
  });

  // A key held while the tab loses focus would otherwise stay lit forever.
  it('clears the pressed face when the window blurs', () => {
    const { result } = renderHook(() => useKeyboardInput(vi.fn(), KEYS));

    press('7');
    expect(result.current).toBe('digit-7');

    act(() => {
      window.dispatchEvent(new Event('blur'));
    });
    expect(result.current).toBeNull();
  });

  it('stops listening once unmounted', () => {
    const dispatch = vi.fn();
    const { unmount } = renderHook(() => useKeyboardInput(dispatch, KEYS));

    unmount();
    press('7');
    expect(dispatch).not.toHaveBeenCalled();
  });

  it('rebinds when the visible keys change', () => {
    const dispatch = vi.fn();
    const { rerender } = renderHook(({ keys }) => useKeyboardInput(dispatch, keys), {
      initialProps: { keys: [] as KeyDefinition[] },
    });

    press('7');
    expect(dispatch).not.toHaveBeenCalled();

    rerender({ keys: KEYS });
    press('7');
    expect(dispatch).toHaveBeenCalledWith({ type: 'insert', text: '7' });
  });
});
