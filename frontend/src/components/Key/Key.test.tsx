import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { Key } from './Key';
import type { KeyFace } from '../../types/keys';

const face: KeyFace = {
  id: 'digit-7',
  label: '7',
  ariaLabel: 'Seven',
  action: { type: 'insert', text: '7' },
  variant: 'digit',
};

describe('Key', () => {
  it('renders the label and dispatches the action', async () => {
    const onPress = vi.fn();
    render(<Key face={face} onPress={onPress} />);

    const button = screen.getByTestId('digit-7');
    expect(button).toHaveTextContent('7');
    expect(button).toHaveAttribute('aria-label', 'Seven');

    await userEvent.click(button);
    expect(onPress).toHaveBeenCalledWith({ type: 'insert', text: '7' });
  });

  it('prints the secondary label as a hint', () => {
    render(<Key face={face} hint="sin⁻¹" onPress={vi.fn()} />);
    expect(screen.getByTestId('digit-7')).toHaveTextContent('sin⁻¹');
  });

  it('does not fire while disabled', async () => {
    const onPress = vi.fn();
    render(<Key face={face} onPress={onPress} disabled />);

    const button = screen.getByTestId('digit-7');
    expect(button).toBeDisabled();

    await userEvent.click(button);
    expect(onPress).not.toHaveBeenCalled();
  });

  // A spanning key must widen without also growing taller, which is what the
  // square aspect ratio would otherwise do to it.
  it('spans columns only when asked to', () => {
    const { rerender } = render(<Key face={face} onPress={vi.fn()} />);
    expect(screen.getByTestId('digit-7')).not.toHaveStyle({ gridColumn: 'span 2' });

    rerender(<Key face={{ ...face, span: 2 }} onPress={vi.fn()} />);
    expect(screen.getByTestId('digit-7')).toHaveStyle({ gridColumn: 'span 2' });
  });

  it('is a real button, so it is reachable by keyboard', () => {
    render(<Key face={face} onPress={vi.fn()} />);
    expect(screen.getByRole('button', { name: 'Seven' })).toHaveAttribute('type', 'button');
  });
});
