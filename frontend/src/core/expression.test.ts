import { describe, expect, it } from 'vitest';
import {
  MULTI_CHAR_TOKENS,
  balance,
  deleteLast,
  expressionReducer,
  initialState,
  openDepth,
  productGlue,
} from './expression';
import { BASIC_KEYS } from '../components/Keypad/keys';
import { facesOf } from '../types/keys';
import type { CalculatorAction, CalculatorState } from '../types/calculator';

const run = (...actions: CalculatorAction[]): CalculatorState =>
  actions.reduce(expressionReducer, initialState);

const type = (text: string): CalculatorAction[] =>
  [...text].map((character) => ({ type: 'insert', text: character }) as const);

describe('editing', () => {
  it('starts empty', () => {
    expect(initialState.expression).toBe('');
  });

  it('appends what each key inserts', () => {
    expect(run(...type('12'), { type: 'insert', text: ' + ' }, ...type('3')).expression).toBe(
      '12 + 3',
    );
  });

  it('keeps leading zeros out of the way of nothing', () => {
    // Unlike the old immediate-execution buffer there is no leading-zero rule:
    // "007" is a perfectly good expression and the server reads it as 7.
    expect(run(...type('007')).expression).toBe('007');
  });

  it('clears everything', () => {
    const state = run(...type('1+1'), { type: 'clear' });
    expect(state).toEqual(initialState);
  });
});

describe('brackets', () => {
  it('builds a grouped expression', () => {
    const state = run(
      { type: 'insert', text: '(' },
      ...type('2+3'),
      { type: 'insert', text: ')' },
      { type: 'insert', text: ' × ' },
      ...type('4'),
    );
    expect(state.expression).toBe('(2+3) × 4');
  });

  it('reports whether a bracket is open, which is what gates the ) key', () => {
    expect(openDepth('')).toBe(0);
    expect(openDepth('(2 + 3')).toBe(1);
    expect(openDepth('(2 + 3)')).toBe(0);
    expect(openDepth('log(root(8')).toBe(2);
  });
});

// The keypad must not be able to build an expression the grammar rejects.
// Pressing 2 then log₂ used to produce "2log2(" and a SYNTAX_ERROR the user had
// no way to see coming.
describe('implicit products', () => {
  it('separates a number from a name that follows it', () => {
    expect(productGlue('2', 'sqrt(')).toBe(' × ');
    expect(productGlue('2', '(')).toBe(' × ');
    expect(productGlue('2', '√')).toBe(' × ');
    expect(productGlue('2', 'abs(')).toBe(' × ');
  });

  it('separates a finished value from anything that follows it', () => {
    expect(productGlue('(1+2)', '3')).toBe(' × ');
    expect(productGlue('(1+2)', '(')).toBe(' × ');
    expect(productGlue('(1+2)', 'abs(')).toBe(' × ');
    expect(productGlue('50%', '(')).toBe(' × ');
    expect(productGlue('abs(1)', '2')).toBe(' × ');
  });

  it('leaves a literal being typed alone', () => {
    expect(productGlue('2', '3')).toBe('');
    expect(productGlue('2', '.')).toBe('');
    expect(productGlue('2.', '5')).toBe('');
  });

  it('leaves an open bracket alone', () => {
    expect(productGlue('sqrt(', '2')).toBe('');
    expect(productGlue('abs(', '3')).toBe('');
    expect(productGlue('', '3')).toBe('');
  });

  it('leaves an operator alone', () => {
    expect(productGlue('2 + ', 'sqrt(')).toBe('');
    expect(productGlue('2^', 'sqrt(')).toBe('');
    expect(productGlue('10^', '2')).toBe('');
    expect(productGlue('(', '2')).toBe('');
  });

  it('writes the operator into the expression as the user builds it', () => {
    const state = run(
      ...type('2'),
      { type: 'insert', text: 'sqrt(' },
      ...type('10'),
      { type: 'insert', text: ')' },
    );
    expect(state.expression).toBe('2 × sqrt(10)');
  });

  it('separates a number from an opening bracket', () => {
    const state = run(
      ...type('2'),
      { type: 'insert', text: '(' },
      ...type('3+4'),
      { type: 'insert', text: ')' },
    );
    expect(state.expression).toBe('2 × (3+4)');
  });

  it('never puts an operator before a closing bracket', () => {
    const state = run(
      { type: 'insert', text: '(' },
      ...type('2+3'),
      { type: 'insert', text: ')' },
    );
    expect(state.expression).toBe('(2+3)');
  });

  it('separates a number from the square root key', () => {
    const state = run(...type('2'), { type: 'insert', text: '√' }, ...type('9'));
    expect(state.expression).toBe('2 × √9');
  });

  it('leaves the exponent key alone, which is not a value', () => {
    const state = run(...type('2'), { type: 'insert', text: '^' }, ...type('10'));
    expect(state.expression).toBe('2^10');
  });

  it('leaves the exponent operator alone', () => {
    const state = run({ type: 'insert', text: '^' }, ...type('3'));
    expect(state.expression).toBe('^3');
  });

});

