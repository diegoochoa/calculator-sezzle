import { Calculator } from './components/Calculator';
import { api } from './api/config';
import styles from './App.module.css';

export default function App() {
  return (
    <main className={styles.page}>
      <header className={styles.header}>
        <p className={styles.eyebrow}>Sezzle</p>
        <h1 className={styles.title}>Calculator</h1>
      </header>

      <Calculator api={api} />

      <footer className={styles.footer}>
        Expressions are evaluated by the calculation API, with full operator
        precedence — <code>2 + 3 × 4</code> is 14. Works with your keyboard:
        digits, <kbd>+</kbd> <kbd>−</kbd> <kbd>*</kbd> <kbd>/</kbd> <kbd>^</kbd>{' '}
        <kbd>q</kbd> <kbd>(</kbd> <kbd>)</kbd>, <kbd>Enter</kbd> to evaluate,{' '}
        <kbd>Esc</kbd> to clear.
      </footer>
    </main>
  );
}
