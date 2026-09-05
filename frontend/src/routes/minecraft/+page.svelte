<script lang="ts">
  import Icon from '$lib/shell/components/Icon.svelte';
</script>

<svelte:head>
  <title>Muster · Minecraft</title>
</svelte:head>

<section class="soon">
  <div class="card">
    <span class="mark" aria-hidden="true"><Icon name="minecraft" size={26} strokeWidth={1.5} /></span>
    <h1>Minecraft packs</h1>
    <p>
      Pick a shared modpack, and Muster will set it up as a profile in the Minecraft launcher and
      keep it up to date. This module is not built yet.
    </p>
  </div>
</section>

<style>
  .soon {
    display: grid;
    place-items: center;
    padding: 32px;
    background: var(--bg-app);
  }
  .card {
    max-width: 440px;
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
  h1 {
    margin: 0 0 8px;
    font-size: 20px;
  }
  p {
    margin: 0;
    color: var(--text-muted);
    font-size: 13px;
    line-height: 1.5;
  }
</style>
