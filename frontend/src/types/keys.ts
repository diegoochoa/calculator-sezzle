import type { KeyAction } from './calculator';

/**
 * `compact` is the shorter, muted key used for secondary operations — the
 * function row on the basic pad and every key on the scientific pad.
 */
export type KeyVariant = 'digit' | 'function' | 'operator' | 'equals' | 'compact';

/** What a key shows and does in one of its two states. */
export type KeyFace = {
  id: string;
  label: string;
  /** Announced by screen readers instead of the glyph. */
  ariaLabel: string;
  action: KeyAction;
  variant: KeyVariant;
  /** Grid columns to span. Only `0` uses this, the way a phone keypad does. */
  span?: number;
  /** Physical keys that trigger this face. */
  bindings?: string[];
};

export type KeyDefinition = KeyFace & {
  /** Alternate face revealed by the `2nd` toggle. */
  secondary?: KeyFace;
};

/** Every face a key can present, so both are reachable from the keyboard. */
export const facesOf = (key: KeyDefinition): KeyFace[] =>
  key.secondary ? [key, key.secondary] : [key];
