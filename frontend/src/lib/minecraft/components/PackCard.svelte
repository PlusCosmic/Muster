<script lang="ts">
  import Icon from '$lib/shell/components/Icon.svelte';
  import { openExternal, revealPath } from '$lib/shell/api';
  import { relativeTime } from '$lib/shell/format';
  import { toastError } from '$lib/shell/stores/toasts.svelte';
  import { dialogs } from '$lib/shell/stores/dialogs.svelte';
  import { packs } from '$lib/minecraft/stores/packs.svelte';
  import type { ModrinthVersion, Pack } from '$lib/minecraft/types';
  import LaunchSettings from './LaunchSettings.svelte';
  import ModrinthVersionModal from './ModrinthVersionModal.svelte';

  let { pack }: { pack: Pack } = $props();

  // Shown by default before the first install, so the memory choice is
  // made consciously rather than discovered later.
  // svelte-ignore state_referenced_locally
  let launchOpen = $state(!pack.installed);

  const check = $derived(packs.checks[pack.id] ?? null);
  const checking = $derived(!!packs.checking[pack.id]);
  const syncing = $derived(packs.syncingId === pack.id);
  const busyElsewhere = $derived(packs.syncingId !== null && !syncing);
  const report = $derived(packs.reports[pack.id] ?? null);
  const progress = $derived(syncing ? packs.progress : null);

  const modrinth = $derived(pack.source === 'modrinth');
  /** A Modrinth pack held at a version other than the one on disk. */
  const moving = $derived(modrinth && pack.installed && !!check && check.targetVersion !== pack.installedVersion);

  const action = $derived.by(() => {
    if (!pack.installed) return { label: 'Install', icon: 'download' as const };
    if (moving && check) return { label: `Install v${check.targetVersion}`, icon: 'download' as const };
    if (check && !check.upToDate) return { label: modrinth ? 'Repair' : 'Update', icon: 'refresh' as const };
    return { label: 'Re-sync', icon: 'refresh' as const };
  });

  const status = $derived.by(() => {
    if (syncing) return null;
    if (checking) return { kind: 'muted', text: 'Checking for updates…' };
    if (!check) return pack.installed ? { kind: 'muted', text: `Installed v${pack.installedVersion}` } : { kind: 'muted', text: 'Not installed' };
    if (!pack.installed) return { kind: 'info', text: `v${check.targetVersion} · Minecraft ${check.minecraft} · ${check.loader} ${check.loaderVersion}` };
    if (moving) {
      const n = check.toDownload;
      return { kind: 'warn', text: `Held at v${check.targetVersion}; v${pack.installedVersion} is installed (${n} file${n === 1 ? '' : 's'} to change)` };
    }
    if (!check.upToDate) {
      const n = check.toDownload;
      return modrinth
        ? { kind: 'warn', text: `${n} file${n === 1 ? '' : 's'} missing or changed — repair to restore v${pack.installedVersion}` }
        : { kind: 'warn', text: `Update available: v${check.latestVersion} (${n} file${n === 1 ? '' : 's'})` };
    }
    if (!check.loaderInstalled) return { kind: 'warn', text: 'Up to date, but the launcher is missing the mod loader — re-sync to install it' };
    return { kind: 'ok', text: `Up to date · v${pack.installedVersion}` };
  });

  // Modrinth packs never move on their own: a newer release is offered, and
  // the version picker lets the user choose any version, including older.
  let versionsOpen = $state(false);
  let versions = $state<ModrinthVersion[]>([]);
  let loadingVersions = $state(false);
  let changingVersion = $state(false);
  async function openVersions() {
    loadingVersions = true;
    const vs = await packs.listVersions(pack.id);
    loadingVersions = false;
    if (!vs) return;
    versions = vs;
    versionsOpen = true;
  }
  async function holdAt(version: string) {
    changingVersion = true;
    const p = await packs.setVersion(pack.id, version);
    changingVersion = false;
    if (p) versionsOpen = false;
  }

  const percent = $derived(
    progress && progress.phase === 'files' && progress.total > 0 ? Math.round((progress.done / progress.total) * 100) : null
  );

  const phaseText = $derived.by(() => {
    if (!progress) return 'Starting…';
    switch (progress.phase) {
      case 'files':
        return `Downloading ${progress.done} of ${progress.total}: ${progress.current}`;
      case 'loader':
        return progress.current;
      case 'profile':
        return 'Adding to the Minecraft launcher…';
      case 'waiting':
        return progress.current;
    }
    return progress.current;
  });

  const gb = (mb: number) => (mb / 1024).toFixed(1).replace(/\.0$/, '');
  const removable = $derived(pack.source === 'modrinth' || !!pack.code);
  async function remove() {
    if (!removable) return;
    const again = pack.source === 'modrinth' ? 'the link' : 'the code';
    const ok = await dialogs.confirm({
      title: `Remove ${pack.name}?`,
      body: `The pack disappears from this list. Its files and its entry in the Minecraft launcher are left in place; you can add ${again} again any time.`,
      confirmLabel: 'Remove',
      danger: true
    });
    if (ok) await packs.remove(pack);
  }

  const memory = $derived(
    `${gb(pack.launch.maxMemoryMb)} GB RAM${pack.launchCustomised ? '' : pack.recommendedMaxMemoryMb ? ' (recommended)' : ' (default)'}`
  );
