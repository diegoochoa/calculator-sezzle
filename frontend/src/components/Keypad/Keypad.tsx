import { Key } from '../Key';
import { BASIC_KEYS } from './keys';
import type { KeyDispatch } from '../../types/calculator';
import type { KeyDefinition } from '../../types/keys';
import styles from './Keypad.module.css';

type KeypadProps = {
  onPress: KeyDispatch;
  /** Key id currently held on the physical keyboard. */
  pressedKeyId: string | null;
  /** Set while an evaluation is in flight; only `=` is blocked. */
  isEvaluating?: boolean;
  /** False when no bracket is open, which is when `)` could only be an error. */
  canCloseBracket?: boolean;
};

export function Keypad({
  onPress,
  pressedKeyId,
  isEvaluating = false,
  canCloseBracket = false,
}: KeypadProps) {
  // The keypad should not be able to build an expression the grammar rejects,
  // so a bracket with nothing to close is unavailable rather than an error
  // waiting to happen.
  const isDisabled = (definition: KeyDefinition): boolean => {
    if (definition.action.type === 'evaluate') return isEvaluating;
    if (definition.id === 'paren-close') return !canCloseBracket;
    return false;
  };

  return (
    <div className={styles.keypad} role="group" aria-label="Calculator keypad">
      {BASIC_KEYS.map((definition) => (
        <Key
          key={definition.id}
          face={definition}
          onPress={onPress}
          isPressed={definition.id === pressedKeyId}
          disabled={isDisabled(definition)}
        />
      ))}
    </div>
  );
}
