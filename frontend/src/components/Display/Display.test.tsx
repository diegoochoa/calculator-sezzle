import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { Display } from './Display';

const base = { expression: '', evaluated: null, status: 'idle' as const, error: null };

describe('Display', () => {
  it('shows a zero when nothing has been entered', () => {
    render(<Display {...base} />);
    expect(screen.getByTestId('display')).toHaveTextContent('0');
  });

  it('shows the expression being composed', () => {
    render(<Display {...base} expression="2 + 3 × 4" />);
    expect(screen.getByTestId('display')).toHaveTextContent('2 + 3 × 4');
  });

  it('shows the result over the expression that produced it', () => {
    render(<Display {...base} expression="14" evaluated="2 + 3 × 4" />);

    expect(screen.getByTestId('display')).toHaveTextContent('14');
    expect(screen.getByText('2 + 3 × 4')).toBeInTheDocument();
  });

  // `<output>` already carries an implicit role of status, so the progress line
  // is found by its text rather than by role — there are two status regions on
  // screen and that is intentional: the value and its progress.
  it('announces progress while a request is in flight', () => {
    render(<Display {...base} expression="1+1" status="evaluating" />);
    expect(screen.getByText('Calculating…')).toHaveAttribute('role', 'status');
  });

  it('announces a failure assertively', () => {
    render(
      <Display
        {...base}
        expression="1/0"
        error={{ code: 'DIVISION_BY_ZERO', message: "Can't divide by zero" }}
      />,
    );

    expect(screen.getByRole('alert')).toHaveTextContent("Can't divide by zero");
    // The expression stays put so it can be fixed rather than retyped.
    expect(screen.getByTestId('display')).toHaveTextContent('1/0');
  });

  // The server reports the offset with most failures; pointing at the character
  // beats making the user re-read their own expression.
  it('marks the character the server blamed', () => {
    const { container } = render(
      <Display
        {...base}
        expression="100 + 1/0"
        error={{ code: 'DIVISION_BY_ZERO', message: 'nope', position: 7 }}
      />,
    );

    const mark = container.querySelector('mark');
    expect(mark).not.toBeNull();
    expect(mark).toHaveTextContent('/');
  });

  it('renders plainly when the offset is out of range', () => {
    const { container } = render(
      <Display {...base} expression="1/0" error={{ code: 'X', message: 'nope', position: 99 }} />,
    );

    expect(container.querySelector('mark')).toBeNull();
    expect(screen.getByTestId('display')).toHaveTextContent('1/0');
  });

  it('renders no mark when the server gave no offset', () => {
    const { container } = render(
      <Display {...base} expression="1/0" error={{ code: 'X', message: 'nope' }} />,
    );
    expect(container.querySelector('mark')).toBeNull();
  });

  // Results are announced to a screen reader as they arrive.
  it('marks the value as a live region', () => {
    render(<Display {...base} expression="14" />);
    expect(screen.getByTestId('display')).toHaveAttribute('aria-live', 'polite');
  });
});
