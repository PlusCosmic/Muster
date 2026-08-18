<script lang="ts">
  import { app, CORE_ID } from '$lib/stores/app.svelte';
  import type { ModInfo, ModSource } from '$lib/types';
  import Icon from './Icon.svelte';
  import ModRow from './ModRow.svelte';
  import WarningsPanel from './WarningsPanel.svelte';

  type Column = 'active' | 'inactive';
  type Edge = 'above' | 'below';

  let query = $state('');
  let sourceFilter = $state<'all' | ModSource>('all');
  let mismatchOnly = $state(false);

  let drag = $state<{ packageId: string; from: Column; fromIndex: number } | null>(null);
  let dropAt = $state<{ column: Column; index: number; edge: Edge } | null>(null);
  let overColumn = $state<Column | null>(null);

  const warnedIds = $derived(
    new Set(app.warnings.map((w) => w.packageId).filter((id): id is string => !!id))
  );

  const filteredInactive = $derived.by(() => {
    const q = query.trim().toLowerCase();
    return app.inactiveMods.filter((m) => {
      if (sourceFilter !== 'all' && m.source !== sourceFilter) return false;
      if (mismatchOnly && !app.isVersionMismatch(m)) return false;
      if (!q) return true;
      return (
        m.name.toLowerCase().includes(q) ||
        m.packageId.includes(q) ||
        m.authors.toLowerCase().includes(q)
      );
    });
  });

  const mismatchCount = $derived(app.activeMods.filter((m) => app.isVersionMismatch(m)).length);

  const SOURCES: Array<{ value: 'all' | ModSource; label: string }> = [
    { value: 'all', label: 'All' },
    { value: 'official', label: 'DLC' },
    { value: 'local', label: 'Local' },
    { value: 'workshop', label: 'Steam' }
  ];

  // --- drag and drop ------------------------------------------------------

  function onRowDragStart(e: DragEvent, mod: ModInfo, from: Column, fromIndex: number) {
    if (from === 'active' && mod.packageId === CORE_ID) {
      e.preventDefault();
      return;
    }
    drag = { packageId: mod.packageId, from, fromIndex };
    if (e.dataTransfer) {
      e.dataTransfer.effectAllowed = 'move';
      e.dataTransfer.setData('text/plain', mod.packageId);
    }
  }

  function edgeFor(e: DragEvent, el: HTMLElement): Edge {
    const rect = el.getBoundingClientRect();
    return e.clientY - rect.top < rect.height / 2 ? 'above' : 'below';
  }

  function onRowDragOver(e: DragEvent, column: Column, index: number) {
    if (!drag) return;
    e.preventDefault();
    e.stopPropagation();
    if (e.dataTransfer) e.dataTransfer.dropEffect = 'move';
    overColumn = column;
    dropAt =
      column === 'active'
        ? { column, index, edge: edgeFor(e, e.currentTarget as HTMLElement) }
        : { column, index, edge: 'above' };
  }

  function onColumnDragOver(e: DragEvent, column: Column) {
    if (!drag) return;
    e.preventDefault();
    if (e.dataTransfer) e.dataTransfer.dropEffect = 'move';
    overColumn = column;
    // Rows stop propagation, so reaching here means the pointer is over empty
    // space in the column — the drop will land at the end of the list.
    dropAt =
      column === 'active' && app.activeIds.length > 0
        ? { column, index: app.activeIds.length - 1, edge: 'below' }
        : null;
  }

  function commitDrop(column: Column, index: number | null, edge: Edge) {
    const d = drag;
    if (!d) return;

    if (column === 'inactive') {
      if (d.from === 'active') app.deactivate(d.packageId);
      return;
    }

    // Dropping into the active column.
    let insert = index === null ? app.activeIds.length : index + (edge === 'below' ? 1 : 0);
    if (d.from === 'inactive') {
      app.activate(d.packageId, insert);
    } else {
      if (insert > d.fromIndex) insert -= 1;
      app.reorder(d.fromIndex, insert);
    }
  }

  function onRowDrop(e: DragEvent, column: Column, index: number) {
    if (!drag) return;
    e.preventDefault();
    e.stopPropagation();
    const edge = column === 'active' ? edgeFor(e, e.currentTarget as HTMLElement) : 'above';
    commitDrop(column, index, edge);
    resetDrag();
  }

  function onColumnDrop(e: DragEvent, column: Column) {
    if (!drag) return;
    e.preventDefault();
    commitDrop(column, null, 'below');
    resetDrag();
  }

  function resetDrag() {
    drag = null;
    dropAt = null;
    overColumn = null;
  }

  function dropEdgeFor(column: Column, index: number): Edge | null {
    if (!dropAt || dropAt.column !== column || dropAt.index !== index) return null;
    return dropAt.edge;
  }
