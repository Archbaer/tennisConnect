<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import 'bootstrap/dist/css/bootstrap.min.css';
  import 'bootstrap-icons/font/bootstrap-icons.css';
  import '../app.css';
  import { online, watchConnectivity } from '#lib/offline';
  import { unrevealedMatches } from '#lib/matchState';
  import { loadSession, session } from '#lib/session';

  let { children } = $props();

  const protectedPrefixes = ['/discover', '/matches', '/profile', '/onboarding', '/settings', '/account'];

  function needsAccount(path: string): boolean {
    if (path === '/account/recovery') return false;
    return protectedPrefixes.some((prefix) => path === prefix || path.startsWith(`${prefix}/`));
  }

  function isActive(path: string): boolean {
    return page.url.pathname === path || page.url.pathname.startsWith(`${path}/`);
  }

  function isMatchReveal(path: string): boolean {
    return /^\/matches\/[^/]+\/new$/.test(path);
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

<div class="app-shell" class:with-tabs={$session === 'signed-in' && !isMatchReveal(page.url.pathname)} class:match-reveal={isMatchReveal(page.url.pathname)}>
  <a class="skip-link" href="#main">Skip to content</a>

  <header class="site-header">
    <div class="page header-inner">
      <a class="brand" href="/">
        <span class="tennis-ball" aria-hidden="true"></span>
        Tennis Connect
      </a>
      <nav class="nav nav-pills desktop-nav" aria-label="Primary">
        {#if $session === 'signed-in'}
          <a class="nav-link" class:active={isActive('/discover')} aria-current={isActive('/discover') ? 'page' : undefined} href="/discover"><i class="bi bi-compass" aria-hidden="true"></i> Discover</a>
          <a class="nav-link" class:active={isActive('/matches')} aria-current={isActive('/matches') ? 'page' : undefined} aria-label={$unrevealedMatches > 0 ? 'Matches, new match' : undefined} href="/matches"><i class="bi bi-people" aria-hidden="true"></i> Matches</a>
          <a class="nav-link" class:active={isActive('/profile')} aria-current={isActive('/profile') ? 'page' : undefined} href="/profile"><i class="bi bi-person" aria-hidden="true"></i> You</a>
          <a class="nav-link" class:active={isActive('/settings')} aria-current={isActive('/settings') ? 'page' : undefined} href="/settings"><i class="bi bi-sliders" aria-hidden="true"></i> Settings</a>
        {:else if $session === 'suspended'}
          <a class="nav-link" href="/appeal"><i class="bi bi-shield-exclamation" aria-hidden="true"></i> Appeal</a>
        {:else}
          <a class="btn btn-primary btn-sm" href="/#sign-in"><i class="bi bi-box-arrow-in-right" aria-hidden="true"></i> Sign in</a>
        {/if}
      </nav>
    </div>
    <div class="net-line" aria-hidden="true"></div>
  </header>

  {#if !$online}
    <div class="offline-banner" role="status" aria-live="polite">
      You are offline. Saved screens still work; sending changes will wait until you reconnect.
    </div>
  {/if}

  <main id="main">
    {@render children()}
  </main>

  {#if $session === 'signed-in' && !isMatchReveal(page.url.pathname)}
    <nav class="tab-bar" aria-label="Primary mobile">
      <a href="/discover" class:active={isActive('/discover')} aria-current={isActive('/discover') ? 'page' : undefined}>
        <i class="bi bi-compass" aria-hidden="true"></i>
        <span>Discover</span>
      </a>
      <a href="/matches" class:active={isActive('/matches')} aria-current={isActive('/matches') ? 'page' : undefined} aria-label={$unrevealedMatches > 0 ? 'Matches, new match' : undefined}>
        <i class="bi bi-people" aria-hidden="true"></i>
        <span>Matches</span>
      </a>
      <a href="/profile" class:active={isActive('/profile')} aria-current={isActive('/profile') ? 'page' : undefined}>
        <i class="bi bi-person" aria-hidden="true"></i>
        <span>You</span>
      </a>
      <a href="/settings" class:active={isActive('/settings')} aria-current={isActive('/settings') ? 'page' : undefined}>
        <i class="bi bi-sliders" aria-hidden="true"></i>
        <span>Settings</span>
      </a>
    </nav>
  {/if}

  <footer class="site-footer">
    <div class="page d-flex flex-wrap justify-content-between align-items-center gap-2">
      <span class="muted"><span class="tennis-ball me-2" aria-hidden="true"></span>Adults only. UK tennis. Your area is never shown exactly.</span>
      <a href="/#how">Privacy and safety</a>
    </div>
  </footer>
</div>

<style>
  .site-header {
    background: rgb(247 249 244 / 94%);
    position: sticky;
    top: 0;
    z-index: 20;
    backdrop-filter: blur(10px);
  }

  .header-inner {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    padding-block: 0.7rem;
  }

  .brand {
    display: inline-flex;
    align-items: center;
    gap: 0.55rem;
    font-weight: 800;
    letter-spacing: -0.03em;
    color: var(--ink);
    text-decoration: none;
    white-space: nowrap;
  }

  .nav-pills {
    gap: 0.15rem;
    font-size: 0.9rem;
    font-weight: 700;
    flex-wrap: nowrap;
  }

  .nav-pills .nav-link {
    color: var(--court);
    border-radius: 999px;
    padding: 0.4rem 0.7rem;
  }

  .nav-pills .nav-link:hover,
  .nav-pills .nav-link:focus-visible {
    background: #e5efeb;
    color: var(--court-deep);
  }

  .nav-pills .nav-link.active {
    background: var(--court);
    color: white;
  }

  .net-line {
    height: 4px;
    background-image: repeating-linear-gradient(
      to right,
      var(--court) 0 13px,
      transparent 13px 20px
    );
    opacity: 0.55;
  }

  .site-footer {
    margin-top: auto;
    border-top: 1px solid var(--line);
    background: var(--court-deep);
    color: #dce8e2;
    padding-block: 1.6rem;
    font-size: 0.85rem;
  }

  .site-footer .page {
    padding-block: 0;
  }

  .site-footer a,
  .site-footer .muted {
    color: #dce8e2;
  }

  .offline-banner {
    padding: 0.65rem 1.25rem;
    text-align: center;
    font-weight: 700;
    color: var(--ink);
    background: #f7e4e1;
    border-bottom: 1px solid #e6c4bf;
  }

  .tab-bar {
    display: none;
  }

  .match-reveal .desktop-nav,
  .match-reveal .site-footer {
    display: none;
  }

  .match-reveal main {
    padding-bottom: 0;
  }

  @media (max-width: 767px) {
    .with-tabs .desktop-nav,
    .with-tabs .site-footer {
      display: none;
    }

    .with-tabs main {
      padding-bottom: calc(4.75rem + env(safe-area-inset-bottom));
    }

    .tab-bar {
      position: fixed;
      left: 0;
      right: 0;
      bottom: 0;
      z-index: 30;
      display: grid;
      grid-template-columns: repeat(4, minmax(0, 1fr));
      padding-bottom: env(safe-area-inset-bottom);
      border-top: 1px solid var(--line);
      background: rgb(255 255 255 / 96%);
      backdrop-filter: blur(12px);
    }

    .tab-bar a {
      display: grid;
      justify-items: center;
      align-content: center;
      gap: 0.15rem;
      min-height: 4rem;
      padding: 0.35rem 0.25rem;
      color: var(--muted);
      font-size: 0.72rem;
      font-weight: 700;
      text-decoration: none;
    }

    .tab-bar a i {
      font-size: 1.2rem;
    }

    .tab-bar a.active {
      color: var(--court);
      background: #eef4ef;
    }

    .tab-bar a.active i {
      color: var(--court);
    }
  }

  @media (min-width: 768px) {
    .tab-bar {
      display: none;
    }
  }
</style>
