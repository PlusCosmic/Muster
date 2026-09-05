<script lang="ts">
  import { importDefaultFlow, newProfileFlow } from '$lib/rimworld/actions';
  import { app } from '$lib/rimworld/stores/app.svelte';
  import Icon from '$lib/shell/components/Icon.svelte';

  interface Props {
    onopensettings: () => void;
  }
  let { onopensettings }: Props = $props();

  const gameFound = $derived(!!app.detected?.gameInstall);
  const savedataFound = $derived(!!app.detected?.defaultSavedata);
</script>

<section class="empty">
  <div class="card">
    <span class="mark" aria-hidden="true">RW</span>
    <h1>RimWorld profiles</h1>
    <p class="lede">
      A profile is a self-contained RimWorld savedata folder — its own mod list, mod settings,
      saves and scenarios. Installed mods stay shared, so switching profiles never re-downloads
      anything.
    </p>

    <div class="choices">
      <button class="choice primary" onclick={importDefaultFlow} disabled={!savedataFound}>
        <span class="choice-icon"><Icon name="import" size={18} /></span>
        <span class="choice-text">
          <strong>Import current setup</strong>
          <span>
            Copy the config, mod list and saves from your existing RimWorld installation into a
            first profile. Your originals are left untouched.
          </span>
        </span>
      </button>

      <button class="choice" onclick={newProfileFlow}>
        <span class="choice-icon"><Icon name="plus" size={18} /></span>
        <span class="choice-text">
          <strong>Start a new profile</strong>
          <span>Begin from an empty savedata folder with only Core active.</span>
        </span>
      </button>
    </div>

    {#if !gameFound || !savedataFound}
      <div class="notice">
        <Icon name="alert" size={14} />
        <div>
          <strong>
            {#if !gameFound}RimWorld’s install folder wasn’t found.{:else}RimWorld’s savedata folder
              wasn’t found.{/if}
          </strong>
          <span>Set the paths manually so Muster can find your mods and saves.</span>
        </div>
        <button class="btn btn-sm" onclick={onopensettings}>
          <Icon name="gear" size={13} /> Open settings
        </button>
      </div>
    {/if}
  </div>
</section>

<style>
  .empty {
    grid-area: main;
    display: grid;
    place-items: center;
    padding: 32px;
    overflow-y: auto;
    background:
      radial-gradient(70% 50% at 50% 0%, rgba(211, 112, 58, 0.07), transparent 70%),
      var(--bg-app);
  }

  .card {
    width: 100%;
    max-width: 560px;
    text-align: center;
  }

  .mark {
    display: grid;
    place-items: center;
    width: 48px;
    height: 48px;
    margin: 0 auto 16px;
    font-size: 17px;
    font-weight: 800;
    color: var(--text-on-accent);
    background: linear-gradient(150deg, #e08a4c, var(--accent-active));
    border-radius: 12px;
    box-shadow: 0 8px 26px rgba(211, 112, 58, 0.28);
  }

  h1 {
    font-size: 22px;
    letter-spacing: -0.02em;
  }

  .lede {
    margin: 10px auto 26px;
    max-width: 460px;
    font-size: 13px;
    line-height: 1.6;
    color: var(--text-muted);
  }

  .choices {
    display: grid;
    gap: 10px;
    text-align: left;
  }

  .choice {
    display: flex;
    align-items: flex-start;
    gap: 13px;
    width: 100%;
    padding: 14px 16px;
    font: inherit;
    color: var(--text);
    background: var(--bg-panel);
    border: 1px solid var(--border);
    border-radius: var(--r-lg);
    cursor: pointer;
    transition: border-color var(--t-med), background var(--t-med), transform var(--t-fast);
  }
  .choice:hover:not(:disabled) {
    background: var(--bg-raised);
    border-color: var(--border-strong);
  }
  .choice:active:not(:disabled) {
    transform: translateY(1px);
  }
  .choice:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
  .choice.primary {
    border-color: var(--accent-line);
    background: rgba(211, 112, 58, 0.07);
  }
  .choice.primary:hover:not(:disabled) {
    background: rgba(211, 112, 58, 0.12);
    border-color: var(--accent);
  }

  .choice-icon {
    display: grid;
    place-items: center;
    width: 34px;
    height: 34px;
    flex: none;
    color: var(--accent);
    background: var(--bg-sunken);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
  }

  .choice-text {
    display: grid;
    gap: 3px;
    min-width: 0;
  }
  .choice-text strong {
    font-size: 13.5px;
    font-weight: 650;
  }
  .choice-text span {
    font-size: 12px;
    line-height: 1.5;
    color: var(--text-muted);
  }

  .notice {
    display: flex;
    align-items: center;
    gap: 11px;
    margin-top: 20px;
    padding: 11px 12px;
    text-align: left;
    color: var(--warn);
    background: var(--warn-soft);
    border: 1px solid rgba(224, 163, 60, 0.3);
    border-radius: var(--r-md);
  }
  .notice div {
    flex: 1;
    display: grid;
    gap: 1px;
  }
  .notice strong {
    font-size: 12.5px;
  }
  .notice span {
    font-size: 11.5px;
    color: var(--text-muted);
  }
</style>