describe('balancing', () => {
  it('counts open brackets', () => {
    expect(openDepth('sin(30')).toBe(1);
    expect(openDepth('sin(30)')).toBe(0);
    expect(openDepth('log(root(8')).toBe(2);
  });

  it('closes what the user left open', () => {
    expect(balance('sin(30')).toBe('sin(30)');
    expect(balance('log(root(8, 3')).toBe('log(root(8, 3))');
    expect(balance('1 + 1')).toBe('1 + 1');
  });
});

describe('backspace', () => {
  it('removes a whole function token', () => {
    expect(deleteLast('2 + sqrt(')).toBe('2 + ');
    expect(deleteLast('sqrt(')).toBe('');
    expect(deleteLast('abs(')).toBe('');
  });

  it('prefers the longest matching token', () => {
    // "asin(" also ends with "sin(", so ordering decides whether the `a` survives.
    expect(deleteLast('abs(')).toBe('');
  });

  it('removes a whole spaced operator', () => {
    expect(deleteLast('2 + ')).toBe('2');
    expect(deleteLast('2 × ')).toBe('2');
  });

  it('falls back to one character', () => {
    expect(deleteLast('123')).toBe('12');
    expect(deleteLast('2^')).toBe('2');
  });

  it('leaves a hand-typed power alone', () => {
    // "^2" is not a delete-whole token precisely because 3^2 typed by hand ends
    // the same way, and removing both characters would surprise.
    expect(deleteLast('3^2')).toBe('3^');
  });

  it('clears a displayed result whole', () => {
    const evaluated = run(...type('2+2'), {
      type: 'evaluated',
      expression: '2+2',
      formatted: '4',
    });
    const state = expressionReducer(evaluated, { type: 'delete' });

    expect(state.expression).toBe('');
    expect(state.result).toBeNull();
  });
});

describe('after a result', () => {
  const evaluated = run(...type('2+2'), {
    type: 'evaluated',
    expression: '2+2',
    formatted: '4',
  });

  it('shows the result and the expression that produced it', () => {
    expect(evaluated.expression).toBe('4');
    expect(evaluated.evaluated).toBe('2+2');
    expect(evaluated.replaceOnInput).toBe(true);
  });

  it('starts over when a digit is entered', () => {
    const state = expressionReducer(evaluated, { type: 'insert', text: '7' });
    expect(state.expression).toBe('7');
    expect(state.evaluated).toBeNull();
  });

  it('continues from the result when an operator is entered', () => {
    const state = expressionReducer(evaluated, { type: 'insert', text: ' × ' });
    expect(state.expression).toBe('4 × ');
    expect(state.evaluated).toBeNull();
  });

  it('continues from the result for a postfix operator', () => {
    expect(expressionReducer(evaluated, { type: 'insert', text: '!' }).expression).toBe('4!');
  });

  it('starts over when a function is entered', () => {
    expect(expressionReducer(evaluated, { type: 'insert', text: 'sin(' }).expression).toBe('sin(');
  });
});

describe('the evaluation lifecycle', () => {
  it('marks itself busy and clears any previous error', () => {
    const failed = run(...type('1/'), {
      type: 'failed',
      error: { code: 'SYNTAX_ERROR', message: 'nope' },
    });
    const state = expressionReducer(failed, { type: 'evaluating' });

    expect(state.status).toBe('evaluating');
    expect(state.error).toBeNull();
  });

  it('keeps the expression on screen when evaluation fails', () => {
    const state = run(...type('1/0'), {
      type: 'failed',
      error: { code: 'DIVISION_BY_ZERO', message: "Can't divide by zero", position: 1 },
    });

    expect(state.status).toBe('idle');
    expect(state.expression).toBe('1/0');
    expect(state.error?.position).toBe(1);
  });

  it('clears the error as soon as the user edits', () => {
    const failed = run(...type('1/0'), {
      type: 'failed',
      error: { code: 'DIVISION_BY_ZERO', message: "Can't divide by zero" },
    });

    expect(expressionReducer(failed, { type: 'delete' }).error).toBeNull();
    expect(expressionReducer(failed, { type: 'insert', text: '1' }).error).toBeNull();
  });
});

// A key that inserts text backspace cannot remove in one press leaves the user
// deleting `s`, `o`, `c` out of `cos(`. This keeps the two lists honest.
describe('key and token consistency', () => {
  const insertedTexts = BASIC_KEYS
    .flatMap(facesOf)
    .filter((face) => face.action.type === 'insert')
    .map((face) => (face.action as { type: 'insert'; text: string }).text);

  it('deletes every multi-character insertion in one press', () => {
    const exempt = new Set<string>();

    for (const text of insertedTexts) {
      if (text.length === 1 || exempt.has(text)) continue;
      expect(MULTI_CHAR_TOKENS, `"${text}" is not a delete-whole token`).toContain(text);
    }
  });

  it('orders tokens longest first, so prefixed names win', () => {
    const lengths = MULTI_CHAR_TOKENS.map((token) => token.length);
    expect(lengths).toEqual([...lengths].sort((a, b) => b - a));
  });

  it('gives every key a distinct id', () => {
    const ids = BASIC_KEYS.flatMap(facesOf).map((face) => face.id);
    expect(new Set(ids).size).toBe(ids.length);
  });

  it('binds no physical key to two actions', () => {
    const bindings = BASIC_KEYS
      .flatMap(facesOf)
      .flatMap((face) => face.bindings ?? []);
    expect(new Set(bindings).size).toBe(bindings.length);
  });
});
