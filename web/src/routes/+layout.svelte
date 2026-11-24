<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import '../app.css';
  import { online, watchConnectivity } from '#lib/offline';
  import { loadSession, session } from '#lib/session';

  let { children } = $props();

  const protectedPrefixes = ['/discover', '/matches', '/profile', '/onboarding', '/settings', '/account', '/report'];

  function needsAccount(path: string): boolean {
    return protectedPrefixes.some((prefix) => path === prefix || path.startsWith(`${prefix}/`));
  }

  onMount(() => {
    const stop = watchConnectivity();
    loadSession().then((state) => {
      const path = window.location.pathname;
      if (state === 'signed-out' && needsAccount(path)) {
        goto(`/?next=${encodeURIComponent(path)}`, { replace: true });
      }
      if (state === 'suspended' && needsAccount(path)) {
        goto('/appeal', { replace: true });
      }
    });
    return stop;
  });
</script>

<svelte:head>
  <title>Tennis Connect</title>
  <meta name="description" content="Meet adult tennis players near you, safely." />
</svelte:head>

<a class="skip-link" href="#main">Skip to content</a>

<header class="site-header">
  <div class="page header-inner">
    <a class="brand" href="/">Tennis Connect</a>
    <nav aria-label="Primary">
      {#if $session === 'signed-in'}
        <a href="/discover">Discover</a>
        <a href="/matches">Matches</a>
        <a href="/profile">You</a>
        <a href="/settings">Settings</a>
      {:else if $session === 'suspended'}
        <a href="/appeal">Appeal</a>
      {:else}
        <a href="/#sign-in">Sign in</a>
      {/if}
    </nav>
  </div>
</header>

{#if !$online}
  <div class="offline-banner" role="status" aria-live="polite">
    You are offline. Saved screens still work; sending changes will wait until you reconnect.
  </div>
{/if}

<main id="main">
  {@render children()}
</main>

<footer class="site-footer">
  <div class="page row spread">
    <span class="muted">Adults only. UK tennis. Your area is never shown exactly.</span>
    <a href="/#how">Privacy and safety</a>
  </div>
</footer>

<style>
  .site-header {
    border-bottom: 1px solid var(--rule);
    background: rgb(246 247 245 / 92%);
    position: sticky;
    top: 0;
    z-index: 5;
    backdrop-filter: blur(8px);
  }

  .header-inner {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    padding-block: 0.75rem;
  }

  .brand {
    font-weight: 800;
    letter-spacing: -0.02em;
    color: var(--ink);
    text-decoration: none;
  }

  nav {
    display: flex;
    gap: 0.9rem;
    font-size: 0.92rem;
    font-weight: 600;
  }

  nav a {
    text-decoration: none;
  }

  .site-footer {
    border-top: 1px solid var(--rule);
    padding-block: 1.5rem;
    font-size: 0.85rem;
  }

  .site-footer .page {
    padding-block: 0;
  }

  .offline-banner {
    padding: 0.65rem 1.25rem;
    text-align: center;
    font-weight: 600;
    color: var(--ink);
  }

  .offline-banner {
    background: #f7e4e1;
    border-bottom: 1px solid #e6c4bf;
  }

</style>
