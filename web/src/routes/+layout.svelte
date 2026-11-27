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
  import { initTheme, theme, toggleTheme } from '#lib/theme';

  let { children } = $props();

  const protectedPrefixes = ['/dashboard', '/discover', '/matches', '/profile', '/onboarding', '/settings', '/account'];

  const tabRoutes = ['/dashboard', '/discover', '/matches', '/profile', '/settings'];

  let signedIn = $derived($session === 'signed-in');
  let pageTitle = $derived(titleFor(page.url.pathname));
  let sectionTitle = $derived(sectionFor(page.url.pathname));
  let activeTab = $derived(tabRoutes.findIndex((route) => isActive(route)));

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

  function sectionFor(path: string): string {
    if (path.startsWith('/dashboard')) return 'Dashboard';
    if (path.startsWith('/discover')) return 'Discover';
    if (path.startsWith('/matches')) return 'Matches';
    if (path.startsWith('/profile')) return 'Your card';
    if (path.startsWith('/settings') || path.startsWith('/account')) return 'Account';
    if (path.startsWith('/onboarding')) return 'Getting started';
    if (path.startsWith('/reviewer')) return 'Internal';
    if (path.startsWith('/appeal')) return 'Safety';
    return 'Tennis Connect';
  }

  function titleFor(path: string): string {
    if (path === '/') return 'Play more tennis';
    if (path === '/dashboard') return 'Overview';
    if (path === '/register') return 'Confirm your age';
    if (path === '/check-inbox') return 'Check your inbox';
    if (path === '/onboarding') return 'Set up your card';
    if (path === '/discover') return 'Discover';
    if (/^\/discover\/[^/]+$/.test(path)) return 'Player profile';
    if (path === '/matches') return 'Matches';
    if (/^\/matches\/[^/]+\/new$/.test(path)) return "It's a match";
    if (/^\/matches\/[^/]+$/.test(path)) return 'Match';
    if (path === '/profile') return 'You';
    if (path === '/profile/edit') return 'Edit profile';
    if (path === '/profile/photos') return 'Photos';
    if (path === '/profile/play-times') return 'Play times';
    if (path === '/settings') return 'Settings';
    if (path === '/account/recovery') return 'Recover your account';
    if (path === '/account/export') return 'Download your data';
    if (path === '/account/delete') return 'Delete account';
    if (path === '/appeal') return 'Appeal a suspension';
    if (path === '/reviewer') return 'Reviewer console';
    return 'Tennis Connect';
  }

  onMount(() => {
    const stopTheme = initTheme();
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
    return () => {
      stop();
      stopTheme();
    };
  });
</script>

<svelte:head>
  <title>Tennis Connect</title>
  <meta name="description" content="Meet adult tennis players near you, safely." />
</svelte:head>

<div
  class="app-shell"
  class:signed-in={signedIn}
  class:with-tabs={signedIn && !isMatchReveal(page.url.pathname)}
  class:match-reveal={isMatchReveal(page.url.pathname)}