</script>

<div class="editor">
  <div class="columns">
    <!-- ---------------- Inactive ---------------- -->
    <section
      class="column"
      class:drop-target={overColumn === 'inactive' && drag?.from === 'active'}
      ondragover={(e) => onColumnDragOver(e, 'inactive')}
      ondrop={(e) => onColumnDrop(e, 'inactive')}
      role="group"
      aria-label="Inactive mods"
    >
      <header>
        <h3>Inactive</h3>
        <span class="count">{filteredInactive.length}<span class="of">/{app.inactiveMods.length}</span></span>
        <div class="spacer"></div>
        <button
          class="btn btn-sm"
          disabled={filteredInactive.length === 0}
          title="Activate every mod currently shown"
          onclick={() => app.activateMany(filteredInactive.map((m) => m.packageId))}
        >
          <Icon name="moveAll" size={13} /> Add shown
        </button>
      </header>

      <div class="toolbar">
        <div class="search">
          <Icon name="search" size={13} />
          <input
            class="search-input"
            placeholder="Search name, author, packageId…"
            bind:value={query}
            spellcheck="false"
            autocomplete="off"
          />
          {#if query}
            <button class="clear" onclick={() => (query = '')} aria-label="Clear search">
              <Icon name="x" size={12} />
            </button>
          {/if}
        </div>
        <div class="chips">
          {#each SOURCES as s (s.value)}
            <button
              class="chip"
              class:on={sourceFilter === s.value}
              onclick={() => (sourceFilter = s.value)}>{s.label}</button
            >
          {/each}
          <button
            class="chip"
            class:on={mismatchOnly}
            title="Show only mods that don’t list the installed game version"
            onclick={() => (mismatchOnly = !mismatchOnly)}
          >
            <Icon name="alert" size={11} />
          </button>
        </div>
      </div>

      <ul class="list" role="listbox" aria-label="Inactive mods">
        {#if app.loadingMods}
          <li class="placeholder">Scanning installed mods…</li>
        {:else if app.inactiveMods.length === 0}
          <li class="placeholder">Every installed mod is active.</li>
        {:else if filteredInactive.length === 0}
          <li class="placeholder">No mods match this filter.</li>
        {:else}
          {#each filteredInactive as mod, i (mod.packageId)}
            <ModRow
              {mod}
              column="inactive"
              mismatch={app.isVersionMismatch(mod)}
              gameVersion={app.gameMajorMinor}
              dragging={drag?.packageId === mod.packageId}
              onmove={() => app.activate(mod.packageId)}
              ondragstart={(e) => onRowDragStart(e, mod, 'inactive', i)}
              ondragover={(e) => onRowDragOver(e, 'inactive', i)}
              ondrop={(e) => onRowDrop(e, 'inactive', i)}
              ondragend={resetDrag}
            />
          {/each}
        {/if}
      </ul>
    </section>

    <!-- ---------------- Active ---------------- -->
    <section
      class="column"
      class:drop-target={overColumn === 'active' && drag !== null}
      ondragover={(e) => onColumnDragOver(e, 'active')}
      ondrop={(e) => onColumnDrop(e, 'active')}
      role="group"
      aria-label="Active mods"
    >
      <header>
        <h3>Active</h3>
        <span class="count">{app.activeIds.length}</span>
        {#if mismatchCount > 0}
          <span class="warn-pill" title="Active mods that don’t list the installed game version">
            <Icon name="alert" size={11} />
            {mismatchCount}
          </span>
        {/if}
        <div class="spacer"></div>
        <button
          class="btn btn-sm"
          disabled={app.sorting || app.activeIds.length === 0}
          title="Sort the load order using dependencies and the community rules database"
          onclick={() => app.autoSort()}
        >
          <Icon name="sort" size={13} class={app.sorting ? 'spin' : ''} />
          {app.sorting ? 'Sorting…' : 'Auto-sort'}
        </button>
        <button
          class="btn btn-sm"
          disabled={app.activeIds.length <= 1}
          title="Deactivate everything except Core"
          onclick={() => app.deactivateAll()}
        >
          Clear
        </button>
      </header>

      <p class="order-hint">Load order runs top to bottom. Drag rows to reorder.</p>

      <ul class="list" role="listbox" aria-label="Active mods">
        {#if app.loadingActive}
          <li class="placeholder">Reading ModsConfig.xml…</li>
        {:else if app.activeMods.length === 0}
          <li class="placeholder">No active mods. Add some from the left.</li>
        {:else}
          {#each app.activeMods as mod, i (mod.packageId)}
            <ModRow
              {mod}
              column="active"
              position={i + 1}
              locked={mod.packageId === CORE_ID}
              missing={!app.modsById.has(mod.packageId)}
              warned={warnedIds.has(mod.packageId)}
              mismatch={app.isVersionMismatch(mod)}
              gameVersion={app.gameMajorMinor}
              dragging={drag?.packageId === mod.packageId}
              dropEdge={dropEdgeFor('active', i)}
              onmove={() => app.deactivate(mod.packageId)}
              onmoveup={() => app.moveBy(mod.packageId, -1)}
              onmovedown={() => app.moveBy(mod.packageId, 1)}
              ondragstart={(e) => onRowDragStart(e, mod, 'active', i)}
              ondragover={(e) => onRowDragOver(e, 'active', i)}
              ondrop={(e) => onRowDrop(e, 'active', i)}
              ondragend={resetDrag}
            />
          {/each}
        {/if}
      </ul>
    </section>
  </div>

  {#if app.warnings.length > 0}
    <WarningsPanel warnings={app.warnings} onclose={() => (app.warnings = [])} />
  {/if}
</div>

<style>
  .editor {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
    gap: 10px;
    padding: 12px 16px 16px;
  }

  .columns {
    flex: 1;
    min-height: 0;
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 12px;
  }

  .column {
    display: flex;
    flex-direction: column;
    min-height: 0;
    background: var(--bg-panel);
    border: 1px solid var(--border);
    border-radius: var(--r-lg);
    overflow: hidden;
    transition: border-color var(--t-med), background var(--t-med);
  }
  .column.drop-target {
    border-color: var(--accent-line);
    background: rgba(211, 112, 58, 0.05);
  }

  header {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 9px 10px 9px 12px;
    border-bottom: 1px solid var(--border-subtle);
  }

  h3 {
    font-size: 12px;
    font-weight: 700;
    letter-spacing: 0.07em;
    text-transform: uppercase;
    color: var(--text-muted);
  }

  .count {
    font-family: var(--font-mono);
    font-size: 11px;
    color: var(--text-faint);
    padding: 1px 6px;
    background: var(--bg-sunken);
    border-radius: 99px;
  }
  .of {
    opacity: 0.6;
  }

  .warn-pill {
    display: inline-flex;
    align-items: center;
    gap: 3px;
    padding: 1px 6px;
    font-size: 11px;
    font-weight: 600;
    color: var(--warn);
    background: var(--warn-soft);
    border-radius: 99px;
  }

  .spacer {
    flex: 1;
  }

  .toolbar {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 10px;
    border-bottom: 1px solid var(--border-subtle);
    background: var(--bg-sunken);
  }

  .search {
    flex: 1;
    min-width: 0;
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 0 8px;
    color: var(--text-faint);
    background: var(--bg-app);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
    transition: border-color var(--t-fast);
  }
  .search:focus-within {
    border-color: var(--accent);
  }
  .search-input {
    flex: 1;
    min-width: 0;
    padding: 5px 0;
    font: inherit;
    font-size: 12.5px;
    color: var(--text);
    background: none;
    border: none;
    outline: none;
  }
  .search-input::placeholder {
    color: var(--text-faint);
  }
  .clear {
    display: grid;
    place-items: center;
    padding: 2px;
    color: var(--text-faint);
    background: none;
    border: none;
    cursor: pointer;
  }
  .clear:hover {
    color: var(--text);
  }

  .chips {
    display: flex;
    gap: 3px;
    flex: none;
  }
  .chip {
    display: inline-flex;
    align-items: center;
    padding: 3px 7px;
    font: inherit;
    font-size: 11px;
    font-weight: 600;
    color: var(--text-faint);
    background: transparent;
    border: 1px solid var(--border);
    border-radius: var(--r-sm);
    cursor: pointer;
    transition: all var(--t-fast);
  }
  .chip:hover {
    color: var(--text);
    border-color: var(--border-strong);
  }
  .chip.on {
    color: var(--accent);
    background: var(--accent-soft);
    border-color: var(--accent-line);
  }

  .order-hint {
    padding: 6px 12px;
    font-size: 11px;
    color: var(--text-faint);
    background: var(--bg-sunken);
    border-bottom: 1px solid var(--border-subtle);
  }

  .list {
    flex: 1;
    min-height: 0;
    margin: 0;
    padding: 5px;
    list-style: none;
    overflow-y: auto;
  }

  .placeholder {
    padding: 22px 14px;
    text-align: center;
    font-size: 12.5px;
    color: var(--text-faint);
  }

  @media (max-width: 980px) {
    .columns {
      grid-template-columns: 1fr;
    }
  }
</style>
