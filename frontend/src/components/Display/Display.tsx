import type { ReactNode } from 'react';
import type { CalculatorError, CalculatorStatus } from '../../types/calculator';
import styles from './Display.module.css';

type DisplayProps = {
  /** The expression being composed; after `=` this is the result. */
  expression: string;
  /** The expression that produced the result, shown above it. */
  evaluated: string | null;
  status: CalculatorStatus;
  error: CalculatorError | null;
};

/** Long values step down in size instead of overflowing or wrapping. */
const sizeFor = (value: string): string => {
  if (value.length > 22) return styles.sizeSm;
  if (value.length > 14) return styles.sizeMd;
  return styles.sizeLg;
};

/**
 * Marks the character the server blamed. The API reports an offset with most
 * failures, and pointing at the bad character beats making the user re-read
 * their own expression.
 */
function withErrorMark(expression: string, position?: number): ReactNode {
  if (position === undefined || position < 0 || position >= expression.length) {
    return expression;
  }
  return (
    <>
      {expression.slice(0, position)}
      <mark className={styles.mark}>{expression[position]}</mark>
      {expression.slice(position + 1)}
    </>
  );
}

export function Display({ expression, evaluated, status, error }: DisplayProps) {
  const value = expression === '' ? '0' : expression;
  const isBusy = status === 'evaluating';

  const className = [styles.value, sizeFor(value), isBusy && styles.busy]
    .filter(Boolean)
    .join(' ');

  return (
    <div className={styles.display}>
      <div className={styles.context} aria-hidden="true">
        {evaluated ?? ' '}
      </div>

      <output className={className} aria-live="polite" data-testid="display">
        {error ? withErrorMark(value, error.position) : value}
      </output>

      {/* The message replaces nothing: the expression stays on screen so the
          user can fix it rather than retype it. */}
      <p className={error ? styles.error : styles.status} role={error ? 'alert' : 'status'}>
        {error ? error.message : isBusy ? 'Calculating…' : ' '}
      </p>
    </div>
  );
}
