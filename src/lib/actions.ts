// User-facing flows that pair a dialog with a store call. Kept out of the
// components so the sidebar, header and empty states all behave identically.

import { app } from './stores/app.svelte';
import { dialogs } from './stores/dialogs.svelte';

function suggestCopyName(name: string): string {
  return `${name} copy`;
}

export async function newProfileFlow(): Promise<void> {
  const name = await dialogs.prompt({
    title: 'New profile',
    body: 'Creates an empty savedata folder with its own config, mod list and saves. Nothing is copied from your existing RimWorld setup.',
    label: 'Profile name',
    placeholder: 'e.g. Medieval Run',
    confirmLabel: 'Create profile'
  });
  if (name) await app.createProfile(name);
}

export async function importDefaultFlow(): Promise<void> {
  const detected = app.detected?.defaultSavedata;
  const name = await dialogs.prompt({
    title: 'Import current setup',
    body:
      'Copies the config, mod list, saves and scenarios from your existing RimWorld installation into a new profile. ' +
      'The originals are read-only here — your vanilla setup is left exactly as it is, so you can always go back to launching RimWorld normally.',
    label: 'Name for the imported profile',
    placeholder: 'e.g. My Current Setup',
    initial: 'My Current Setup',
    note: detected ? `Reading from: ${detected}` : 'RimWorld’s savedata folder was not detected — set it in Settings if the import fails.',
    confirmLabel: 'Import'
  });
  if (name) await app.importDefault(name);
}

export async function cloneProfileFlow(id: string): Promise<void> {
  const profile = app.profiles.find((p) => p.id === id);
  if (!profile) return;
  const name = await dialogs.prompt({
    title: `Clone “${profile.name}”`,
    body: 'Makes a full copy of this profile, including its saves, mod list and mod settings.',
    label: 'Name for the copy',
    initial: suggestCopyName(profile.name),
    confirmLabel: 'Clone'
  });
  if (name) await app.cloneProfile(id, name);
}

export async function renameProfileFlow(id: string): Promise<void> {
  const profile = app.profiles.find((p) => p.id === id);
  if (!profile) return;
  const name = await dialogs.prompt({
    title: 'Rename profile',
    label: 'Profile name',
    initial: profile.name,
    note: 'Only the display name changes — the folder on disk keeps its current path.',
    confirmLabel: 'Rename'
  });
  if (name && name !== profile.name) await app.renameProfile(id, name);
}

export async function deleteProfileFlow(id: string): Promise<void> {
  const profile = app.profiles.find((p) => p.id === id);
  if (!profile) return;
  const ok = await dialogs.confirm({
    title: `Delete “${profile.name}”?`,
    body: `This profile’s folder — including its ${profile.saveCount} save${
      profile.saveCount === 1 ? '' : 's'
    } and its mod settings — will be removed from RimForge.`,
    note: 'The folder is moved to your system trash, so you can restore it from there. Installed mods are shared and are never touched.',
    confirmLabel: 'Move to trash',
    danger: true
  });
  if (ok) await app.deleteProfile(id);
}

export async function launchProfileFlow(id: string): Promise<void> {
  if (app.dirty && app.selectedId === id) {
    const ok = await dialogs.confirm({
      title: 'Launch with unsaved changes?',
      body: 'Your edits to this profile’s mod list have not been written to ModsConfig.xml yet.',
      note: 'RimWorld will start with the last saved load order.',
      confirmLabel: 'Save and launch',
      cancelLabel: 'Cancel'
    });
    if (!ok) return;
    const saved = await app.save();
    if (!saved) return;
  }
  await app.launch(id);
}

export async function createShortcutFlow(id: string): Promise<void> {
  await app.createShortcut(id);
}

/** Switch profiles, confirming first if the mod-list draft is dirty. */
export async function selectProfileGuarded(id: string): Promise<void> {
  if (id === app.selectedId) return;
  if (app.dirty) {
    const from = app.selected?.name ?? 'this profile';
    const ok = await dialogs.confirm({
      title: 'Discard unsaved changes?',
      body: `You have unsaved mod-list changes in “${from}”.`,
      note: 'Switching profiles will discard them.',
      confirmLabel: 'Discard and switch',
      danger: true
    });
    if (!ok) return;
  }
  await app.selectProfile(id);
}

export async function reloadCurrentFlow(): Promise<void> {
  if (app.dirty) {
    const ok = await dialogs.confirm({
      title: 'Reload from disk?',
      body: 'This re-reads the installed mods and this profile’s saved load order.',
      note: 'Your unsaved mod-list changes will be discarded.',
      confirmLabel: 'Discard and reload',
      danger: true
    });
    if (!ok) return;
  }
  await app.reloadCurrent();
}
