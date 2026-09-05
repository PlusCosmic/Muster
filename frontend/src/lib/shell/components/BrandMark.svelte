<script lang="ts">
  // The app mark, recoloured per theme via the --mark-* tokens so it follows
  // live theme previews too. (Inline copy of lib/assets/rimforge.svg.)
  let { size = 28 }: { size?: number } = $props();
</script>

<svg class="mark" style:width="{size}px" style:height="{size}px" viewBox="0 0 1024 1024" aria-hidden="true">
  <defs>
    <linearGradient id="mark-tile" x1="0" y1="0" x2="1" y2="1">
      <stop offset="0" class="g1" />
      <stop offset="1" class="g2" />
    </linearGradient>
  </defs>
  <rect width="1024" height="1024" rx="212" fill="url(#mark-tile)" />
  <g class="fg">
    <path d="M 400 420 L 624 420 Q 646 420 656 440 L 700 540 Q 708 560 684 560 L 340 560 Q 316 560 324 540 L 368 440 Q 378 420 400 420 Z" />
    <path d="M 208 600 L 432 600 Q 454 600 464 620 L 508 720 Q 516 740 492 740 L 148 740 Q 124 740 132 720 L 176 620 Q 186 600 208 600 Z" />
    <path d="M 592 600 L 816 600 Q 838 600 848 620 L 892 720 Q 900 740 876 740 L 532 740 Q 508 740 516 720 L 560 620 Q 570 600 592 600 Z" />
    <path d="M 512 180 l 22 56 56 22 -56 22 -22 56 -22 -56 -56 -22 56 -22 z" />
  </g>
</svg>

<style>
  .mark {
    display: block;
    flex: none;
  }
  .mark .g1 {
    stop-color: var(--mark-grad-1);
  }
  .mark .g2 {
    stop-color: var(--mark-grad-2);
  }
  .mark .fg {
    fill: var(--mark-fg);
  }
</style>
