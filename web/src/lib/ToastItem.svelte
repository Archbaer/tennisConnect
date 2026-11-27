<script lang="ts">
  import { onMount } from 'svelte';
  import { dismissToast, TOAST_DURATION_MS, type Toast } from '#lib/toasts';

  export let toast: Toast;

  let timer: number | undefined;
  let focused = false;

  function pause() {
    if (timer === undefined) return;
    window.clearTimeout(timer);
    timer = undefined;
  }

  function schedule() {
    pause();
    if (focused) return;
    timer = window.setTimeout(() => dismissToast(toast.id), TOAST_DURATION_MS);
  }

  function onFocusIn() {
    focused = true;
    pause();
  }

  function onFocusOut(event: FocusEvent) {
    const next = event.relatedTarget as Node | null;
    if (next && (event.currentTarget as Node).contains(next)) return;
    focused = false;
    schedule();
  }

  onMount(() => {
    schedule();
    return pause;
  });
</script>

<div class="toast-item" role="group" onmouseenter={pause} onmouseleave={schedule} onfocusin={onFocusIn} onfocusout={onFocusOut}>
  <p class="toast-message" aria-hidden={toast.announce ? undefined : 'true'}>
    <i class="bi bi-check-circle-fill" aria-hidden="true"></i>
    <span>{toast.message}</span>
  </p>
  <button class="toast-close" type="button" aria-label="Dismiss notification" onclick={() => dismissToast(toast.id)}>
    <i class="bi bi-x-lg" aria-hidden="true"></i>
  </button>
</div>

<style>
  .toast-item {
    position: relative;
    display: flex;
    align-items: center;
    gap: 0.5rem;
    min-height: 3.25rem;
    padding: 0.35rem 0.35rem 0.35rem 0.85rem;
    overflow: hidden;
    border: 1px solid var(--line);
    border-radius: var(--radius-control);
    background: linear-gradient(180deg, var(--surface-top), var(--surface) 72%);
    box-shadow: var(--card-highlight), var(--tc-shadow-3);
    pointer-events: auto;
    animation: toast-in 260ms cubic-bezier(0.2, 0.8, 0.2, 1) both;
  }

  .toast-item::after {
    content: '';
    position: absolute;
    inset: 0;
    pointer-events: none;
    background: linear-gradient(100deg, transparent 16%, var(--skeleton-sheen) 50%, transparent 84%);
    transform: translateX(-130%);
    animation: toast-sweep 700ms ease-out both;
  }

  .toast-item > * {
    position: relative;
    z-index: 1;
  }

  .toast-message {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    flex: 1;
    min-width: 0;
    margin: 0;
    font-size: 0.88rem;
    font-weight: 600;
    line-height: 1.3;
  }

  .toast-message i {
    flex: 0 0 auto;
    color: var(--success);
    font-size: 1rem;
  }

  .toast-close {
    flex: 0 0 auto;
    width: 2.75rem;
    height: 2.75rem;
    min-height: 0;
    padding: 0;
    border: 0;
    border-radius: 999px;
    background: transparent;
    color: var(--muted);
    box-shadow: none;
  }

  .toast-close:hover {
    background: var(--surface-hover);
    color: var(--ink);
    transform: none;
    box-shadow: none;
  }

  @keyframes toast-in {
    from {
      opacity: 0;
      transform: translateY(10px) scale(0.98);
    }
    to {
      opacity: 1;
      transform: none;
    }
  }

  @keyframes toast-sweep {
    to {
      transform: translateX(130%);
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .toast-item,
    .toast-item::after {
      animation: none !important;
      transition: none !important;
    }
  }
</style>
