// Focus plumbing for the layers that sit over the page: the modal, and the
// profile sidebar when a narrow window turns it into a drawer. Both cover
// content that stays in the tab order underneath them, so both have to keep
// Tab to themselves while they are open.

const FOCUSABLE =
  'a[href], button:not(:disabled), input:not(:disabled), select:not(:disabled), textarea:not(:disabled), [tabindex]:not([tabindex="-1"])';

/** Move focus into `container`, honouring an explicit `data-autofocus` pick. */
export function focusFirst(container: HTMLElement | null): void {
  const target =
    container?.querySelector<HTMLElement>('[data-autofocus]') ??
    container?.querySelector<HTMLElement>(FOCUSABLE);
  target?.focus();
}

/** Wrap Tab around `container`'s focusables. Call from a keydown handler. */
export function trapTab(e: KeyboardEvent, container: HTMLElement | null): void {
  if (e.key !== 'Tab' || !container) return;

  const focusables = [...container.querySelectorAll<HTMLElement>(FOCUSABLE)];
  if (focusables.length === 0) return;

  const first = focusables[0];
  const last = focusables[focusables.length - 1];
  const active = document.activeElement as HTMLElement | null;

  // Leaving the top, or arriving from outside with Shift held, wraps to the
  // bottom; leaving the bottom wraps to the top.
  if (e.shiftKey && (active === first || !container.contains(active))) {
    e.preventDefault();
    last.focus();
  } else if (!e.shiftKey && active === last) {
    e.preventDefault();
    first.focus();
  }
}
