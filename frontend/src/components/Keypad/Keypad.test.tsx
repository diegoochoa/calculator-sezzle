import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { Keypad } from './Keypad';
import { BASIC_KEYS } from './keys';

const renderKeypad = (props: Partial<Parameters<typeof Keypad>[0]> = {}) => {
  const onPress = vi.fn();
  render(<Keypad onPress={onPress} pressedKeyId={null} {...props} />);
  return { onPress };
};

describe('Keypad', () => {
  it('renders every key', () => {
    renderKeypad();
    for (const key of BASIC_KEYS) {
      expect(screen.getByTestId(key.id)).toBeInTheDocument();
    }
  });

  it('dispatches the action a key carries', async () => {
    const { onPress } = renderKeypad();

    await userEvent.click(screen.getByTestId('digit-7'));
    expect(onPress).toHaveBeenCalledWith({ type: 'insert', text: '7' });

    await userEvent.click(screen.getByTestId('add'));
    expect(onPress).toHaveBeenCalledWith({ type: 'insert', text: ' + ' });
  });

  it('exposes the promoted square root and power keys', async () => {
    const { onPress } = renderKeypad();

    await userEvent.click(screen.getByTestId('sqrt'));
    expect(onPress).toHaveBeenCalledWith({ type: 'insert', text: '√' });

    await userEvent.click(screen.getByTestId('power'));
    expect(onPress).toHaveBeenCalledWith({ type: 'insert', text: '^' });
  });

  // The keypad must not be able to build an expression the grammar rejects, so
  // a bracket with nothing to close is unavailable rather than an error waiting
  // to happen.
  it('disables the closing bracket while nothing is open', () => {
    renderKeypad({ canCloseBracket: false });
    expect(screen.getByTestId('paren-close')).toBeDisabled();
    expect(screen.getByTestId('paren-open')).toBeEnabled();
  });

  it('enables the closing bracket once one is open', () => {
    renderKeypad({ canCloseBracket: true });
    expect(screen.getByTestId('paren-close')).toBeEnabled();
  });

  it('blocks a second submission mid-request but keeps editing live', () => {
    renderKeypad({ isEvaluating: true });

    expect(screen.getByTestId('equals')).toBeDisabled();
    expect(screen.getByTestId('digit-7')).toBeEnabled();
    expect(screen.getByTestId('delete')).toBeEnabled();
  });

  it('allows submission when idle', () => {
    renderKeypad({ isEvaluating: false });
    expect(screen.getByTestId('equals')).toBeEnabled();
  });

  it('labels every key for a screen reader', () => {
    renderKeypad();
    for (const key of BASIC_KEYS) {
      expect(screen.getByTestId(key.id)).toHaveAttribute('aria-label', key.ariaLabel);
    }
  });
});
