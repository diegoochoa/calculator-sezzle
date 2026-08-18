import { useEffect, useMemo, useState } from 'react';
import type { KeyDispatch } from '../types/calculator';
import { facesOf, type KeyDefinition } from '../types/keys';

/**
 * Routes physical keystrokes through the same actions as the on-screen keys and
 * reports which face is being pressed so the UI can flash it. Only the keys
 * currently on screen are bound, so scientific shortcuts stay inert in basic
 * mode.
 */
export function useKeyboardInput(press: KeyDispatch, keys: KeyDefinition[]): string | null {
  const [pressedKeyId, setPressedKeyId] = useState<string | null>(null);

  const bindings = useMemo(
    () =>
      new Map(
        keys
          .flatMap(facesOf)
          .flatMap((face) => (face.bindings ?? []).map((binding) => [binding, face] as const)),
      ),
    [keys],
  );

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.metaKey || event.ctrlKey || event.altKey) return;

      const face = bindings.get(event.key);
      if (!face) return;

      // Enter would otherwise re-trigger the last focused button.
      event.preventDefault();
      press(face.action);
      setPressedKeyId(face.id);
    };

    const handleKeyUp = () => setPressedKeyId(null);

    window.addEventListener('keydown', handleKeyDown);
    window.addEventListener('keyup', handleKeyUp);
    window.addEventListener('blur', handleKeyUp);

    return () => {
      window.removeEventListener('keydown', handleKeyDown);
      window.removeEventListener('keyup', handleKeyUp);
      window.removeEventListener('blur', handleKeyUp);
    };
  }, [bindings, press]);

  return pressedKeyId;
}