>
  <a class="skip-link" href="#main">Skip to content</a>

  {#if signedIn}
    <aside class="app-sidebar" aria-label="App navigation">
      <div class="sidebar-inner">
        <a class="sidebar-brand" href="/">
          <span class="tennis-ball" aria-hidden="true"></span>
          Tennis Connect
        </a>

        <nav class="sidebar-nav" aria-label="Dashboard">
          <a
            class:active={isActive('/dashboard')}
            aria-current={isActive('/dashboard') ? 'page' : undefined}
            href="/dashboard"
          >
            <i class="bi bi-grid-1x2" aria-hidden="true"></i>
            <span>Dashboard</span>
          </a>
          <a
            class:active={isActive('/matches')}
            aria-current={isActive('/matches') ? 'page' : undefined}
            aria-label={$unrevealedMatches > 0 ? 'Matches, new match' : undefined}
            href="/matches"
          >
            <i class="bi bi-people" aria-hidden="true"></i>
            <span>Matches</span>
            {#if $unrevealedMatches > 0}
              <span class="nav-count" aria-hidden="true">{$unrevealedMatches}</span>
            {/if}
          </a>
          <a
            class:active={isActive('/discover')}
            aria-current={isActive('/discover') ? 'page' : undefined}
            href="/discover"
          >
            <i class="bi bi-compass" aria-hidden="true"></i>
            <span>Discover</span>
          </a>
          <a
            class:active={isActive('/profile')}
            aria-current={isActive('/profile') ? 'page' : undefined}
            href="/profile"
          >
            <i class="bi bi-person" aria-hidden="true"></i>
            <span>You</span>
          </a>
          <a
            class:active={isActive('/settings')}
            aria-current={isActive('/settings') ? 'page' : undefined}
            href="/settings"
          >
            <i class="bi bi-sliders" aria-hidden="true"></i>
            <span>Settings</span>
          </a>
        </nav>

        <div class="sidebar-foot">
          <p class="kicker">Private by design</p>
          <p>Exact areas stay on the server. Photos are checked before anyone sees them.</p>
        </div>
      </div>
    </aside>
  {/if}

  <div class="app-column">
    <header class="site-header">
      <div class="page header-inner">
        {#if signedIn}
          <div class="topbar-copy">
            <p class="topbar-kicker">{sectionTitle}</p>
            {#if page.url.pathname === '/dashboard'}
              <h1 class="topbar-title">Overview</h1>
            {:else}
              <p class="topbar-title">{pageTitle}</p>
            {/if}
          </div>
        {:else}
          <a class="brand" href="/">
            <span class="tennis-ball" aria-hidden="true"></span>
            Tennis Connect
          </a>
        {/if}

        <div class="header-actions">
          {#if $session === 'suspended'}
            <a class="btn btn-outline-secondary btn-sm" href="/appeal"><i class="bi bi-shield-exclamation" aria-hidden="true"></i> Appeal</a>
          {:else if !signedIn}
            <a class="btn btn-primary btn-sm" href="/#sign-in"><i class="bi bi-box-arrow-in-right" aria-hidden="true"></i> Sign in</a>
          {/if}
          <button
            class="theme-toggle"
            type="button"
            onclick={toggleTheme}
            aria-label={$theme === 'dark' ? 'Use light theme' : 'Use dark theme'}
            title={$theme === 'dark' ? 'Use light theme' : 'Use dark theme'}
          >
            <i class="bi" class:bi-sun={$theme === 'dark'} class:bi-moon-stars={$theme === 'light'} aria-hidden="true"></i>
          </button>
        </div>
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

    <footer class="site-footer">
      <div class="page d-flex flex-wrap justify-content-between align-items-center gap-2">
        <span class="muted"><span class="tennis-ball me-2" aria-hidden="true"></span>Adults only. UK tennis. Your area is never shown exactly.</span>
        <a href="/#how">Privacy and safety</a>
      </div>
    </footer>
  </div>

  {#if signedIn && !isMatchReveal(page.url.pathname)}
    <nav class="tab-bar" aria-label="Primary mobile" data-active={activeTab}>
      <span class="tab-indicator" aria-hidden="true">
        {#key page.url.pathname}
          <span class="tab-indicator-bar"></span>
        {/key}
      </span>
      <a href="/dashboard" class:active={isActive('/dashboard')} aria-current={isActive('/dashboard') ? 'page' : undefined}>
        <i class="bi bi-grid-1x2" aria-hidden="true"></i>
        <span>Dashboard</span>
      </a>
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
</div>

<style>
  .signed-in {
    display: grid;
    grid-template-columns: 15rem minmax(0, 1fr);
    align-items: stretch;
  }

  .app-sidebar {
    position: relative;
    min-width: 0;
    border-right: 1px solid var(--line);
    background: var(--sidebar-bg);
    backdrop-filter: blur(18px) saturate(1.2);
    -webkit-backdrop-filter: blur(18px) saturate(1.2);
    box-shadow: inset -1px 0 0 var(--highlight);
  }

  .sidebar-inner {
    position: sticky;
    top: 0;
    display: flex;
    flex-direction: column;
    gap: 1rem;
    height: 100dvh;
    padding: 1rem;
    overflow-y: auto;
  }

  .sidebar-brand {
    display: inline-flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.35rem 0.25rem;
    color: var(--ink);
    font-weight: 800;
    letter-spacing: -0.03em;
    text-decoration: none;
  }

  .sidebar-nav {
    display: grid;
    gap: 0.15rem;
    font-size: 0.9rem;
    font-weight: 700;
  }

  .sidebar-nav a {
    position: relative;
    display: flex;
    align-items: center;
    gap: 0.55rem;
    padding: 0.55rem 0.65rem;
    border-radius: 12px;
    color: var(--muted);
    text-decoration: none;
    transition: background 160ms ease, color 160ms ease, transform 160ms ease, box-shadow 160ms ease;
  }

  .sidebar-nav a:hover {
    background: var(--surface-hover);
    color: var(--court);
    transform: translateX(2px);
  }

  .sidebar-nav a.active {
    background: linear-gradient(135deg, var(--nav-active-from), var(--nav-active-to) 72%);
    color: white;
    transform: none;
    box-shadow: inset 0 1px 0 rgb(255 255 255 / 22%), 0 8px 20px var(--nav-glow);
  }

  .sidebar-nav a.active::before {
    content: '';
    position: absolute;
    left: -0.55rem;
    top: 50%;
    width: 3px;
    height: 1.15rem;
    border-radius: 999px;
    background: var(--ball);
    box-shadow: 0 0 10px var(--ball);
    transform: translateY(-50%);
  }

  .sidebar-nav a.active i {
    color: var(--ball);
  }

  .nav-count {
    display: inline-grid;
    place-items: center;
    min-width: 1.25rem;
    height: 1.25rem;
    margin-left: auto;
    padding-inline: 0.3rem;
    border-radius: 999px;
    background: linear-gradient(180deg, #efff9e, var(--ball));
    color: #0a1714;
    font-size: 0.68rem;
    font-weight: 800;
    box-shadow:
      inset 0 1px 0 rgb(255 255 255 / 70%),
      0 0 0 2px var(--surface),
      0 0 12px var(--ball-glow);
  }

  .sidebar-foot {
    margin-top: auto;
    padding: 0.75rem;
    border: 1px solid var(--line);
    border-radius: var(--radius-control);
    background: linear-gradient(180deg, var(--surface-top), var(--surface));
    box-shadow: var(--card-highlight), var(--tc-shadow-1);
  }

  .sidebar-foot p:last-child {
    margin: 0;
    color: var(--muted);
    font-size: 0.75rem;
    line-height: 1.35;
  }

  .app-column {
    display: flex;
    flex-direction: column;
    min-width: 0;
    min-height: 100dvh;
  }

  .site-header {
    position: sticky;
    top: 0;
    z-index: 20;
    padding-top: env(safe-area-inset-top);
    background: var(--header-bg);
    backdrop-filter: blur(12px) saturate(1.2);
    -webkit-backdrop-filter: blur(12px) saturate(1.2);
    box-shadow: var(--header-shadow);
  }

  .header-inner {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
    padding-block: 0.4rem;
  }

  .topbar-copy {
    min-width: 0;
  }

  .topbar-kicker {
    margin: 0 0 0.1rem;
    color: var(--muted);
    font-family: var(--font-mono);
    font-size: 0.64rem;
    font-weight: 700;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }

  .topbar-title {
    margin: 0;
    font-size: 1.08rem;
    font-weight: 800;
    letter-spacing: -0.035em;
    line-height: 1.05;
  }

  h1.topbar-title {
    font-size: 1.22rem;
  }

  .header-actions {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    min-width: 0;
  }

  .theme-toggle {
    flex: 0 0 auto;
    width: var(--tap);
    height: var(--tap);
    min-height: 0;
    padding: 0;
    border: 1px solid var(--line);
    border-radius: 999px;
    background: transparent;
    color: var(--court);
    box-shadow: none;
  }

  .theme-toggle:hover {
    background: var(--court);
    color: white;
    transform: none;
    box-shadow: none;
  }

  .brand {
    display: inline-flex;
    align-items: center;
    gap: 0.55rem;
    color: var(--ink);
    font-weight: 800;
    letter-spacing: -0.03em;
    text-decoration: none;
    white-space: nowrap;
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
    padding-block: 1rem;
    font-size: 0.82rem;
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
    background: var(--danger-bg);
    border-bottom: 1px solid var(--line);
  }

  .tab-bar {
    display: none;
  }

  .match-reveal .site-footer {
    display: none;
  }

  .match-reveal main {
    padding-bottom: 0;
  }

  @media (max-width: 991px) {
    .signed-in {
      grid-template-columns: minmax(0, 1fr);
    }

    .app-sidebar {
      display: none;
    }
  }

  @media (min-width: 768px) {
    .header-inner {
      padding-block: 0.7rem;
    }

    .topbar-title {
      font-size: clamp(1.25rem, 2.4vw, 1.55rem);
    }

    h1.topbar-title {
      font-size: clamp(1.6rem, 3vw, 2rem);
    }
  }

  @media (max-width: 767px) {
    .with-tabs {
      --sticky-offset: calc(4.4rem + env(safe-area-inset-bottom));
    }

    .with-tabs .site-footer {
      display: none;
    }

    .with-tabs main {
      padding-bottom: calc(4.4rem + env(safe-area-inset-bottom));
    }

    .tab-bar {
      position: fixed;
      left: 0;
      right: 0;
      bottom: 0;
      z-index: 30;
      display: grid;
      grid-template-columns: repeat(5, minmax(0, 1fr));
      padding-bottom: env(safe-area-inset-bottom);
      border-top: 1px solid var(--line);
      background: var(--tab-bg);
      backdrop-filter: blur(14px) saturate(1.2);
      -webkit-backdrop-filter: blur(14px) saturate(1.2);
      box-shadow: 0 -8px 24px rgb(6 40 31 / 6%);
    }

    .tab-bar[data-active='0'] {
      --tab-index: 0;
    }

    .tab-bar[data-active='1'] {
      --tab-index: 1;
    }

    .tab-bar[data-active='2'] {
      --tab-index: 2;
    }

    .tab-bar[data-active='3'] {
      --tab-index: 3;
    }

    .tab-bar[data-active='4'] {
      --tab-index: 4;
    }

    .tab-indicator {
      position: absolute;
      top: -1px;
      left: 0;
      width: 20%;
      height: 3px;
      transform: translateX(calc(var(--tab-index, 0) * 100%));
      transition: transform 280ms cubic-bezier(0.2, 0.8, 0.2, 1), opacity 160ms ease;
      pointer-events: none;
    }

    .tab-bar[data-active='-1'] .tab-indicator {
      opacity: 0;
    }

    .tab-indicator-bar {
      position: relative;
      display: block;
      height: 100%;
      margin-inline: 0.9rem;
      overflow: hidden;
      border-radius: 999px;
      background: linear-gradient(90deg, var(--court-light), var(--ball) 55%, var(--sky));
      box-shadow: 0 0 10px var(--ball-glow), 0 0 22px var(--focus-glow);
    }

    .tab-indicator-bar::after {
      content: '';
      position: absolute;
      inset: 0;
      background: linear-gradient(100deg, transparent 12%, rgb(255 255 255 / 88%) 50%, transparent 88%);
      transform: translateX(-130%);
      animation: tab-sweep 620ms ease-out both;
    }

    @keyframes tab-sweep {
      to {
        transform: translateX(130%);
      }
    }

    .tab-bar a {
      display: grid;
      justify-items: center;
      align-content: center;
      gap: 0.15rem;
      min-height: 4.4rem;
      padding: 0.3rem 0.2rem;
      color: var(--muted);
      font-size: 0.7rem;
      font-weight: 700;
      text-decoration: none;
      transition: background 160ms ease, color 160ms ease;
    }

    .tab-bar a i {
      font-size: 1.25rem;
    }

    .tab-bar a.active {
      margin: 0.25rem 0.12rem;
      border-radius: 14px;
      color: var(--court);
      background: linear-gradient(180deg, var(--surface-hover), var(--tab-active-bg));
      box-shadow: inset 0 2px 0 var(--court-light), var(--card-highlight);
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

  @supports not ((backdrop-filter: blur(1px)) or (-webkit-backdrop-filter: blur(1px))) {
    .app-sidebar,
    .tab-bar {
      background: var(--surface);
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .tab-indicator,
    .tab-indicator-bar::after {
      animation: none !important;
      transition: none !important;
    }
  }
</style>
