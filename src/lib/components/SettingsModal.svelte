<script lang="ts">
  import { app } from '$lib/stores/app.svelte';
  import { theme, THEMES } from '$lib/stores/theme.svelte';
  import { absoluteTime, relativeTime } from '$lib/format';
  import { MOCK_ENABLED } from '$lib/backend';
  import type { Settings } from '$lib/types';
  import Icon from './Icon.svelte';
  import Modal from './Modal.svelte';

  interface Props {
    onclose: () => void;
  }
  let { onclose }: Props = $props();

  const blank: Settings = {
    steamRootOverride: null,
    gameInstallOverride: null,
    defaultSavedataOverride: null
  };

  let draft = $state<Settings>({ ...(app.settings ?? blank) });
  let draftTheme = $state(theme.current);
  let saving = $state(false);
  let refreshing = $state(false);

  function previewTheme(id: string) {
    draftTheme = id;
    theme.preview(id);
  }

  /** Close without saving: undo any live theme preview. */
  function cancel() {
    theme.apply();
    onclose();
  }

  const fields = [
    {
      key: 'steamRootOverride',
      label: 'Steam root',
      detectedKey: 'steamRoot',
      hint: 'Where Steam is installed. Used to find library folders, workshop mods and the launcher.'
    },
    {
      key: 'gameInstallOverride',
      label: 'RimWorld install folder',
      detectedKey: 'gameInstall',
      hint: 'The folder containing the RimWorld executable, Data/ and Mods/.'
    },
    {
      key: 'defaultSavedataOverride',
      label: 'Default savedata folder',
      detectedKey: 'defaultSavedata',
      hint: 'RimWorld’s normal config/saves location — the source for “Import current setup”.'
    }
  ] as const;

  function value(key: keyof Settings): string {
    return draft[key] ?? '';
  }

  function setValue(key: keyof Settings, raw: string) {
    const trimmed = raw.trim();
    draft = { ...draft, [key]: trimmed.length > 0 ? trimmed : null };
  }

  function detectedValue(key: (typeof fields)[number]['detectedKey']): string | null {
    return app.detected?.[key] ?? null;
  }

  const changed = $derived(
    JSON.stringify(draft) !== JSON.stringify(app.settings ?? blank) ||
      draftTheme !== theme.current
  );

  async function save() {
    saving = true;
    const ok = await app.updateSettings(draft);
    saving = false;
    if (ok) {
      theme.set(draftTheme);
      onclose();
    }
  }

  async function refreshRules() {
    refreshing = true;
    await app.refreshRulesDb();
    refreshing = false;
  }

  const rules = $derived(app.rulesStatus);
</script>

