<script lang="ts">
  import { onMount } from 'svelte';
  import Icon from '$lib/shell/components/Icon.svelte';
  import { openExternal } from '$lib/shell/api';
  import MinecraftSettingsModal from '$lib/minecraft/components/MinecraftSettingsModal.svelte';
  import PackCard from '$lib/minecraft/components/PackCard.svelte';
  import { packs } from '$lib/minecraft/stores/packs.svelte';
  import { dialogs } from '$lib/shell/stores/dialogs.svelte';

  let settingsOpen = $state(false);
  let codeDraft = $state('');
  let addingCode = $state(false);

  async function addCodeFromCard() {
    const code = codeDraft.trim();
    if (!code) return;
    addingCode = true;
    const p = await packs.addCode(code);
    addingCode = false;
    if (p) codeDraft = '';
  }

  async function addCodeFlow() {
    const code = await dialogs.prompt({
      title: 'Add a pack',
      body: 'Enter the pack code you were given. A pasted link with the code in it works too.',
      label: 'Pack code',
      placeholder: 'amber-otter-42',
      confirmLabel: 'Add pack'
    });
    if (code) await packs.addCode(code);
  }

  onMount(() => {
    packs.init().then(() => {
      // One upstream check per pack so cards show update state without a click.
      for (const p of packs.packs) void packs.check(p.id);
    });
  });

  function onkeydown(e: KeyboardEvent) {
    if ((e.ctrlKey || e.metaKey) && e.key === ',') {
      e.preventDefault();
      settingsOpen = true;
    }
  }

  async function refresh() {
    await packs.loadPacks();
    for (const p of packs.packs) void packs.check(p.id);
  }

  const launcherMissing = $derived(packs.detected !== null && !packs.detected.launcherInstalled);
</script>

<svelte:head>
  <title>Muster · Minecraft</title>
</svelte:head>

<svelte:window {onkeydown} />

