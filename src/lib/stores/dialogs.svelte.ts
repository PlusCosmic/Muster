// Promise-based confirm/prompt dialogs so call sites read like plain async
// code. `DialogHost` (mounted once in the layout) renders whatever is open.

export interface ConfirmSpec {
  title: string;
  body: string;
  /** Extra emphasised line, e.g. "This moves the folder to your system trash." */
  note?: string;
  confirmLabel?: string;
  cancelLabel?: string;
  danger?: boolean;
}

export interface PromptSpec {
  title: string;
  body?: string;
  note?: string;
  label: string;
  placeholder?: string;
  initial?: string;
  confirmLabel?: string;
}

interface OpenConfirm extends ConfirmSpec {
  resolve: (ok: boolean) => void;
}
interface OpenPrompt extends PromptSpec {
  resolve: (value: string | null) => void;
}

class DialogStore {
  confirmSpec = $state<OpenConfirm | null>(null);
  promptSpec = $state<OpenPrompt | null>(null);

  confirm(spec: ConfirmSpec): Promise<boolean> {
    return new Promise((resolve) => {
      this.confirmSpec = { ...spec, resolve };
    });
  }

  prompt(spec: PromptSpec): Promise<string | null> {
    return new Promise((resolve) => {
      this.promptSpec = { ...spec, resolve };
    });
  }

  settleConfirm(ok: boolean) {
    const s = this.confirmSpec;
    this.confirmSpec = null;
    s?.resolve(ok);
  }

  settlePrompt(value: string | null) {
    const s = this.promptSpec;
    this.promptSpec = null;
    s?.resolve(value);
  }
}

export const dialogs = new DialogStore();
