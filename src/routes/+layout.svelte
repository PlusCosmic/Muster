<script lang="ts">
  import type { Snippet } from 'svelte';
  import '../app.css';
  import DialogHost from '$lib/components/DialogHost.svelte';
  import ToastHost from '$lib/components/ToastHost.svelte';

  let { children }: { children: Snippet } = $props();
</script>

{@render children()}

<DialogHost />
<ToastHost />