</script>

<article class="card" class:syncing>
  <header>
    <div class="title">
      <h2>{pack.name}</h2>
      <div class="meta">
        {#if pack.server}<span class="pill"><Icon name="link" size={11} /> {pack.server}</span>{/if}
        <button class="pill clickable" title="Launch settings" onclick={() => (launchOpen = !launchOpen)}>{memory}</button>
        {#if pack.code}<span class="pill muted mono" title="Pack code">{pack.code}</span>{/if}
        {#if pack.source === 'modrinth'}
          <button class="pill muted clickable" title="Open on Modrinth" onclick={() => openExternal(pack.packUrl)}>
            Modrinth <Icon name="externalLink" size={10} />
          </button>
        {/if}
        {#if pack.installed && pack.syncedAtMs}
          <span class="pill muted" title="Last synced">synced {relativeTime(pack.syncedAtMs, 'a while ago')}</span>
        {/if}
      </div>
    </div>
  </header>

  {#if pack.description}
    <p class="desc">{pack.description}</p>
  {/if}

  {#if syncing}
    <div class="progress" role="progressbar" aria-valuenow={percent ?? undefined} aria-valuemin={0} aria-valuemax={100}>
      <div class="bar" class:indeterminate={percent === null} style:width={percent === null ? undefined : `${percent}%`}></div>
    </div>
    <p class="phase">{phaseText}</p>
  {:else if status}
    <p class="status {status.kind}">
      {#if status.kind === 'ok'}<span class="dot"></span>{:else if status.kind === 'warn'}<Icon name="alert" size={12} />{/if}
      {status.text}
    </p>
  {/if}

  {#if modrinth && !syncing}
    <div class="held">
      <span>Held at <strong class="mono">v{pack.heldVersion}</strong></span>
      {#if check?.updateAvailable}
        <span class="sep">·</span>
        <span class="newer"><Icon name="info" size={12} /> v{check.latestVersion} is out</span>
        <button class="btn btn-sm" disabled={busyElsewhere} onclick={() => packs.moveTo(pack.id, check.latestVersion)}>
          <Icon name="download" size={12} /> Update to v{check.latestVersion}
        </button>
      {/if}
      <span class="spacer"></span>
      <button class="linkish" disabled={loadingVersions || busyElsewhere} onclick={openVersions}>
        {loadingVersions ? 'Loading versions…' : 'Change version…'}
      </button>
    </div>
  {/if}

  {#if launchOpen && !syncing}
    <LaunchSettings {pack} onclose={() => (launchOpen = false)} />
  {/if}

  {#if report && report.manual.length > 0 && !syncing}
    <div class="manual">
      <strong><Icon name="alert" size={12} /> {report.manual.length} file{report.manual.length === 1 ? '' : 's'} need a manual download</strong>
      <p>Their authors don't allow automatic downloads. Get each file and drop it into the pack's <code>mods</code> folder, then re-sync.</p>
      <ul>
        {#each report.manual as m (m.path)}
          <li>
            <button class="linkish" onclick={() => openExternal(m.url)}>{m.name} <Icon name="externalLink" size={11} /></button>
            <span class="mono">{m.path}</span>
          </li>
        {/each}
      </ul>
    </div>
  {/if}

  <footer>
    <button class="btn btn-primary" disabled={syncing || busyElsewhere} onclick={() => packs.sync(pack.id)}>
      <Icon name={syncing ? 'spinner' : action.icon} size={14} class={syncing ? 'spin' : ''} />
      {syncing ? 'Installing…' : action.label}
    </button>
    {#if pack.profileWritten}
      <button class="btn" disabled={syncing} title="Open the Minecraft launcher; this pack is preselected" onclick={() => packs.openLauncher()}>
        <Icon name="play" size={14} /> Open launcher
      </button>
    {/if}
    <span class="spacer"></span>
    {#if pack.installed}
      <button
        class="btn btn-ghost btn-icon"
        title="Open the pack folder"
        onclick={() => revealPath(pack.installDir).catch((e) => toastError('Open folder', e))}
      >
        <Icon name="folder" size={15} />
      </button>
    {/if}
    <button class="btn btn-ghost btn-icon" title="Launch settings" class:active={launchOpen} onclick={() => (launchOpen = !launchOpen)}>
      <Icon name="gear" size={15} />
    </button>
    <button class="btn btn-ghost btn-icon" title="Check for updates" disabled={checking || syncing} onclick={() => packs.check(pack.id)}>
      <Icon name="refresh" size={15} class={checking ? 'spin' : ''} />
    </button>
    {#if removable}
      <button class="btn btn-ghost btn-icon" title="Remove this pack from the list" disabled={syncing} onclick={remove}>
        <Icon name="trash" size={15} />
      </button>
    {/if}
  </footer>
</article>

{#if versionsOpen}
  <ModrinthVersionModal
    title="Change version"
    name={pack.name}
    pageUrl={pack.packUrl}
    {versions}
    suggested={pack.heldVersion ?? ''}
    held={pack.heldVersion}
    installed={pack.installedVersion}
    confirmLabel="Hold at"
    busy={changingVersion}
    onconfirm={holdAt}
    onclose={() => (versionsOpen = false)}
  />
{/if}

<style>
  .held {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
    padding: 8px 12px;
    font-size: 12px;
    color: var(--text-muted);
    background: var(--bg-raised);
    border-radius: var(--r-md);
  }
  .held strong {
    color: var(--text);
    font-weight: 600;
  }
  .held .sep {
    color: var(--text-faint);
  }
  .held .newer {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    color: var(--text);
  }
  .held .spacer {
    flex: 1;
  }
  .held .linkish {
    padding: 0;
    font: inherit;
    font-size: 12px;
    color: var(--accent);
    background: none;
    border: none;
    cursor: pointer;
  }
  .held .linkish:hover:not(:disabled) {
    text-decoration: underline;
  }
  .held .linkish:disabled {
    color: var(--text-faint);
    cursor: default;
  }
  .card {
    display: flex;
    flex-direction: column;
    gap: 10px;
    padding: 16px 18px;
    background: var(--bg-panel);
    border: 1px solid var(--border);
    border-radius: var(--r-lg);
    transition: border-color var(--t-fast);
  }
  .card.syncing {
    border-color: var(--accent);
  }
  h2 {
    margin: 0;
    font-size: 16px;
    font-weight: 700;
    letter-spacing: -0.01em;
  }
  .meta {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    margin-top: 6px;
  }
  .pill {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 2px 8px;
    font-size: 11px;
    color: var(--text-muted);
    background: var(--bg-raised);
    border-radius: 999px;
  }
  .pill.clickable {
    cursor: pointer;
    border: none;
    font: inherit;
    font-size: 11px;
  }
  .pill.clickable:hover {
    color: var(--text);
    background: var(--bg-hover);
  }
  .pill.muted {
    color: var(--text-faint);
    background: transparent;
    border: 1px solid var(--border-subtle);
  }
  .desc {
    margin: 0;
    font-size: 13px;
    line-height: 1.5;
    color: var(--text-muted);
  }
  .status {
    display: flex;
    align-items: center;
    gap: 6px;
    margin: 0;
    font-size: 12px;
    color: var(--text-faint);
  }
  .status.ok {
    color: var(--text-muted);
  }
  .status.warn {
    color: var(--warn);
  }
  .status.info {
    color: var(--text-muted);
  }
  .dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--ok, #6fbf73);
  }
  .progress {
    height: 6px;
    overflow: hidden;
    background: var(--bg-raised);
    border-radius: 3px;
  }
  .bar {
    height: 100%;
    background: var(--accent);
    border-radius: 3px;
    transition: width 120ms linear;
  }
  .bar.indeterminate {
    width: 35%;
    animation: mc-slide 1.1s ease-in-out infinite;
  }
  @keyframes mc-slide {
    0% {
      transform: translateX(-100%);
    }
    100% {
      transform: translateX(300%);
    }
  }
  .phase {
    margin: 0;
    font-size: 12px;
    color: var(--text-muted);
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }
  .manual {
    padding: 10px 12px;
    font-size: 12px;
    background: var(--bg-sunken);
    border: 1px solid var(--border-subtle);
    border-radius: var(--r-md);
  }
  .manual strong {
    display: flex;
    align-items: center;
    gap: 5px;
    color: var(--warn);
  }
  .manual p {
    margin: 4px 0 6px;
    color: var(--text-muted);
    line-height: 1.5;
  }
  .manual ul {
    margin: 0;
    padding: 0;
    list-style: none;
    display: grid;
    gap: 3px;
  }
  .manual li {
    display: flex;
    align-items: baseline;
    gap: 8px;
  }
  .manual .mono {
    color: var(--text-faint);
    font-size: 11px;
  }
  .linkish {
    display: inline-flex;
    align-items: center;
    gap: 3px;
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
  footer {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-top: 4px;
  }
  .spacer {
    flex: 1;
  }
</style>