<div class="page">
  <header class="bar">
    <div class="heading">
      <h1>Minecraft</h1>
      <span class="sub">Shared packs</span>
    </div>
    <span class="spacer"></span>
    <button class="btn btn-primary" onclick={addCodeFlow} title="Add a pack by its code">
      <Icon name="plus" size={14} /> Add pack
    </button>
    <button class="btn" disabled={packs.loadingPacks || packs.syncingId !== null} onclick={refresh} title="Reload your packs and check for updates">
      <Icon name="refresh" size={14} class={packs.loadingPacks ? 'spin' : ''} /> Refresh
    </button>
    <button class="btn" onclick={() => packs.openLauncher()} title="Open the Minecraft launcher">
      <Icon name="play" size={14} /> Open launcher
    </button>
    <button class="btn btn-ghost btn-icon" title="Settings (Ctrl+,)" onclick={() => (settingsOpen = true)}>
      <Icon name="gear" size={16} />
    </button>
  </header>

  <main>
    {#if packs.booting}
      <div class="center">
        <span class="spinner" aria-hidden="true"></span>
        <span>Loading packs…</span>
      </div>
    {:else if packs.needsManifest}
      <section class="setup">
        <span class="mark" aria-hidden="true"><Icon name="minecraft" size={26} strokeWidth={1.5} /></span>
        <h2>Add your first pack</h2>
        <p>
          Enter the pack code you were given by whoever runs it. Pasting a link that contains the code
          works too.
        </p>
        <form class="row" onsubmit={(e) => { e.preventDefault(); addCodeFromCard(); }}>
          <input class="input mono" placeholder="amber-otter-42" bind:value={codeDraft} spellcheck="false" autocomplete="off" />
          <button class="btn btn-primary" type="submit" disabled={!codeDraft.trim() || addingCode}>
            {addingCode ? 'Looking up…' : 'Add pack'}
          </button>
        </form>
        <p class="alt">Have a pack list URL instead? <button class="linkish" onclick={() => (settingsOpen = true)}>Set it in Settings</button>.</p>
      </section>
    {:else if packs.listError}
      <section class="setup">
        <span class="mark warn" aria-hidden="true"><Icon name="alert" size={24} /></span>
        <h2>Couldn't load the pack list</h2>
        <p class="mono err">{packs.listError}</p>
        <div class="row">
          <button class="btn btn-primary" onclick={refresh}><Icon name="refresh" size={14} /> Try again</button>
          <button class="btn" onclick={() => (settingsOpen = true)}><Icon name="gear" size={14} /> Settings</button>
        </div>
      </section>
    {:else}
      {#if launcherMissing}
        <div class="notice">
          <Icon name="alert" size={14} />
          <div>
            <strong>The Minecraft launcher hasn't run on this machine yet.</strong>
            <span>Install it, sign in and press Play once so it sets itself up; packs are added to it as profiles.</span>
          </div>
          <button class="btn btn-sm" onclick={() => openExternal('https://www.minecraft.net/download')}>
            <Icon name="externalLink" size={13} /> Get the launcher
          </button>
        </div>
      {/if}
      {#if packs.packs.length === 0}
        <div class="center"><Icon name="folder" size={24} /><span>No packs yet. Use Add pack to enter a code.</span></div>
      {:else}
        <div class="grid">
          {#each packs.packs as pack (pack.id)}
            <PackCard {pack} />
          {/each}
        </div>
      {/if}
    {/if}
  </main>
</div>

{#if settingsOpen}
  <MinecraftSettingsModal onclose={() => (settingsOpen = false)} />
{/if}

<style>
  .page {
    display: grid;
    grid-template-rows: var(--header-h) minmax(0, 1fr);
    height: 100%;
    min-height: 0;
    background: var(--bg-app);
  }
  .bar {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 0 16px;
    border-bottom: 1px solid var(--border-subtle);
  }
  .heading {
    display: flex;
    align-items: baseline;
    gap: 10px;
  }
  h1 {
    margin: 0;
    font-size: 15px;
    font-weight: 700;
  }
  .sub {
    font-size: 11px;
    color: var(--text-faint);
  }
  .spacer {
    flex: 1;
  }
  main {
    min-height: 0;
    overflow-y: auto;
    padding: 20px;
  }
  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(360px, 1fr));
    gap: 14px;
    max-width: 1100px;
  }
  .center {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 14px;
    height: 100%;
    color: var(--text-faint);
    font-size: 13px;
  }
  .spinner {
    width: 22px;
    height: 22px;
    border: 2px solid var(--border-strong);
    border-top-color: var(--accent);
    border-radius: 50%;
    animation: mc-spin 800ms linear infinite;
  }
  @keyframes mc-spin {
    to {
      transform: rotate(360deg);
    }
  }
  .setup {
    max-width: 480px;
    margin: 8vh auto 0;
    text-align: center;
  }
  .mark {
    display: grid;
    place-items: center;
    width: 48px;
    height: 48px;
    margin: 0 auto 16px;
    color: var(--mark-fg);
    background: linear-gradient(135deg, var(--mark-grad-1), var(--mark-grad-2));
    border-radius: var(--r-lg);
  }
  .mark.warn {
    color: var(--warn);
    background: var(--bg-raised);
  }
  .setup h2 {
    margin: 0 0 8px;
    font-size: 20px;
  }
  .setup p {
    margin: 0 0 16px;
    color: var(--text-muted);
    font-size: 13px;
    line-height: 1.5;
  }
  .err {
    color: var(--warn);
    font-size: 12px;
    word-break: break-word;
  }
  .row {
    display: flex;
    justify-content: center;
    gap: 8px;
  }
  .row .input {
    flex: 1;
    min-width: 0;
  }
  .alt {
    margin-top: 14px;
    font-size: 12px;
    color: var(--text-faint);
  }
  .linkish {
    padding: 0;
    font: inherit;
    color: var(--accent);
    background: none;
    border: none;
    cursor: pointer;
  }
  .linkish:hover {
    text-decoration: underline;
  }
  .notice {
    display: flex;
    align-items: center;
    gap: 12px;
    max-width: 1100px;
    margin-bottom: 14px;
    padding: 10px 14px;
    font-size: 12px;
    color: var(--text-muted);
    background: var(--bg-panel);
    border: 1px solid var(--border);
    border-left: 3px solid var(--warn);
    border-radius: var(--r-md);
  }
  .notice > :global(svg) {
    color: var(--warn);
    flex: none;
  }
  .notice div {
    flex: 1;
    display: grid;
    gap: 2px;
  }
  .notice strong {
    color: var(--text);
  }
</style>
