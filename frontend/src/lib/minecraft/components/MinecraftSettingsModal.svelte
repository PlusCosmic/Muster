<script lang="ts">
  import AboutSection from '$lib/shell/components/AboutSection.svelte';
  import AppearanceFields from '$lib/shell/components/AppearanceFields.svelte';
  import Icon from '$lib/shell/components/Icon.svelte';
  import Modal from '$lib/shell/components/Modal.svelte';
  import { MOCK_ENABLED } from '$lib/shell/mock';
  import { theme } from '$lib/shell/stores/theme.svelte';
  import { titlebar } from '$lib/shell/stores/titlebar.svelte';
  import { packs } from '$lib/minecraft/stores/packs.svelte';
  import type { Settings } from '$lib/minecraft/types';

  let { onclose }: { onclose: () => void } = $props();

  const blank: Settings = { codes: [], manifestUrl: null, registryUrlOverride: null, minecraftDirOverride: null, packs: {} };
  let draft = $state<Settings>({ ...(packs.settings ?? blank) });
  let draftTheme = $state(theme.current);
  let draftTitlebar = $state(titlebar.current);
  let saving = $state(false);

  function previewTheme(id: string) {
    draftTheme = id;
    theme.preview(id);
  }
  function previewTitlebar(shown: boolean) {
    draftTitlebar = shown;
    titlebar.preview(shown);
  }
  function cancel() {
    theme.apply();
    titlebar.apply();
    onclose();
  }
  function setValue(key: keyof Settings, raw: string) {
    const trimmed = raw.trim();
    draft = { ...draft, [key]: trimmed.length > 0 ? trimmed : null };
  }

  const changed = $derived(
    JSON.stringify(draft) !== JSON.stringify(packs.settings ?? blank) ||
      draftTheme !== theme.current ||
      draftTitlebar !== titlebar.current
  );

  async function save() {
    saving = true;
    const ok = await packs.updateSettings(draft);
    saving = false;
    if (ok) {
      theme.set(draftTheme);
      titlebar.set(draftTitlebar);
      onclose();
    }
  }

  const d = $derived(packs.detected);
</script>

<Modal title="Settings" subtitle="Pack list and Minecraft launcher" width={620} onclose={cancel}>
  <section class="group">
    <h3>Appearance</h3>
    <AppearanceFields {draftTheme} {draftTitlebar} ontheme={previewTheme} ontitlebar={previewTitlebar} />
  </section>

  <section class="group">
    <h3>Pack sources</h3>
    <p class="group-note">
      Packs normally arrive as codes: add one from the main screen. A pack list URL is the older
      way in and still works alongside them.
    </p>
    <div class="field">
      <label class="label" for="mc-registry">Pack registry</label>
      <div class="input-row">
        <input
          id="mc-registry"
          class="input mono"
          spellcheck="false"
          autocomplete="off"
          placeholder={d?.registryUrl ?? 'https://api.musterlauncher.com'}
          value={draft.registryUrlOverride ?? ''}
          oninput={(e) => setValue('registryUrlOverride', e.currentTarget.value)}
        />
        {#if draft.registryUrlOverride}
          <button class="btn btn-sm" title="Clear" onclick={() => setValue('registryUrlOverride', '')}>
            <Icon name="x" size={13} />
          </button>
        {/if}
      </div>
      <p class="hint">Where pack codes are looked up. Only change this if you run your own registry.</p>
    </div>
    <div class="field">
      <label class="label" for="mc-manifest">Pack list URL</label>
      <div class="input-row">
        <input
          id="mc-manifest"
          class="input mono"
          spellcheck="false"
          autocomplete="off"
          placeholder="https://…/manifest.json"
          value={draft.manifestUrl ?? ''}
          oninput={(e) => setValue('manifestUrl', e.currentTarget.value)}
        />
        {#if draft.manifestUrl}
          <button class="btn btn-sm" title="Clear" onclick={() => setValue('manifestUrl', '')}>
            <Icon name="x" size={13} />
          </button>
        {/if}
      </div>
      <p class="hint">
        {#if !draft.manifestUrl}
          <span class="undetected"><Icon name="alert" size={11} /> No pack list yet.</span>
        {/if}
      </p>
    </div>
  </section>

  <section class="group">
    <h3>Minecraft launcher</h3>
    <div class="field">
      <label class="label" for="mc-dir">Minecraft folder</label>
      <div class="input-row">
        <input
          id="mc-dir"
          class="input mono"
          spellcheck="false"
          autocomplete="off"
          placeholder={d?.minecraftDir ?? 'Not detected — enter a path'}
          value={draft.minecraftDirOverride ?? ''}
          oninput={(e) => setValue('minecraftDirOverride', e.currentTarget.value)}
        />
        {#if draft.minecraftDirOverride}
          <button class="btn btn-sm" title="Clear override" onclick={() => setValue('minecraftDirOverride', '')}>
            <Icon name="x" size={13} />
          </button>
        {/if}
      </div>
      <p class="hint">
        The launcher's <span class="mono">.minecraft</span> folder, where packs are registered as profiles.
        {#if d?.minecraftDir}
          <br />
          {#if d.launcherInstalled}
            <span class="detected"><span class="ok-dot"></span>Launcher found: <span class="mono" data-selectable>{d.minecraftDir}</span></span>
          {:else}
            <span class="undetected"><Icon name="alert" size={11} /> Nothing at <span class="mono">{d.minecraftDir}</span> yet — install the launcher and run it once.</span>
          {/if}
        {/if}
      </p>
    </div>
    <dl class="facts">
      <dt>Packs folder</dt>
      <dd class="mono" data-selectable>{d?.packsDir ?? '—'}</dd>
    </dl>
  </section>

  <section class="group">
    <h3>About Muster</h3>
    <AboutSection />
  </section>

  {#if MOCK_ENABLED}
    <p class="mock">Mock backend active (dev only — drop <code>?mock=1</code> to use real commands).</p>
  {/if}

  {#snippet footer()}
    <button class="btn" onclick={cancel}>Cancel</button>
    <button class="btn btn-primary" disabled={!changed || saving} onclick={save}>
      {saving ? 'Saving…' : 'Save settings'}
    </button>
  {/snippet}
</Modal>

<style>
  .group + .group {
    margin-top: 22px;
    padding-top: 18px;
    border-top: 1px solid var(--border-subtle);
  }
  h3 {
    font-size: 11px;
    font-weight: 700;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--text-muted);
  }
  .group-note {
    margin-top: 5px;
    font-size: 12px;
    color: var(--text-faint);
    line-height: 1.5;
  }
  .field {
    margin-top: 14px;
  }
  .input-row {
    display: flex;
    gap: 6px;
    margin-top: 6px;
  }
  .input-row .input {
    flex: 1;
    min-width: 0;
  }
  .detected {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    color: var(--text-muted);
  }
  .undetected {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    color: var(--warn);
  }
  .ok-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--ok, #6fbf73);
  }
  .facts {
    display: grid;
    grid-template-columns: max-content 1fr;
    gap: 6px 16px;
    margin: 14px 0 0;
    font-size: 12px;
  }
  .facts dt {
    color: var(--text-faint);
  }
  .facts dd {
    margin: 0;
    word-break: break-all;
  }
  .mock {
    margin-top: 18px;
    font-size: 11px;
    color: var(--text-faint);
  }
</style>
