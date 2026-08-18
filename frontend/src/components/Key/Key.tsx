import type { KeyAction } from '../../types/calculator';
import type { KeyFace } from '../../types/keys';
import styles from './Key.module.css';

type KeyProps = {
  /** The label and action currently presented by this key. */
  face: KeyFace;
  /** Secondary label printed above the key, the way it is on a real calculator. */
  hint?: string;
  onPress: (action: KeyAction) => void;
  /** Momentary flash while the matching physical key is held. */
  isPressed?: boolean;
  disabled?: boolean;
};

export function Key({ face, hint, onPress, isPressed = false, disabled = false }: KeyProps) {
  const className = [
    styles.key,
    styles[face.variant],
    face.span !== undefined && styles.spanning,
    isPressed && styles.pressed,
  ]
    .filter(Boolean)
    .join(' ');

  return (
    <button
      type="button"
      className={className}
      style={face.span ? { gridColumn: `span ${face.span}` } : undefined}
      onClick={() => onPress(face.action)}
      aria-label={face.ariaLabel}
      disabled={disabled}
      data-testid={face.id}
    >
      {hint !== undefined && (
        <span className={styles.hint} aria-hidden="true">
          {hint}
        </span>
      )}
      <span aria-hidden="true">{face.label}</span>
    </button>
  );
}
