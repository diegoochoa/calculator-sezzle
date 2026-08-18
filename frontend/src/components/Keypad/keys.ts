import type { KeyDefinition } from '../../types/keys';

const digit = (value: string, span?: number): KeyDefinition => ({
  id: `digit-${value}`,
  label: value,
  ariaLabel: value,
  action: { type: 'insert', text: value },
  variant: 'digit',
  span,
  bindings: [value],
});

/**
 * The always-visible pad, laid out in six rows of four.
 *
 * The top row carries the operations that are not pure arithmetic — square
 * root, exponentiation and the brackets. Brackets are explicit rather than one
 * smart key now that there is room for both: closing early or opening a nested
 * group deliberately is worth more than saving a slot.
 *
 * There is no `+/−` key. The server's grammar has a real unary minus, so `−` at
 * the start of a term already negates it.
 */
export const BASIC_KEYS: KeyDefinition[] = [
  {
    id: 'sqrt',
    label: '√',
    ariaLabel: 'Square root',
    action: { type: 'insert', text: '√' },
    variant: 'compact',
    bindings: ['q', 'Q'],
  },
  {
    id: 'power',
    label: 'xʸ',
    ariaLabel: 'To the power of',
    action: { type: 'insert', text: '^' },
    variant: 'compact',
    bindings: ['^'],
  },
  {
    id: 'paren-open',
    label: '(',
    ariaLabel: 'Open bracket',
    action: { type: 'insert', text: '(' },
    variant: 'compact',
    bindings: ['('],
  },
  {
    id: 'paren-close',
    label: ')',
    ariaLabel: 'Close bracket',
    action: { type: 'insert', text: ')' },
    variant: 'compact',
    bindings: [')'],
  },

  {
    id: 'clear',
    label: 'AC',
    ariaLabel: 'All clear',
    action: { type: 'clear' },
    variant: 'function',
    bindings: ['Escape', 'c', 'C'],
  },
  {
    id: 'delete',
    label: '⌫',
    ariaLabel: 'Delete last entry',
    action: { type: 'delete' },
    variant: 'function',
    bindings: ['Backspace'],
  },
  {
    id: 'percent',
    label: '%',
    ariaLabel: 'Percent, divides by one hundred',
    action: { type: 'insert', text: '%' },
    variant: 'function',
    bindings: ['%'],
  },
  {
    id: 'divide',
    label: '÷',
    ariaLabel: 'Divide',
    action: { type: 'insert', text: ' ÷ ' },
    variant: 'operator',
    bindings: ['/'],
  },

  digit('7'),
  digit('8'),
  digit('9'),
  {
    id: 'multiply',
    label: '×',
    ariaLabel: 'Multiply',
    action: { type: 'insert', text: ' × ' },
    variant: 'operator',
    bindings: ['*', 'x', 'X'],
  },

  digit('4'),
  digit('5'),
  digit('6'),
  {
    id: 'subtract',
    label: '−',
    ariaLabel: 'Subtract',
    action: { type: 'insert', text: ' − ' },
    variant: 'operator',
    bindings: ['-'],
  },

  digit('1'),
  digit('2'),
  digit('3'),
  {
    id: 'add',
    label: '+',
    ariaLabel: 'Add',
    action: { type: 'insert', text: ' + ' },
    variant: 'operator',
    bindings: ['+'],
  },

  digit('0'),
  {
    id: 'decimal',
    label: '.',
    ariaLabel: 'Decimal point',
    action: { type: 'insert', text: '.' },
    variant: 'digit',
    bindings: ['.'],
  },
  {
    // Twenty-three keys in a four-column grid leaves one cell over. It goes to
    // the most-pressed key rather than inflating a digit.
    id: 'equals',
    label: '=',
    ariaLabel: 'Evaluate',
    action: { type: 'evaluate' },
    variant: 'equals',
    span: 2,
    bindings: ['Enter', '='],
  },
];
