import { Display } from '../Display';
import { Keypad, BASIC_KEYS } from '../Keypad';
import { openDepth } from '../../core/expression';
import { useCalculator } from '../../hooks/useCalculator';
import { useKeyboardInput } from '../../hooks/useKeyboardInput';
import type { CalculatorApi } from '../../api/client';
import styles from './Calculator.module.css';

type CalculatorProps = {
  api: CalculatorApi;
};

/**
 * Container component: owns the expression state and wires input sources
 * (keypad + physical keyboard) to the presentational pieces. Evaluation is a
 * round trip to the API — nothing in this tree does arithmetic.
 */
export function Calculator({ api }: CalculatorProps) {
  const { expression, evaluated, status, error, press } = useCalculator(api);
  const pressedKeyId = useKeyboardInput(press, BASIC_KEYS);

  return (
    <section className={styles.calculator} aria-label="Calculator">
      <Display expression={expression} evaluated={evaluated} status={status} error={error} />

      <Keypad
        onPress={press}
        pressedKeyId={pressedKeyId}
        isEvaluating={status === 'evaluating'}
        canCloseBracket={openDepth(expression) > 0}
      />
    </section>
  );
}