<Modal title="Settings" subtitle="Path overrides and mod-sorting data" width={620} onclose={cancel}>
  <section class="group">
    <h3>Appearance</h3>
    <div class="field theme-row">
      <div>
        <label class="label" for="rf-theme">Theme</label>
        <p class="hint">
          {THEMES.find((t) => t.id === draftTheme)?.description}
          {#if draftTheme !== theme.current}— previewing; Save to keep it{/if}
        </p>
      </div>
      <select
        id="rf-theme"
        class="input select"
        value={draftTheme}
        onchange={(e) => previewTheme(e.currentTarget.value)}
      >
        {#each THEMES as t (t.id)}
          <option value={t.id}>{t.name}</option>
        {/each}
      </select>
    </div>
  </section>

  <section class="group">
    <h3>Paths</h3>
    <p class="group-note">
      Leave a field empty to use the detected location. Overrides are stored in RimForge’s
      settings.json and applied on top of detection.
    </p>

    {#each fields as f (f.key)}
      {@const detected = detectedValue(f.detectedKey)}
      <div class="field">
        <label class="label" for="rf-{f.key}">{f.label}</label>
        <div class="input-row">
          <input
            id="rf-{f.key}"
            class="input mono"
            spellcheck="false"
            autocomplete="off"
            placeholder={detected ?? 'Not detected — enter a path'}
            value={value(f.key)}
            oninput={(e) => setValue(f.key, e.currentTarget.value)}
          />
          {#if draft[f.key]}
            <button class="btn btn-sm" title="Clear override" onclick={() => setValue(f.key, '')}>
              <Icon name="x" size={13} />
            </button>
          {/if}
        </div>
        <p class="hint">
          {f.hint}
          {#if detected}
            <br /><span class="detected"
              ><span class="ok-dot"></span>Detected: <span class="mono" data-selectable>{detected}</span></span
            >
          {:else}
            <br /><span class="undetected"><Icon name="alert" size={11} /> Nothing detected here.</span>
          {/if}
        </p>
      </div>
    {/each}
  </section>

  <section class="group">
    <h3>Detected environment</h3>
    <dl class="facts">
      <dt>Game version</dt>
      <dd class="mono" data-selectable>{app.detected?.gameVersion ?? '—'}</dd>
      <dt>Profiles folder</dt>
      <dd class="mono" data-selectable>{app.detected?.profilesDir ?? '—'}</dd>
      <dt>Workshop folders</dt>
      <dd>
        {#if app.detected?.workshopDirs.length}
          {#each app.detected.workshopDirs as dir (dir)}
            <div class="mono" data-selectable>{dir}</div>
          {/each}
        {:else}
          <span class="muted">None found</span>
        {/if}
      </dd>
      <dt>Installed mods</dt>
      <dd>{app.installedMods.length}</dd>
    </dl>
  </section>

  <section class="group">
    <h3>Community rules database</h3>
    <p class="group-note">
      RimSort’s community load-order rules make auto-sort much better. RimForge caches it locally
      and only re-fetches when you ask.
    </p>
    <div class="rules">
      <div class="rules-state">
        {#if rules?.cached}
          <span class="pill ok"><span class="ok-dot"></span> Cached</span>
          <span class="rules-meta">
            {rules.ruleCount.toLocaleString()} rules · fetched {relativeTime(
              rules.fetchedAtMs,
              'at an unknown time'
            )}
          </span>
          <span class="rules-abs">{absoluteTime(rules.fetchedAtMs)}</span>
        {:else}
          <span class="pill warn"><Icon name="alert" size={11} /> Not cached</span>
          <span class="rules-meta">
            Auto-sort will fall back to About.xml metadata only until this is fetched.
          </span>
        {/if}
      </div>
      <button class="btn" disabled={refreshing} onclick={refreshRules}>
        <Icon name="refresh" size={14} class={refreshing ? 'spin' : ''} />
        {refreshing ? 'Fetching…' : 'Refresh now'}
      </button>
    </div>
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

  .theme-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    margin-top: 10px;
  }
  .theme-row .hint {
    margin-top: 2px;
  }
  .select {
    width: 180px;
    flex: none;
    cursor: pointer;
    /* native select needs an explicit bg/appearance to match .input */
    -webkit-appearance: none;
    appearance: none;
    padding-right: 28px;
    background-image: linear-gradient(45deg, transparent 50%, var(--text-muted) 50%),
      linear-gradient(135deg, var(--text-muted) 50%, transparent 50%);
    background-position: calc(100% - 16px) 55%, calc(100% - 11px) 55%;
    background-size: 5px 5px;
    background-repeat: no-repeat;
  }

  .input-row {
    display: flex;
    gap: 6px;
    align-items: center;
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
    background: var(--ok);
    flex: none;
  }

  .facts {
    display: grid;
    grid-template-columns: 150px 1fr;
    gap: 6px 14px;
    margin: 12px 0 0;
    font-size: 12px;
  }
  .facts dt {
    color: var(--text-faint);
  }
  .facts dd {
    margin: 0;
    min-width: 0;
    word-break: break-all;
  }
  .muted {
    color: var(--text-faint);
  }

  .rules {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-top: 12px;
    padding: 11px 12px;
    background: var(--bg-sunken);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
  }
  .rules-state {
    flex: 1;
    min-width: 0;
    display: grid;
    gap: 3px;
  }
  .pill {
    justify-self: start;
    display: inline-flex;
    align-items: center;
    gap: 5px;
    padding: 1px 8px;
    font-size: 11px;
    font-weight: 600;
    border-radius: 99px;
  }
  .pill.ok {
    color: var(--ok);
    background: var(--ok-soft);
  }
  .pill.warn {
    color: var(--warn);
    background: var(--warn-soft);
  }
  .rules-meta {
    font-size: 12px;
    color: var(--text-muted);
  }
  .rules-abs {
    font-size: 11px;
    color: var(--text-faint);
  }

  .mock {
    margin-top: 18px;
    padding: 8px 10px;
    font-size: 11.5px;
    color: var(--info);
    background: var(--info-soft);
    border-radius: var(--r-sm);
  }
</style>
