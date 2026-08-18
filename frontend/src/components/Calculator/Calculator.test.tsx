import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { Calculator } from './Calculator';
import type { CalculatorApi } from '../../api/client';
import type { CalculateResponse } from '../../api/types';

const stubApi = (formatted = '14'): CalculatorApi & { calculate: ReturnType<typeof vi.fn> } => ({
  calculate: vi.fn(
    async (request): Promise<CalculateResponse> => ({
      result: Number(formatted),
      formatted,
      expression: request.expression,
    }),
  ),
});

describe('Calculator', () => {
  it('composes an expression from key presses and evaluates it', async () => {
    const api = stubApi('14');
    render(<Calculator api={api} />);

    await userEvent.click(screen.getByTestId('digit-2'));
    await userEvent.click(screen.getByTestId('add'));
    await userEvent.click(screen.getByTestId('digit-3'));
    expect(screen.getByTestId('display')).toHaveTextContent('2 + 3');

    await userEvent.click(screen.getByTestId('equals'));

    await waitFor(() =>
      expect(api.calculate).toHaveBeenCalledWith({ expression: '2 + 3' }),
    );
    await waitFor(() => expect(screen.getByTestId('display')).toHaveTextContent('14'));
  });

  // The keypad must not be able to build an expression the grammar rejects.
  it('gates the closing bracket on there being one to close', async () => {
    render(<Calculator api={stubApi()} />);

    expect(screen.getByTestId('paren-close')).toBeDisabled();

    await userEvent.click(screen.getByTestId('paren-open'));
    expect(screen.getByTestId('paren-close')).toBeEnabled();

    await userEvent.click(screen.getByTestId('digit-1'));
    await userEvent.click(screen.getByTestId('paren-close'));
    expect(screen.getByTestId('paren-close')).toBeDisabled();
  });

  it('surfaces a server failure without discarding the expression', async () => {
    const api: CalculatorApi = {
      calculate: vi.fn(async () => {
        const { CalculatorApiError } = await import('../../api/client');
        throw new CalculatorApiError('DIVISION_BY_ZERO', "Can't divide by zero", 422, 1);
      }),
    };
    render(<Calculator api={api} />);

    await userEvent.click(screen.getByTestId('digit-1'));
    await userEvent.click(screen.getByTestId('divide'));
    await userEvent.click(screen.getByTestId('digit-0'));
    await userEvent.click(screen.getByTestId('equals'));

    await waitFor(() =>
      expect(screen.getByRole('alert')).toHaveTextContent("Can't divide by zero"),
    );
    expect(screen.getByTestId('display')).toHaveTextContent('1 ÷ 0');
  });

  it('drives the same actions from the physical keyboard', async () => {
    const api = stubApi('4');
    render(<Calculator api={api} />);

    await userEvent.keyboard('2+2');
    expect(screen.getByTestId('display')).toHaveTextContent('2 + 2');

    await userEvent.keyboard('{Enter}');
    await waitFor(() =>
      expect(api.calculate).toHaveBeenCalledWith({ expression: '2 + 2' }),
    );
  });
});
